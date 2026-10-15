package service

import (
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"gorm.io/gorm"

	"lxdapi/internal/db"
	"lxdapi/internal/lxc"
	"lxdapi/models"
	"lxdapi/pkg/logger"
)

// ============================================================
// 多节点 / 机房（v1.1.17）
//
// 设计约定：
//   - 主控库保存「商业数据」：机房 regions、节点 nodes、商品 products、订单 orders、实例 user_products。
//   - 镜像、存储池、IP 池、端口映射、NAT、反代、防火墙、流量等「节点本地资源」的唯一真实来源是
//     各节点自己的 lxdapi 数据库；主控通过 node_id 把请求路由到节点执行（AdminNodeProxy）。
//     主控库里的同名表只代表主控「本机」(node 0)。
// ============================================================

const DefaultRegionName = "默认机房"

// LocalLXDAvailable 本机是否安装了 LXD（lxc 命令可用）。主控（如 webhost）通常没有。
func LocalLXDAvailable() bool {
	return lxc.BinaryAvailable()
}

// ---------------- 机房 ----------------

// RegionInput 机房入参
type RegionInput struct {
	Name        string `json:"name"`
	Code        string `json:"code"`
	Description string `json:"description"`
	SortOrder   int    `json:"sort_order"`
	Status      string `json:"status"`
}

// RegionView 机房 + 统计
type RegionView struct {
	models.Region
	NodeCount    int64 `json:"node_count"`
	ProductCount int64 `json:"product_count"`
}

// ListRegions 机房列表（含节点/商品数量）
func ListRegions(onlyActive bool) ([]RegionView, error) {
	var regions []models.Region
	q := db.DB.Order("sort_order ASC, id ASC")
	if onlyActive {
		q = q.Where("status = ? OR status = '' OR status IS NULL", "active")
	}
	if err := q.Find(&regions).Error; err != nil {
		return nil, err
	}
	out := make([]RegionView, 0, len(regions))
	for _, r := range regions {
		v := RegionView{Region: r}
		db.DB.Model(&models.Node{}).Where("region_id = ?", r.ID).Count(&v.NodeCount)
		db.DB.Model(&models.Product{}).Where("region_id = ?", r.ID).Count(&v.ProductCount)
		out = append(out, v)
	}
	return out, nil
}

// GetRegion 获取机房
func GetRegion(id uint) (*models.Region, error) {
	var r models.Region
	if err := db.DB.First(&r, id).Error; err != nil {
		return nil, fmt.Errorf("机房不存在")
	}
	return &r, nil
}

// CreateRegion 新增机房
func CreateRegion(in RegionInput) (*models.Region, error) {
	in.Name = strings.TrimSpace(in.Name)
	if in.Name == "" {
		return nil, fmt.Errorf("机房名称不能为空")
	}
	var cnt int64
	db.DB.Model(&models.Region{}).Where("name = ?", in.Name).Count(&cnt)
	if cnt > 0 {
		return nil, fmt.Errorf("机房名称已存在")
	}
	r := models.Region{Name: in.Name, Code: in.Code, Description: in.Description, SortOrder: in.SortOrder, Status: in.Status}
	if r.Status == "" {
		r.Status = "active"
	}
	if err := db.DB.Create(&r).Error; err != nil {
		return nil, err
	}
	return &r, nil
}

// UpdateRegion 更新机房（改名时同步节点的 region 文本）
func UpdateRegion(id uint, in RegionInput) error {
	r, err := GetRegion(id)
	if err != nil {
		return err
	}
	in.Name = strings.TrimSpace(in.Name)
	if in.Name == "" {
		return fmt.Errorf("机房名称不能为空")
	}
	var cnt int64
	db.DB.Model(&models.Region{}).Where("name = ? AND id <> ?", in.Name, id).Count(&cnt)
	if cnt > 0 {
		return fmt.Errorf("机房名称已存在")
	}
	if in.Status == "" {
		in.Status = "active"
	}
	return db.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(r).Updates(map[string]interface{}{
			"name": in.Name, "code": in.Code, "description": in.Description,
			"sort_order": in.SortOrder, "status": in.Status,
		}).Error; err != nil {
			return err
		}
		return tx.Model(&models.Node{}).Where("region_id = ?", id).Update("region", in.Name).Error
	})
}

// DeleteRegion 删除机房（仍有节点时拒绝）
func DeleteRegion(id uint) error {
	var cnt int64
	db.DB.Model(&models.Node{}).Where("region_id = ?", id).Count(&cnt)
	if cnt > 0 {
		return fmt.Errorf("该机房下仍有 %d 个节点，请先移走节点", cnt)
	}
	return db.DB.Delete(&models.Region{}, id).Error
}

// EnsureRegionByName 按名称取机房，不存在则创建；空名称归入「默认机房」
func EnsureRegionByName(tx *gorm.DB, name string) (*models.Region, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		name = DefaultRegionName
	}
	var r models.Region
	err := tx.Where("name = ?", name).First(&r).Error
	if err == nil {
		return &r, nil
	}
	r = models.Region{Name: name, Status: "active"}
	if err := tx.Create(&r).Error; err != nil {
		return nil, err
	}
	return &r, nil
}

// resolveNodeRegion 节点保存时决定其机房：优先 region_id，否则按 region 名称自动建/取
func resolveNodeRegion(tx *gorm.DB, regionID uint, regionName string) (*models.Region, error) {
	if regionID > 0 {
		var r models.Region
		if err := tx.First(&r, regionID).Error; err != nil {
			return nil, fmt.Errorf("机房不存在")
		}
		return &r, nil
	}
	return EnsureRegionByName(tx, regionName)
}

// SyncProductsRegionOfNode 节点机房变化后，同步该节点下商品的 region_id
func SyncProductsRegionOfNode(tx *gorm.DB, nodeID, regionID uint) error {
	return tx.Model(&models.Product{}).Where("node_id = ?", nodeID).Update("region_id", regionID).Error
}

// ProductRegionID 计算商品应属的机房（节点的机房；本机商品为 0）
func ProductRegionID(nodeID uint) uint {
	if nodeID == 0 {
		return 0
	}
	var n models.Node
	if err := db.DB.Select("id, region_id").First(&n, nodeID).Error; err != nil {
		return 0
	}
	return n.RegionID
}

// ---------------- 名称映射（列表展示用） ----------------

// NodeNames 节点 ID → 名称、机房 ID
type NodeBrief struct {
	ID         uint   `json:"id"`
	Name       string `json:"name"`
	RegionID   uint   `json:"region_id"`
	RegionName string `json:"region_name"`
	Status     string `json:"status"`
}

// NodeBriefMap 所有节点的简要信息（含机房名）
func NodeBriefMap() map[uint]NodeBrief {
	out := map[uint]NodeBrief{}
	var nodes []models.Node
	db.DB.Find(&nodes)
	regionNames := RegionNameMap()
	for _, n := range nodes {
		rn := regionNames[n.RegionID]
		if rn == "" {
			rn = n.Region
		}
		out[n.ID] = NodeBrief{ID: n.ID, Name: n.Name, RegionID: n.RegionID, RegionName: rn, Status: n.Status}
	}
	return out
}

// RegionNameMap 机房 ID → 名称
func RegionNameMap() map[uint]string {
	out := map[uint]string{}
	var regions []models.Region
	db.DB.Find(&regions)
	for _, r := range regions {
		out[r.ID] = r.Name
	}
	return out
}

// NodeLabel 展示用节点名（0=本机）
func NodeLabel(m map[uint]NodeBrief, id uint) (string, string) {
	if id == 0 {
		return "本机", ""
	}
	if b, ok := m[id]; ok {
		return b.Name, b.RegionName
	}
	return fmt.Sprintf("节点#%d(已删除)", id), ""
}

// ---------------- 一次性迁移 ----------------

const migrationMultiNodeV1117 = "v1117_multinode"

// RunMultiNodeMigration v1.1.17 多节点数据迁移（幂等，可重复执行）：
//  1. 按节点已有的 region 文本建立机房，回填 nodes.region_id（空的归入「默认机房」）；
//  2. 主控本机无 LXD 且恰好只有一个节点时，把 node_id=0 的商品与实例归属到该节点
//     （本机不可能运行这些实例；对应现网：唯一节点「七牛云-LXD」）；
//  3. 回填 products.region_id、orders.node_id。
//
// 每一步都只处理尚未回填的行，重复执行不会改动已迁移数据。
func RunMultiNodeMigration() {
	defer func() {
		if r := recover(); r != nil {
			logger.Error("多节点数据迁移异常: %v", r)
		}
	}()
	notes := []string{}
	err := db.DB.Transaction(func(tx *gorm.DB) error {
		// 1. 机房
		var nodes []models.Node
		if err := tx.Find(&nodes).Error; err != nil {
			return err
		}
		for _, n := range nodes {
			if n.RegionID > 0 {
				var cnt int64
				tx.Model(&models.Region{}).Where("id = ?", n.RegionID).Count(&cnt)
				if cnt > 0 {
					continue
				}
			}
			r, err := EnsureRegionByName(tx, n.Region)
			if err != nil {
				return err
			}
			if err := tx.Model(&models.Node{}).Where("id = ?", n.ID).Updates(map[string]interface{}{"region_id": r.ID, "region": r.Name}).Error; err != nil {
				return err
			}
			notes = append(notes, fmt.Sprintf("节点#%d→机房「%s」", n.ID, r.Name))
		}

		// 2. 本机无 LXD 且唯一节点：node_id=0 的商品/实例归属该节点
		if len(nodes) == 1 && !LocalLXDAvailable() {
			only := nodes[0].ID
			res := tx.Model(&models.Product{}).Where("node_id = 0 OR node_id IS NULL").Update("node_id", only)
			if res.Error != nil {
				return res.Error
			}
			if res.RowsAffected > 0 {
				notes = append(notes, fmt.Sprintf("%d 个本机商品→节点#%d", res.RowsAffected, only))
			}
			res = tx.Model(&models.UserProduct{}).Where("node_id = 0 OR node_id IS NULL").Update("node_id", only)
			if res.Error != nil {
				return res.Error
			}
			if res.RowsAffected > 0 {
				notes = append(notes, fmt.Sprintf("%d 个本机实例→节点#%d", res.RowsAffected, only))
			}
		}

		// 3. 商品机房、订单节点快照
		var reNodes []models.Node
		tx.Find(&reNodes)
		for _, n := range reNodes {
			if err := tx.Model(&models.Product{}).Where("node_id = ? AND (region_id = 0 OR region_id IS NULL OR region_id <> ?)", n.ID, n.RegionID).
				Update("region_id", n.RegionID).Error; err != nil {
				return err
			}
		}
		if err := tx.Exec(`UPDATE orders SET node_id = (SELECT up.node_id FROM user_products up WHERE up.id = orders.user_product_id)
			WHERE (node_id = 0 OR node_id IS NULL) AND user_product_id > 0
			AND EXISTS (SELECT 1 FROM user_products up WHERE up.id = orders.user_product_id AND up.node_id > 0)`).Error; err != nil {
			return err
		}
		if err := tx.Exec(`UPDATE orders SET node_id = (SELECT p.node_id FROM products p WHERE p.id = orders.product_id)
			WHERE (node_id = 0 OR node_id IS NULL) AND product_id > 0
			AND EXISTS (SELECT 1 FROM products p WHERE p.id = orders.product_id AND p.node_id > 0)`).Error; err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		logger.Error("多节点数据迁移失败（已回滚，不影响启动）: %v", err)
		return
	}
	note := strings.Join(notes, "；")
	var m models.AppMigration
	if db.DB.Where("migration_key = ?", migrationMultiNodeV1117).First(&m).Error != nil {
		db.DB.Create(&models.AppMigration{MigrationKey: migrationMultiNodeV1117, AppliedAt: time.Now().Unix(), Note: note})
	} else if note != "" {
		db.DB.Model(&m).Updates(map[string]interface{}{"applied_at": time.Now().Unix(), "note": note})
	}
	if note != "" {
		logger.OK("多节点数据迁移: %s", note)
	}
	warnLocalLeftovers()
}

// warnLocalLeftovers 主控无本机 LXD 但本机库里存在节点本地资源配置时提示（这些配置不会作用到任何节点）
func warnLocalLeftovers() {
	if LocalLXDAvailable() {
		return
	}
	var nodeCnt int64
	db.DB.Model(&models.Node{}).Count(&nodeCnt)
	if nodeCnt == 0 {
		return
	}
	type item struct {
		label string
		model interface{}
	}
	items := []item{
		{"IPv4 地址池", &models.IPv4Pool{}}, {"IPv6 地址池", &models.IPv6Pool{}},
		{"IPv4 端口映射", &models.PortMappingV4{}}, {"IPv6 端口映射", &models.PortMappingV6{}},
		{"反向代理", &models.ReverseProxy{}},
	}
	parts := []string{}
	for _, it := range items {
		var c int64
		db.DB.Model(it.model).Count(&c)
		if c > 0 {
			parts = append(parts, fmt.Sprintf("%s %d 条", it.label, c))
		}
	}
	if len(parts) > 0 {
		logger.Warn("本机未安装 LXD，但本机库中存在 %s：这些配置只属于主控本机、不会作用到任何节点。请在后台顶栏切换到对应节点后重新配置。", strings.Join(parts, "、"))
	}
}

// ---------------- 节点健康检查 ----------------

var nodeHealthOnce sync.Once

// StartNodeHealthChecker 每 5 分钟检测一次各节点连通性，更新 status / last_ping_at
func StartNodeHealthChecker() {
	nodeHealthOnce.Do(func() {
		go func() {
			defer func() {
				if r := recover(); r != nil {
					logger.Error("节点健康检查异常: %v", r)
				}
			}()
			time.Sleep(20 * time.Second)
			for {
				CheckAllNodes()
				time.Sleep(5 * time.Minute)
			}
		}()
	})
}

// CheckAllNodes 并发检测所有节点
func CheckAllNodes() {
	nodes, err := ListNodes()
	if err != nil || len(nodes) == 0 {
		return
	}
	var wg sync.WaitGroup
	for i := range nodes {
		wg.Add(1)
		go func(n models.Node) {
			defer wg.Done()
			status := "online"
			if err := NewNodeClientTimeout(&n, 15*time.Second).Ping(); err != nil {
				status = "offline"
			}
			now := time.Now()
			db.DB.Model(&models.Node{}).Where("id = ?", n.ID).Updates(map[string]interface{}{"status": status, "last_ping_at": &now})
		}(nodes[i])
	}
	wg.Wait()
}

// AnnotateNodes 列表元素转为 map 并附加 node_name、region_name（展示用）
func AnnotateNodes[T any](items []T, nodeIDOf func(*T) uint) []map[string]interface{} {
	briefs := NodeBriefMap()
	out := make([]map[string]interface{}, 0, len(items))
	for i := range items {
		b, err := json.Marshal(items[i])
		if err != nil {
			continue
		}
		m := map[string]interface{}{}
		if json.Unmarshal(b, &m) != nil {
			continue
		}
		nn, rn := NodeLabel(briefs, nodeIDOf(&items[i]))
		m["node_name"] = nn
		m["region_name"] = rn
		out = append(out, m)
	}
	return out
}

// ParseOptionalUint 解析可选的数字查询参数
func ParseOptionalUint(s string) *uint {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}
	var v uint
	if _, err := fmt.Sscanf(s, "%d", &v); err != nil {
		return nil
	}
	return &v
}

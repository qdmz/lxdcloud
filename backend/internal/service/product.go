package service

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"lxdapi/internal/db"
	"lxdapi/models"
	"lxdapi/pkg/logger"
)

// ProductInput 商品入参
type ProductInput struct {
	Name        string  `json:"name"`
	Description string  `json:"description"`
	NodeID      uint    `json:"node_id"`
	Type        string  `json:"type"`
	Image       string  `json:"image"`
	StoragePool string  `json:"storage_pool"`
	NetworkMode string  `json:"network_mode"`
	CPU         int     `json:"cpu"`
	Memory      int     `json:"memory"`
	Disk        int     `json:"disk"`
	Ingress     int     `json:"ingress"`
	Egress      int     `json:"egress"`
	TrafficLimit int64  `json:"traffic_limit"`
	IPv4Count   int     `json:"ipv4_count"`
	IPv6Count   int     `json:"ipv6_count"`
	PortMappings int    `json:"port_mappings"`
	AllowNesting bool   `json:"allow_nesting"`
	MemorySwap  bool    `json:"memory_swap"`
	Privileged  bool    `json:"privileged"`
	Backups     int     `json:"backups"`
	PriceMonthly   float64 `json:"price_monthly"`
	PriceQuarterly float64 `json:"price_quarterly"`
	PriceHalfYear  float64 `json:"price_half_year"`
	PriceYearly    float64 `json:"price_yearly"`
	Stock       *int    `json:"stock"`          // nil：新增时默认 -1（不限），编辑时保持不变
	PerUserLimit *int   `json:"per_user_limit"` // nil：新增时默认 0（不限），编辑时保持不变
	Status      string  `json:"status"`
	SortOrder   int     `json:"sort_order"`
}

// ListProducts 商品列表（后台全部）
func ListProducts() ([]models.Product, error) {
	var products []models.Product
	if err := db.DB.Order("sort_order ASC, id ASC").Find(&products).Error; err != nil {
		return nil, err
	}
	return products, nil
}

// GetActiveProducts 上架商品
func GetActiveProducts() ([]models.Product, error) {
	var products []models.Product
	if err := db.DB.Where("status = ?", "active").Order("sort_order ASC, id ASC").Find(&products).Error; err != nil {
		return nil, err
	}
	return products, nil
}

// GetProduct 获取商品
func GetProduct(id uint) (*models.Product, error) {
	var p models.Product
	if err := db.DB.First(&p, id).Error; err != nil {
		return nil, fmt.Errorf("商品不存在")
	}
	return &p, nil
}

// ProductFilter 商品筛选
type ProductFilter struct {
	NodeID   *uint
	RegionID *uint
}

// ListProductsFiltered 按节点/机房筛选商品（后台）
func ListProductsFiltered(f ProductFilter) ([]models.Product, error) {
	var products []models.Product
	q := db.DB.Order("sort_order ASC, id ASC")
	if f.NodeID != nil {
		q = q.Where("node_id = ?", *f.NodeID)
	}
	if f.RegionID != nil {
		q = q.Where("region_id = ?", *f.RegionID)
	}
	if err := q.Find(&products).Error; err != nil {
		return nil, err
	}
	return products, nil
}

// GetActiveProductsInRegion 上架商品（regionID=0 表示全部机房）
func GetActiveProductsInRegion(regionID uint) ([]models.Product, error) {
	var products []models.Product
	q := db.DB.Where("status = ?", "active")
	if regionID > 0 {
		q = q.Where("region_id = ?", regionID)
	}
	if err := q.Order("sort_order ASC, id ASC").Find(&products).Error; err != nil {
		return nil, err
	}
	return products, nil
}

// validateProductNode 商品所属节点必须存在；主控无本机 LXD 时不允许选「本机」
func validateProductNode(nodeID uint) error {
	if nodeID == 0 {
		if !LocalLXDAvailable() {
			var cnt int64
			db.DB.Model(&models.Node{}).Count(&cnt)
			if cnt > 0 {
				return fmt.Errorf("主控本机未安装 LXD，请为商品选择一个节点")
			}
		}
		return nil
	}
	if _, err := GetNode(nodeID); err != nil {
		return fmt.Errorf("所属节点不存在")
	}
	return nil
}

// ErrNodeVMUnsupported 节点开通接口（/api/system/containers）只创建 LXC 容器，不支持虚拟机
const ErrNodeVMUnsupported = "所选节点不支持虚拟机（VM）类型：节点开通接口只能创建 LXC 容器，请将类型改为「容器」或改选主控本机"

// localVMSupported 主控本机能否创建 LXD 虚拟机（需要本机 LXD 与 /dev/kvm）
func localVMSupported() bool {
	if !LocalLXDAvailable() {
		return false
	}
	_, err := os.Stat("/dev/kvm")
	return err == nil
}

// validateProductType 商品类型校验：仅 container / vm；VM 只能由具备 KVM 的主控本机开通
func validateProductType(nodeID uint, typ string) error {
	switch typ {
	case "", "container":
		return nil
	case "vm":
		if nodeID != 0 {
			return errors.New(ErrNodeVMUnsupported)
		}
		if !localVMSupported() {
			return fmt.Errorf("主控本机不支持虚拟机（未安装 LXD 或缺少 /dev/kvm），请将类型改为「容器」")
		}
		return nil
	default:
		return fmt.Errorf("无效的商品类型：%s", typ)
	}
}

// CreateProduct 新增商品
func CreateProduct(in *ProductInput) error {
	if in.Name == "" {
		return fmt.Errorf("商品名称不能为空")
	}
	if err := validateProductNode(in.NodeID); err != nil {
		return err
	}
	if err := validateProductType(in.NodeID, in.Type); err != nil {
		return err
	}
	if in.Memory <= 0 || in.Disk <= 0 {
		return fmt.Errorf("内存与磁盘必须大于 0")
	}
	p := models.Product{
		Name:         in.Name,
		Description:  in.Description,
		NodeID:       in.NodeID,
		RegionID:     ProductRegionID(in.NodeID),
		Type:         in.Type,
		Image:        in.Image,
		StoragePool:  in.StoragePool,
		NetworkMode:  in.NetworkMode,
		CPU:          in.CPU,
		Memory:       in.Memory,
		Disk:         in.Disk,
		Ingress:      in.Ingress,
		Egress:       in.Egress,
		TrafficLimit: in.TrafficLimit,
		IPv4Count:    in.IPv4Count,
		IPv6Count:    in.IPv6Count,
		PortMappings: in.PortMappings,
		AllowNesting: in.AllowNesting,
		MemorySwap:   in.MemorySwap,
		Privileged:   in.Privileged,
		Backups:      in.Backups,
		PriceMonthly: in.PriceMonthly,
		PriceQuarterly: in.PriceQuarterly,
		PriceHalfYear: in.PriceHalfYear,
		PriceYearly:  in.PriceYearly,
		Stock:        -1,
		Status:       in.Status,
		SortOrder:    in.SortOrder,
	}
	if p.Type == "" {
		p.Type = "container"
	}
	if p.NetworkMode == "" {
		p.NetworkMode = "bridge"
	}
	if p.Status == "" {
		p.Status = "active"
	}
	if in.Stock != nil {
		p.Stock = normStock(*in.Stock)
	}
	if in.PerUserLimit != nil {
		p.PerUserLimit = normLimit(*in.PerUserLimit)
	}
	if err := db.DB.Create(&p).Error; err != nil {
		return err
	}
	if p.Stock == 0 {
		// gorm 对带 default 标签的零值字段会使用列默认值(-1)，库存 0（售罄）需显式写入
		db.DB.Model(&p).Update("stock", 0)
	}
	return nil
}

// UpdateProduct 更新商品
func UpdateProduct(id uint, in *ProductInput) error {
	var p models.Product
	if err := db.DB.First(&p, id).Error; err != nil {
		return fmt.Errorf("商品不存在")
	}
	if strings.TrimSpace(in.Type) == "" {
		in.Type = p.Type
	}
	// 类型或节点有变化时校验 VM 支持（未变化的旧 VM 商品允许继续编辑其它字段，但下单会被拒绝）
	if in.Type != p.Type || in.NodeID != p.NodeID {
		if err := validateProductType(in.NodeID, in.Type); err != nil {
			return err
		}
	}
	if in.NodeID != p.NodeID {
		if err := validateProductNode(in.NodeID); err != nil {
			return err
		}
		// 已售出实例不随商品迁移节点：实例的 node_id 保持开通时的节点
		var sold int64
		db.DB.Model(&models.UserProduct{}).Where("product_id = ? AND status NOT IN ?", id, []string{"failed"}).Count(&sold)
		if sold > 0 {
			logger.Warn("商品 #%d 所属节点 %d→%d，已有 %d 个实例仍保留在原节点", id, p.NodeID, in.NodeID, sold)
		}
	}
	updates := map[string]interface{}{
		"name":           in.Name,
		"description":    in.Description,
		"node_id":        in.NodeID,
		"region_id":      ProductRegionID(in.NodeID),
		"type":           in.Type,
		"image":          in.Image,
		"storage_pool":   in.StoragePool,
		"network_mode":   in.NetworkMode,
		"cpu":            in.CPU,
		"memory":         in.Memory,
		"disk":           in.Disk,
		"ingress":        in.Ingress,
		"egress":         in.Egress,
		"traffic_limit":  in.TrafficLimit,
		"ipv4_count":     in.IPv4Count,
		"ipv6_count":     in.IPv6Count,
		"port_mappings":  in.PortMappings,
		"allow_nesting":  in.AllowNesting,
		"memory_swap":    in.MemorySwap,
		"privileged":     in.Privileged,
		"backups":        in.Backups,
		"price_monthly":  in.PriceMonthly,
		"price_quarterly": in.PriceQuarterly,
		"price_half_year": in.PriceHalfYear,
		"price_yearly":   in.PriceYearly,
		"status":         in.Status,
		"sort_order":     in.SortOrder,
	}
	if in.Stock != nil {
		updates["stock"] = normStock(*in.Stock)
	}
	if in.PerUserLimit != nil {
		updates["per_user_limit"] = normLimit(*in.PerUserLimit)
	}
	return db.DB.Model(&p).Updates(updates).Error
}

// DeleteProduct 删除商品（存在未完成订单或实例时拒绝）
func DeleteProduct(id uint) error {
	var orders int64
	db.DB.Model(&models.Order{}).Where("product_id = ? AND status IN ?", id, []string{"pending", "paid"}).Count(&orders)
	if orders > 0 {
		return fmt.Errorf("该商品存在未完成订单，无法删除")
	}
	var ups int64
	db.DB.Model(&models.UserProduct{}).Where("product_id = ?", id).Count(&ups)
	if ups > 0 {
		return fmt.Errorf("该商品存在 %d 个用户实例，无法删除", ups)
	}
	return db.DB.Delete(&models.Product{}, id).Error
}

// normStock 库存规范化：负数统一为 -1（不限）
func normStock(v int) int {
	if v < 0 {
		return -1
	}
	return v
}

// normLimit 限购规范化：负数视为 0（不限）
func normLimit(v int) int {
	if v < 0 {
		return 0
	}
	return v
}

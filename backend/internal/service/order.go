package service

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"os/exec"
	"strings"
	"sync"
	"time"

	"lxdapi/internal/db"
	"lxdapi/internal/lxc"
	"lxdapi/models"
	"lxdapi/pkg/logger"

	"gorm.io/gorm"
)

var periodFactor = map[string]time.Duration{
	"monthly":   30 * 24 * time.Hour,
	"quarterly": 90 * 24 * time.Hour,
	"halfyear":  180 * 24 * time.Hour,
	"half_year": 180 * 24 * time.Hour,
	"yearly":    365 * 24 * time.Hour,
}

// PeriodPrice 获取周期价格
func PeriodPrice(p *models.Product, period string) float64 {
	switch period {
	case "quarterly":
		return p.PriceQuarterly
	case "halfyear", "half_year":
		return p.PriceHalfYear
	case "yearly":
		return p.PriceYearly
	default:
		return p.PriceMonthly
	}
}

func genOrderNo() string {
	b := make([]byte, 4)
	rand.Read(b)
	return fmt.Sprintf("NO%s%s", time.Now().Format("20060102150405"), hex.EncodeToString(b))
}

func genInstanceName(prefix string) string {
	b := make([]byte, 3)
	rand.Read(b)
	return fmt.Sprintf("%s-%s", strings.ToLower(prefix), hex.EncodeToString(b))
}

// genInstancePassword 自动开通实例的 root 密码：16 位字母数字（不含 shell 特殊字符，VM/节点开通会经 shell 传递）
func genInstancePassword() string {
	const charset = "abcdefghijkmnpqrstuvwxyzABCDEFGHJKLMNPQRSTUVWXYZ23456789"
	b := make([]byte, 16)
	rand.Read(b)
	for i := range b {
		b[i] = charset[int(b[i])%len(charset)]
	}
	return string(b)
}

// CreateOrder 创建订单（new=新购 / renew=续费）
func CreateOrder(userID, productID uint, period, typ string, userProductID uint) (*models.Order, error) {
	p, err := GetProduct(productID)
	if err != nil {
		return nil, err
	}
	// 新购仅限上架商品；续费允许已隐藏（下架）的商品，只有停用（disabled）的商品不再续费
	if typ == "renew" {
		if p.Status == "disabled" {
			return nil, fmt.Errorf("该商品已停止续费")
		}
	} else if p.Status != "active" {
		return nil, fmt.Errorf("商品已下架")
	}
	if typ == "" {
		typ = "new"
	}
	if _, ok := periodFactor[period]; !ok {
		return nil, fmt.Errorf("无效的购买周期")
	}
	price := PeriodPrice(p, period)
	if price < 0 {
		return nil, fmt.Errorf("该周期价格无效")
	}
	if price == 0 && !IsFreePeriod(p, period) {
		// 价格为 0 且商品其他周期有定价：视为该周期未开放，避免误开免费
		return nil, fmt.Errorf("该周期未开放购买")
	}

	var user models.User
	if err := db.DB.First(&user, userID).Error; err != nil {
		return nil, fmt.Errorf("用户不存在")
	}

	isNew := typ != "renew"
	if isNew {
		// 虚拟机类型商品只能由具备 KVM 的主控本机开通；节点不支持时拒绝下单，避免付款后开通失败
		if err := validateProductType(p.NodeID, p.Type); err != nil {
			return nil, fmt.Errorf("该商品暂不可购买：%v", err)
		}
	}
	// 节点快照：新购取商品所在节点；续费取实例所在节点（商品后来迁移节点不影响已售实例）
	orderNode := p.NodeID
	if !isNew && userProductID > 0 {
		var up models.UserProduct
		if err := db.DB.Select("id, node_id").First(&up, userProductID).Error; err == nil {
			orderNode = up.NodeID
		}
	}
	order := &models.Order{
		OrderNo:       genOrderNo(),
		UserID:        userID,
		ProductID:     productID,
		NodeID:        orderNode,
		UserProductID: userProductID,
		Type:          typ,
		Period:        period,
		Amount:        price,
		Status:        "pending",
	}
	if err := createOrderRecord(order, p, isNew); err != nil {
		return nil, err
	}

	// 0 元订单：无需支付，直接标记已支付并走与在线支付相同的开通流程
	if price == 0 {
		NotifyUser(userID, "order", "订单创建成功",
			fmt.Sprintf("订单号 %s 为免费订单，已自动完成支付，正在开通。", order.OrderNo), order.ID)
		paid, err := MarkOrderPaid(order.OrderNo, "free", "")
		if err != nil {
			return nil, fmt.Errorf("免费订单处理失败: %v", err)
		}
		return paid, nil
	}

	NotifyUser(userID, "order", "订单创建成功",
		fmt.Sprintf("订单号 %s，金额 ￥%.2f，请尽快完成支付。", order.OrderNo, order.Amount), order.ID)

	// 邮件通知（后台发送，失败或 SMTP 不可达不影响下单）
	go func(email string, data MailData) {
		_ = SendMailWithTemplate(email, MailTemplateOrderCreated, data)
	}(user.Email, MailData{
		"SiteName":    SiteName(),
		"Username":    user.Username,
		"OrderNo":     order.OrderNo,
		"ProductName": p.Name,
		"Amount":      fmt.Sprintf("%.2f", order.Amount),
	})

	return order, nil
}

// IsFreePeriod 0 元周期是否允许下单：商品全部周期均为 0（免费商品），或月付为 0（月付免费）
func IsFreePeriod(p *models.Product, period string) bool {
	if PeriodPrice(p, period) != 0 {
		return false
	}
	if p.PriceMonthly == 0 && p.PriceQuarterly == 0 && p.PriceHalfYear == 0 && p.PriceYearly == 0 {
		return true
	}
	return period == "monthly"
}

// GetOrderByNo 按订单号查询
func GetOrderByNo(orderNo string) (*models.Order, error) {
	var o models.Order
	if err := db.DB.Where("order_no = ?", orderNo).First(&o).Error; err != nil {
		return nil, fmt.Errorf("订单不存在")
	}
	return &o, nil
}

// ListUserOrders 用户订单列表
func ListUserOrders(userID uint) ([]models.Order, error) {
	var orders []models.Order
	if err := db.DB.Where("user_id = ?", userID).Order("id DESC").Find(&orders).Error; err != nil {
		return nil, err
	}
	return orders, nil
}

// MarkOrderPaid 支付成功回调：幂等更新订单并触发开通/续费
func MarkOrderPaid(orderNo, channel, tradeNo string) (*models.Order, error) {
	o, err := GetOrderByNo(orderNo)
	if err != nil {
		return nil, err
	}
	if o.Status == "paid" {
		return o, nil // 幂等
	}
	if o.Status != "pending" {
		return nil, fmt.Errorf("订单状态异常")
	}

	now := time.Now()
	// 条件更新保证只有一个回调能把订单从 pending 改为 paid（异步通知与同步跳转并发时不会重复开通）
	res := db.DB.Model(&models.Order{}).Where("id = ? AND status = ?", o.ID, "pending").Updates(map[string]interface{}{
		"status":       "paid",
		"pay_channel":  channel,
		"pay_trade_no": tradeNo,
		"paid_at":      now,
	})
	if res.Error != nil {
		return nil, res.Error
	}
	if res.RowsAffected == 0 {
		// 已被其他请求处理
		return GetOrderByNo(orderNo)
	}
	if o.Type != "renew" && o.Type != "recharge" && !o.StockReserved {
		// 升级前创建的未预占库存订单：支付时扣减（有限库存且已为 0 时仅记录告警，已付款仍照常开通）
		if ok, err := reserveStock(o.ProductID); err == nil && ok {
			var p models.Product
			if db.DB.Select("stock").First(&p, o.ProductID).Error == nil && p.Stock >= 0 {
				db.DB.Model(&models.Order{}).Where("id = ?", o.ID).Update("stock_reserved", true)
				o.StockReserved = true
			}
		} else {
			logger.Warn("订单 %s 支付时商品 #%d 库存已为 0（超卖），仍继续开通", o.OrderNo, o.ProductID)
		}
	}
	o.Status = "paid"
	o.PayChannel = channel
	o.PayTradeNo = tradeNo
	o.PaidAt = &now

	// 开通/续费：后台执行，避免创建容器耗时导致支付回调/下单请求超时
	go fulfillOrder(*o)
	return o, nil
}

// fulfillOrder 已支付订单的开通/续费（免费订单与在线支付共用）
func fulfillOrder(o models.Order) {
	defer func() {
		if r := recover(); r != nil {
			logger.Error("订单开通异常 order=%s: %v", o.OrderNo, r)
		}
	}()
	if o.Type == "recharge" {
		if _, err := creditRecharge(&o); err != nil {
			logger.Error("充值入账失败 order=%s: %v", o.OrderNo, err)
		}
		return
	}
	if o.Type == "renew" && o.UserProductID > 0 {
		if err := renewUserProduct(&o); err != nil {
			logger.Error("续费开通失败 order=%s: %v", o.OrderNo, err)
		}
		return
	}
	logger.Info("开始自动开通 order=%s", o.OrderNo)
	if err := provisionUserProduct(&o); err != nil {
		logger.Error("自动开通失败 order=%s: %v", o.OrderNo, err)
		return
	}
	logger.OK("自动开通完成 order=%s", o.OrderNo)
}

// provisionUserProduct 自动开通新实例
func provisionUserProduct(o *models.Order) error {
	p, err := GetProduct(o.ProductID)
	if err != nil {
		return err
	}
	var user models.User
	if err := db.DB.First(&user, o.UserID).Error; err != nil {
		return err
	}

	// 商品未配置镜像时使用默认镜像（否则 lxc init 镜像参数为空必然失败）
	if strings.TrimSpace(p.Image) == "" {
		p.Image = DefaultImage()
		logger.Warn("商品 #%d 未配置系统镜像，使用默认镜像 %s", p.ID, p.Image)
	}

	name := genInstanceName("vm")
	if p.Type == "container" {
		name = genInstanceName("ct")
	}
	password := genInstancePassword()
	expire := time.Now().Add(periodFactor[o.Period]).Round(0)

	up := &models.UserProduct{
		UserID:    user.ID,
		ProductID: p.ID,
		NodeID:    p.NodeID,
		Name:      name,
		Type:      p.Type,
		Status:    "creating",
		ConfigJSON: fmt.Sprintf(`{"image":%q,"cpu":%d,"memory":%d,"disk":%d,"traffic_limit":%d,"password":%q,"ingress":%d,"egress":%d,"allow_nesting":%v,"memory_swap":%v,"privileged":%v}`,
			p.Image, p.CPU, p.Memory, p.Disk, p.TrafficLimit, password, p.Ingress, p.Egress, p.AllowNesting, p.MemorySwap, p.Privileged),
		ExpireAt: &expire,
	}
	if err := db.DB.Create(up).Error; err != nil {
		return err
	}
	// 先关联订单与实例记录（开通失败时实例状态为 failed，不再计入限购）
	db.DB.Model(o).Update("user_product_id", up.ID)

	// 登记为本进程开通中：开通核对任务跳过它（主控重启后此登记消失，由核对任务向节点补齐状态）
	markProvisioning(up.ID, true)
	defer markProvisioning(up.ID, false)

	// 创建实例
	if err := createInstanceOnNode(p, up, name, password, user.Username); err != nil {
		failProvision(up, err.Error())
		return err
	}

	if !activateProvision(up) {
		// 已被核对任务处理（极少见）：以数据库当前状态为准。
		// 注意：此时节点/节点上的容器可能已创建成功，成为孤儿容器占用资源——打 Error 日志便于事后核查清理。
		logger.Error("开通竞态：实例 %d（%s）在开通成功后未能翻为 active（核对任务已先处理），请检查节点上是否有孤儿容器", up.ID, name)
		return nil
	}

	NotifyUser(user.ID, "product", "实例开通成功",
		fmt.Sprintf("实例 %s 已开通，到期时间 %s。", name, expire.Format("2006-01-02 15:04")), up.ID)
	_ = SendMailWithTemplate(user.Email, MailTemplateInstanceCreated, MailData{
		"SiteName":     SiteName(),
		"Username":     user.Username,
		"InstanceName": name,
		"ExpireAt":     expire.Format("2006-01-02 15:04"),
	})
	return nil
}

// productCreateRequest 由商品规格构建容器创建请求（本机与节点共用，保证两边参数一致）
func productCreateRequest(p *models.Product, name, password, username string) *models.CreateContainerRequest {
	return &models.CreateContainerRequest{
		Name:             name,
		Password:         password,
		Image:            p.Image,
		Username:         username, // 归属下单用户，用户面板才能看到并管理
		CPU:              p.CPU,
		Memory:           p.Memory,
		Disk:             productDiskMB(p.Disk), // 商品 GB → 创建接口 MB
		Ingress:          p.Ingress,
		Egress:           p.Egress,
		TrafficLimit:     int(p.TrafficLimit),
		AllowNesting:     p.AllowNesting,
		MemorySwap:       p.MemorySwap,
		Privileged:       p.Privileged,
		StoragePool:      strings.TrimSpace(p.StoragePool),
		IPv4PoolLimit:    p.IPv4Count,
		IPv6PoolLimit:    p.IPv6Count,
		IPv4MappingLimit: p.PortMappings,
		IPv6MappingLimit: p.PortMappings,
	}
}

// nodeProvisionWait 节点开通任务最长等待时间
var nodeProvisionWait = 10 * time.Minute

// createInstanceOnNode 在本机或节点创建容器/VM。
// 实例归属以 up.NodeID（开通时快照）为准；节点开通为异步任务，这里轮询任务直到成功/失败。
func createInstanceOnNode(p *models.Product, up *models.UserProduct, name, password, username string) error {
	if up.NodeID != 0 {
		node, err := GetNode(up.NodeID)
		if err != nil {
			return err
		}
		if p.Type == "vm" {
			return errors.New(ErrNodeVMUnsupported)
		}
		nc := NewNodeClient(node)
		req := productCreateRequest(p, name, password, username)
		// 商品镜像可能指向节点上已被删除的本地镜像：开通前刷新节点镜像并核对，不可用则回退默认镜像
		if !nodeImagesUsable(node, req.Image) {
			fb := fallbackImage()
			logger.Warn("镜像 %s 在节点 %s 上不可用，回退为 %s 开通", req.Image, node.Name, fb)
			req.Image = fb
		}
		taskID, err := nc.CreateContainerReq(req)
		if err != nil {
			return err
		}
		if taskID == 0 {
			return nil // 旧版节点不返回任务 ID
		}
		// 记录节点任务 ID：主控若在等待期间重启，核对任务据此查询节点结果
		up.ProvisionTaskID = taskID
		db.DB.Model(&models.UserProduct{}).Where("id = ?", up.ID).Update("provision_task_id", taskID)
		return nc.WaitTask(taskID, nodeProvisionWait)
	}
	// 本机
	if !LocalLXDAvailable() {
		return fmt.Errorf("主控本机未安装 LXD，商品需配置所属节点")
	}
	if p.Type == "vm" {
		if !localVMSupported() {
			return fmt.Errorf("主控本机不支持虚拟机（缺少 /dev/kvm）")
		}
		return createLocalVM(p, name, password)
	}
	return createLocalContainer(p, name, password, username)
}

// createLocalContainer 本机创建容器（复用 ContainerService）
func createLocalContainer(p *models.Product, name, password, username string) error {
	return NewContainerService().Create(context.Background(), productCreateRequest(p, name, password, username))
}

// productDiskMB 商品硬盘单位为 GB，容器创建接口单位为 MB。
// 兼容旧数据：数值 >= 1024 时视为当初按 MB 录入，原样使用。
func productDiskMB(disk int) int {
	if disk <= 0 {
		return 0
	}
	if disk >= 1024 {
		return disk
	}
	return disk * 1024
}

// createLocalVM 本机创建 VM（依赖 /dev/kvm；LXD 原生 VM）
func createLocalVM(p *models.Product, name, password string) error {
	if !lxc.BinaryAvailable() {
		return lxc.ErrLXDNotInstalled
	}
	args := []string{"init", p.Image, name, "--vm",
		"-c", fmt.Sprintf("limits.cpu=%d", p.CPU),
		"-c", fmt.Sprintf("limits.memory=%dMB", p.Memory),
	}
	if p.Disk > 0 {
		args = append(args, "-d", fmt.Sprintf("root,size=%dGB", p.Disk))
	}
	cmd := exec.Command("lxc", args...)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("创建 VM 失败: %s", string(out))
	}
	if password != "" {
		cmd = exec.Command("lxc", "exec", name, "--", "bash", "-c",
			fmt.Sprintf("echo root:%s | chpasswd && sed -i 's/PasswordAuthentication no/PasswordAuthentication yes/' /etc/ssh/sshd_config && systemctl restart sshd || true", password))
		cmd.CombinedOutput()
	}
	exec.Command("lxc", "start", name).Run()
	return nil
}

// renewUserProduct 续费：延长到期时间并恢复实例
func renewUserProduct(o *models.Order) error {
	var up models.UserProduct
	if err := db.DB.First(&up, o.UserProductID).Error; err != nil {
		return fmt.Errorf("实例不存在")
	}
	base := time.Now()
	if up.ExpireAt != nil && up.ExpireAt.After(base) {
		base = *up.ExpireAt
	}
	newExpire := base.Add(periodFactor[o.Period])
	up.ExpireAt = &newExpire
	if up.Status == "suspended" || up.Status == "expired" {
		up.Status = "active"
		// 尝试恢复实例（本机 lxc start；节点通过 client）；失败（如节点离线）时登记待重试，由定时任务补做
		if err := resumeInstance(&up); err != nil {
			up.PendingAction = "start"
			logger.Warn("续费恢复实例 %s 失败，已加入重试队列: %v", up.Name, err)
		} else {
			up.PendingAction = ""
		}
	}
	if err := db.DB.Save(&up).Error; err != nil {
		return err
	}
	var user models.User
	if err := db.DB.First(&user, o.UserID).Error; err == nil {
		NotifyUser(user.ID, "product", "续费成功",
			fmt.Sprintf("实例 %s 已续费至 %s。", up.Name, newExpire.Format("2006-01-02 15:04")), up.ID)
	}
	return nil
}

// resumeInstance 启动实例；已在运行视为成功
func resumeInstance(up *models.UserProduct) error {
	var err error
	if up.NodeID == 0 {
		if !LocalLXDAvailable() {
			return fmt.Errorf("主控本机未安装 LXD")
		}
		// lxc start 加超时：socket 异常时可能 hang 住，不能无限等待
		startCtx, startCancel := context.WithTimeout(context.Background(), 60*time.Second)
		out, e := exec.CommandContext(startCtx, "lxc", "start", up.Name).CombinedOutput()
		startCancel()
		if e != nil {
			err = fmt.Errorf("%v: %s", e, strings.TrimSpace(string(out)))
		}
	} else {
		node, nerr := GetNode(up.NodeID)
		if nerr != nil {
			return nerr
		}
		err = NewNodeClient(node).ContainerAction(up.Name, "start")
	}
	if err != nil && alreadyRunning(err) {
		return nil
	}
	return err
}

// alreadyRunning 启动操作返回「已在运行」类错误
func alreadyRunning(err error) bool {
	if err == nil {
		return false
	}
	m := strings.ToLower(err.Error())
	return strings.Contains(m, "already running") || strings.Contains(m, "已在运行") || strings.Contains(m, "正在运行")
}

// ListUserProducts 用户实例列表
func ListUserProducts(userID uint) ([]models.UserProduct, error) {
	var ups []models.UserProduct
	if err := db.DB.Where("user_id = ?", userID).Order("id DESC").Find(&ups).Error; err != nil {
		return nil, err
	}
	return ups, nil
}

// GetUserProduct 用户实例
func GetUserProduct(userID, id uint) (*models.UserProduct, error) {
	var up models.UserProduct
	if err := db.DB.Where("id = ? AND user_id = ?", id, userID).First(&up).Error; err != nil {
		return nil, fmt.Errorf("实例不存在")
	}
	return &up, nil
}

// DeleteUserProduct 删除用户实例（删除容器）
func DeleteUserProduct(userID, id uint) error {
	up, err := GetUserProduct(userID, id)
	if err != nil {
		return err
	}
	if up.NodeID == 0 {
		_ = deleteLocalContainer(up.Name, up.Type == "vm")
	} else {
		node, nerr := GetNode(up.NodeID)
		if nerr == nil {
			nc := NewNodeClient(node)
			_ = nc.DeleteContainer(up.Name)
		}
	}
	return db.DB.Delete(up).Error
}

// nodeGone 节点返回「容器不存在」类错误：视为目标已达成（已删除/不存在）
func nodeGone(err error) bool {
	if err == nil {
		return false
	}
	m := strings.ToLower(err.Error())
	return strings.Contains(m, "不存在") || strings.Contains(m, "not found")
}

// lifecycleRound 一轮生命周期处理中记录调用失败的节点，同一轮跳过该节点其余实例，
// 避免一个离线节点（每次请求最长 120s 超时）拖住整个调度器。
type lifecycleRound struct{ failed map[uint]bool }

func newLifecycleRound() *lifecycleRound { return &lifecycleRound{failed: map[uint]bool{}} }

// nodeCall 对实例所在节点执行操作；返回 skipped=true 表示本轮跳过（节点已失败）
func (r *lifecycleRound) nodeCall(nodeID uint, fn func(nc *NodeClient) error) (skipped bool, err error) {
	if r.failed[nodeID] {
		return true, nil
	}
	node, err := GetNode(nodeID)
	if err != nil {
		return false, err
	}
	err = fn(NewNodeClient(node))
	if err != nil && !nodeGone(err) && strings.Contains(err.Error(), "节点连接失败") {
		r.failed[nodeID] = true
	}
	return false, err
}

// SuspendExpiredProducts 到期暂停（宽限期后删除由定时任务调用）。
// 节点实例只有在节点确认停机（或容器已不存在）后才标记 suspended，节点不可达时下一轮重试。
func SuspendExpiredProducts() {
	now := time.Now()
	var ups []models.UserProduct
	db.DB.Where("status = ? AND expire_at < ?", "active", now).Find(&ups)
	round := newLifecycleRound()
	for i := range ups {
		up := &ups[i]
		if up.NodeID == 0 {
			if LocalLXDAvailable() {
				exec.Command("lxc", "stop", up.Name).Run()
			}
		} else {
			skipped, err := round.nodeCall(up.NodeID, func(nc *NodeClient) error { return nc.ContainerAction(up.Name, "stop") })
			if skipped {
				continue
			}
			if err != nil && !nodeGone(err) && !strings.Contains(err.Error(), "已停止") && !strings.Contains(strings.ToLower(err.Error()), "already stopped") {
				logger.Warn("到期停机失败（节点#%d %s），下轮重试: %v", up.NodeID, up.Name, err)
				continue
			}
		}
		up.Status = "suspended"
		up.PendingAction = "" // 再次到期：不再重试续费恢复
		if err := db.DB.Save(up).Error; err != nil {
			continue
		}
		var user models.User
		if err := db.DB.First(&user, up.UserID).Error; err == nil {
			NotifyUser(user.ID, "product", "实例已暂停", fmt.Sprintf("实例 %s 因到期未续费已暂停，请尽快续费。", up.Name), up.ID)
			_ = SendMailWithTemplate(user.Email, MailTemplateSuspended, MailData{
				"SiteName": SiteName(), "Username": user.Username, "InstanceName": up.Name,
			})
		}
	}
}

// DeleteGracePeriodExpired 删除超过宽限期的实例（默认宽限 7 天）。
// 节点删除成功（或容器已不存在）才删除主控记录，否则保留记录下轮重试，避免节点上留下孤儿容器。
func DeleteGracePeriodExpired(graceDays int) {
	if graceDays <= 0 {
		graceDays = 7
	}
	cutoff := time.Now().Add(-time.Duration(graceDays) * 24 * time.Hour)
	// 另要求实例至少已暂停 24 小时（updated_at 为暂停时间）：节点长时间离线恢复后，
	// 不会在同一轮里刚暂停就被删除，给用户留出续费时间
	suspendedBefore := time.Now().Add(-24 * time.Hour)
	var ups []models.UserProduct
	db.DB.Where("status = ? AND expire_at < ? AND updated_at < ?", "suspended", cutoff, suspendedBefore).Find(&ups)
	round := newLifecycleRound()
	for i := range ups {
		up := &ups[i]
		if up.NodeID == 0 {
			if LocalLXDAvailable() {
				if err := deleteLocalContainer(up.Name, up.Type == "vm"); err != nil && !nodeGone(err) {
					logger.Warn("宽限期删除本机实例 %s 失败，下轮重试: %v", up.Name, err)
					continue
				}
			}
		} else {
			skipped, err := round.nodeCall(up.NodeID, func(nc *NodeClient) error { return nc.DeleteContainer(up.Name) })
			if skipped {
				continue
			}
			if err != nil && !nodeGone(err) {
				logger.Warn("宽限期删除失败（节点#%d %s），下轮重试: %v", up.NodeID, up.Name, err)
				continue
			}
		}
		db.DB.Delete(up)
	}
}

// ListAllUserProducts 全部用户实例（管理端）
func ListAllUserProducts(status string) ([]models.UserProduct, error) {
	var ups []models.UserProduct
	q := db.DB
	if status != "" {
		q = q.Where("status = ?", status)
	}
	if err := q.Order("id DESC").Find(&ups).Error; err != nil {
		return nil, err
	}
	return ups, nil
}

// FindUserProductByName 按名称查找实例
func FindUserProductByName(name string) (*models.UserProduct, error) {
	var up models.UserProduct
	if err := db.DB.Where("name = ?", name).First(&up).Error; err != nil {
		return nil, err
	}
	return &up, nil
}

// GetUserOrderByID 用户订单详情（校验归属）
func GetUserOrderByID(userID, id uint) (*models.Order, error) {
	var o models.Order
	if err := db.DB.Where("id = ? AND user_id = ?", id, userID).First(&o).Error; err != nil {
		return nil, fmt.Errorf("订单不存在")
	}
	return &o, nil
}

// CancelUserOrder 用户取消订单
func CancelUserOrder(userID, id uint) error {
	var o models.Order
	if err := db.DB.Where("id = ? AND user_id = ? AND status = ?", id, userID, "pending").First(&o).Error; err != nil {
		return fmt.Errorf("订单不存在或不可取消")
	}
	// 条件更新：避免与支付回调并发时把已支付订单改为取消
	res := db.DB.Model(&models.Order{}).Where("id = ? AND status = ?", o.ID, "pending").Update("status", "cancelled")
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return fmt.Errorf("订单不存在或不可取消")
	}
	if o.StockReserved {
		releaseStock(o.ProductID)
		db.DB.Model(&models.Order{}).Where("id = ?", o.ID).Update("stock_reserved", false)
	}
	return nil
}

var newOrderMu sync.Mutex

// createOrderRecord 写入订单；新购订单在锁内完成限购校验与库存预占（仅数据库操作，不含任何网络 IO）
func createOrderRecord(order *models.Order, p *models.Product, isNew bool) error {
	if !isNew {
		return db.DB.Create(order).Error
	}
	// 同一时刻只处理一个新购下单，保证限购计数与库存预占不会因并发被绕过
	newOrderMu.Lock()
	defer newOrderMu.Unlock()
	if p.PerUserLimit > 0 {
		held := CountUserHolding(order.UserID, p.ID)
		if held >= int64(p.PerUserLimit) {
			return fmt.Errorf("每人限购 %d 个，您已购买 %d 个", p.PerUserLimit, held)
		}
	}
	ok, err := reserveStock(p.ID)
	if err != nil {
		return err
	}
	if !ok {
		return fmt.Errorf("库存不足，商品已售罄")
	}
	var cur models.Product
	if db.DB.Select("stock").First(&cur, p.ID).Error == nil && cur.Stock >= 0 {
		order.StockReserved = true
	}
	if err := db.DB.Create(order).Error; err != nil {
		if order.StockReserved {
			releaseStock(p.ID)
		}
		return err
	}
	return nil
}

// reserveStock 原子扣减库存：库存 <0（不限）时不扣减直接成功；>0 时减 1；=0 时失败
func reserveStock(productID uint) (bool, error) {
	res := db.DB.Model(&models.Product{}).Where("id = ? AND stock > 0", productID).
		UpdateColumn("stock", gorm.Expr("stock - 1"))
	if res.Error != nil {
		return false, res.Error
	}
	if res.RowsAffected > 0 {
		return true, nil
	}
	var p models.Product
	if err := db.DB.Select("stock").First(&p, productID).Error; err != nil {
		return false, fmt.Errorf("商品不存在")
	}
	return p.Stock < 0, nil
}

// releaseStock 归还一个库存（仅对有限库存生效）
func releaseStock(productID uint) {
	db.DB.Model(&models.Product{}).Where("id = ? AND stock >= 0", productID).
		UpdateColumn("stock", gorm.Expr("stock + 1"))
}

// CountUserHolding 用户当前持有的某商品数量：未删除且未开通失败的实例 + 待支付新购订单 + 已支付但尚未生成实例的新购订单
func CountUserHolding(userID, productID uint) int64 {
	var ups, pending, provisioning int64
	db.DB.Model(&models.UserProduct{}).Where("user_id = ? AND product_id = ? AND status <> ?", userID, productID, "failed").Count(&ups)
	db.DB.Model(&models.Order{}).Where("user_id = ? AND product_id = ? AND type <> ? AND status = ?", userID, productID, "renew", "pending").Count(&pending)
	db.DB.Model(&models.Order{}).Where("user_id = ? AND product_id = ? AND type <> ? AND status = ? AND user_product_id = 0", userID, productID, "renew", "paid").Count(&provisioning)
	return ups + pending + provisioning
}

// deleteLocalContainer 本机删除容器/VM。
// 容器走 ContainerService.Delete，同时清理容器记录、端口映射、IP 绑定、流量与访问凭证，避免后台残留孤儿记录。
func deleteLocalContainer(name string, isVM bool) error {
	if !isVM {
		return NewContainerService().Delete(context.Background(), name)
	}
	return exec.Command("lxc", "delete", "-f", name).Run()
}

// AdminDeleteUserProduct 管理员删除实例
func AdminDeleteUserProduct(id uint) error {
	var up models.UserProduct
	if err := db.DB.First(&up, id).Error; err != nil {
		return fmt.Errorf("实例不存在")
	}
	// 删除底层容器/VM（节点或本机）
	if up.Name != "" {
		isVM := up.Type == "vm"
		if up.NodeID > 0 {
			_ = NodeClientDelete(up.NodeID, up.Name, isVM)
		} else {
			_ = deleteLocalContainer(up.Name, isVM)
		}
	}
	return db.DB.Delete(&up).Error
}

// StartLifecycleScheduler 实例生命周期定时任务：每分钟暂停到期实例、清理超宽限期实例
func StartLifecycleScheduler() {
	defer func() {
		if r := recover(); r != nil {
			logger.Error("生命周期调度器异常: %v", r)
		}
	}()
	ticker := time.NewTicker(time.Minute)
	// 启动时立即核对一次：上次运行在开通等待期间被重启而停在 creating 的实例
	RecoverStuckProvisioning()
	for range ticker.C {
		AutoRenewDueProducts()
		SuspendExpiredProducts()
		DeleteGracePeriodExpired(7)
		RecoverStuckProvisioning()
		RetryPendingActions()
	}
}

// CountActiveUserProducts 运行中实例数
func CountActiveUserProducts() int64 {
	var cnt int64
	db.DB.Model(&models.UserProduct{}).Where("status = ?", "active").Count(&cnt)
	return cnt
}

// CountProductSold 商品销量
func CountProductSold(productID uint) int64 {
	var cnt int64
	db.DB.Model(&models.Order{}).Where("product_id = ? AND status = ?", productID, "paid").Count(&cnt)
	return cnt
}

// nodeImagesUsable 校验镜像在节点上是否可用：先快速核对一次，命中则直接返回；
// 未命中时触发节点同步 LXD 镜像模板（剔除已被删除的失效镜像）后再核对一次。
// 非指纹（别名/remote:alias）交由节点 LXD 解析，一律视为可用；无法确认时保持原值不动。
// 注意：即使此处误判为可用，节点侧 Create 的镜像回退重试仍是最后一道防线。
func nodeImagesUsable(node *models.Node, image string) bool {
	image = strings.TrimSpace(image)
	if image == "" || !imageFingerprintRe.MatchString(image) {
		return true
	}
	// 快速路径：命中则直接可用，避免每次开通都触发一次全量模板同步
	if nodeTemplateHasFingerprint(node, image) {
		return true
	}
	nc := NewNodeClientTimeout(node, 30*time.Second)
	if _, err := nc.do("POST", "/api/admin/templates/sync", nil, nil); err != nil {
		logger.Warn("节点模板同步失败: %v", err)
	}
	return nodeTemplateHasFingerprint(node, image)
}

// nodeTemplateHasFingerprint 读取节点镜像模板列表并核对指纹；读不到时视为可用（保持原值不动）
func nodeTemplateHasFingerprint(node *models.Node, image string) bool {
	res, err := nodeGetJSON(node, "/api/admin/templates", nil, 30*time.Second)
	if err != nil {
		logger.Warn("读取节点镜像列表失败，跳过镜像校验: %v", err)
		return true
	}
	if data, ok := res["data"].(map[string]interface{}); ok {
		if tpls, ok := data["templates"].([]interface{}); ok {
			for _, t := range tpls {
				if m, ok := t.(map[string]interface{}); ok && m["fingerprint"] == image {
					return true
				}
			}
		}
	}
	return false
}

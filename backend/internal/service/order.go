package service

import (
	"context"
	"crypto/rand"
	"encoding/hex"
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
	if p.Status != "active" {
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
	order := &models.Order{
		OrderNo:       genOrderNo(),
		UserID:        userID,
		ProductID:     productID,
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
	if o.Type != "renew" && !o.StockReserved {
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

	// 创建实例
	if err := createInstanceOnNode(p, up, name, password, user.Username); err != nil {
		up.Status = "failed"
		db.DB.Save(up)
		if o.StockReserved {
			releaseStock(p.ID)
			db.DB.Model(&models.Order{}).Where("id = ?", o.ID).Update("stock_reserved", false)
		}
		NotifyUser(user.ID, "product", "实例开通失败", fmt.Sprintf("实例 %s 开通失败：%v", name, err), up.ID)
		return err
	}

	now := time.Now()
	up.Status = "active"
	up.ProvisionedAt = &now
	if err := db.DB.Save(up).Error; err != nil {
		return err
	}

	// 关联订单与实例
	db.DB.Model(o).Update("user_product_id", up.ID)

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

// createInstanceOnNode 在本机或节点创建容器/VM
func createInstanceOnNode(p *models.Product, up *models.UserProduct, name, password, username string) error {
	if p.NodeID != 0 {
		node, err := GetNode(p.NodeID)
		if err != nil {
			return err
		}
		nc := NewNodeClient(node)
		return nc.CreateContainer(name, p.Image, password, p.CPU, p.Memory, p.Disk)
	}
	// 本机
	if p.Type == "vm" {
		return createLocalVM(p, name, password)
	}
	return createLocalContainer(p, name, password, username)
}

// createLocalContainer 本机创建容器（复用 ContainerService）
func createLocalContainer(p *models.Product, name, password, username string) error {
	svc := NewContainerService()
	req := &models.CreateContainerRequest{
		Name:         name,
		Password:     password,
		Image:        p.Image,
		Username:     username, // 归属下单用户，用户面板才能看到并管理
		CPU:          p.CPU,
		Memory:       p.Memory,
		Disk:         productDiskMB(p.Disk),
		Ingress:      p.Ingress,
		Egress:       p.Egress,
		TrafficLimit: int(p.TrafficLimit),
		AllowNesting: p.AllowNesting,
		MemorySwap:   p.MemorySwap,
		Privileged:   p.Privileged,
	}
	return svc.Create(context.Background(), req)
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
		// 尝试恢复实例（本机 lxc start；节点通过 client）
		resumeInstance(&up)
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

func resumeInstance(up *models.UserProduct) {
	if up.NodeID == 0 {
		exec.Command("lxc", "start", up.Name).Run()
		return
	}
	node, err := GetNode(up.NodeID)
	if err != nil {
		return
	}
	nc := NewNodeClient(node)
	_ = nc.ContainerAction(up.Name, "start")
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

// SuspendExpiredProducts 到期暂停（宽限期后删除由定时任务调用）
func SuspendExpiredProducts() {
	now := time.Now()
	var ups []models.UserProduct
	db.DB.Where("status = ? AND expire_at < ?", "active", now).Find(&ups)
	for i := range ups {
		up := &ups[i]
		up.Status = "suspended"
		if err := db.DB.Save(up).Error; err != nil {
			continue
		}
		if up.NodeID == 0 {
			exec.Command("lxc", "stop", up.Name).Run()
		} else if node, err := GetNode(up.NodeID); err == nil {
			_ = NewNodeClient(node).ContainerAction(up.Name, "stop")
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

// DeleteGracePeriodExpired 删除超过宽限期的实例（默认宽限 7 天）
func DeleteGracePeriodExpired(graceDays int) {
	if graceDays <= 0 {
		graceDays = 7
	}
	cutoff := time.Now().Add(-time.Duration(graceDays) * 24 * time.Hour)
	var ups []models.UserProduct
	db.DB.Where("status = ? AND expire_at < ?", "suspended", cutoff).Find(&ups)
	for i := range ups {
		up := &ups[i]
		if up.NodeID == 0 {
			exec.Command("lxc", "delete", "-f", up.Name).Run()
		} else if node, err := GetNode(up.NodeID); err == nil {
			_ = NewNodeClient(node).DeleteContainer(up.Name)
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
	for range ticker.C {
		SuspendExpiredProducts()
		DeleteGracePeriodExpired(7)
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

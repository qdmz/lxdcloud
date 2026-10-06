package service

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"lxdapi/internal/db"
	"lxdapi/models"
	"lxdapi/pkg/logger"
)

var periodFactor = map[string]time.Duration{
	"monthly":   30 * 24 * time.Hour,
	"quarterly": 90 * 24 * time.Hour,
	"halfyear":  180 * 24 * time.Hour,
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
	price := PeriodPrice(p, period)
	if price <= 0 {
		return nil, fmt.Errorf("该周期价格无效")
	}

	var user models.User
	if err := db.DB.First(&user, userID).Error; err != nil {
		return nil, fmt.Errorf("用户不存在")
	}

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
	if err := db.DB.Create(order).Error; err != nil {
		return nil, err
	}

	NotifyUser(userID, "order", "订单创建成功",
		fmt.Sprintf("订单号 %s，金额 ￥%.2f，请尽快完成支付。", order.OrderNo, order.Amount), order.ID)

	// 邮件通知（失败不影响下单）
	_ = SendMailWithTemplate(user.Email, MailTemplateOrderCreated, MailData{
		"SiteName":    SiteName(),
		"Username":    user.Username,
		"OrderNo":     order.OrderNo,
		"ProductName": p.Name,
		"Amount":      fmt.Sprintf("%.2f", order.Amount),
	})

	return order, nil
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
	o.Status = "paid"
	o.PayChannel = channel
	o.PayTradeNo = tradeNo
	o.PaidAt = &now
	if err := db.DB.Save(o).Error; err != nil {
		return nil, err
	}

	// 开通/续费
	if o.Type == "renew" && o.UserProductID > 0 {
		if err := renewUserProduct(o); err != nil {
			logger.Error("续费开通失败 order=%s: %v", o.OrderNo, err)
		}
	} else {
		if err := provisionUserProduct(o); err != nil {
			logger.Error("自动开通失败 order=%s: %v", o.OrderNo, err)
		}
	}
	return o, nil
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

	name := genInstanceName("vm")
	if p.Type == "container" {
		name = genInstanceName("ct")
	}
	password := genInstanceName("pwd")
	expire := time.Now().Add(periodFactor[o.Period])

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

	// 同步创建实例
	if err := createInstanceOnNode(p, up, name, password); err != nil {
		up.Status = "failed"
		db.DB.Save(up)
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
func createInstanceOnNode(p *models.Product, up *models.UserProduct, name, password string) error {
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
	return createLocalContainer(p, name, password)
}

// createLocalContainer 本机创建容器（复用 ContainerService）
func createLocalContainer(p *models.Product, name, password string) error {
	svc := NewContainerService()
	req := &models.CreateContainerRequest{
		Name:         name,
		Password:     password,
		Image:        p.Image,
		CPU:          p.CPU,
		Memory:       p.Memory,
		Disk:         p.Disk,
		Ingress:      p.Ingress,
		Egress:       p.Egress,
		TrafficLimit: int(p.TrafficLimit),
		AllowNesting: p.AllowNesting,
		MemorySwap:   p.MemorySwap,
		Privileged:   p.Privileged,
	}
	return svc.Create(context.Background(), req)
}

// createLocalVM 本机创建 VM（依赖 /dev/kvm；LXD 原生 VM）
func createLocalVM(p *models.Product, name, password string) error {
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
		exec.Command("lxc", "delete", "-f", up.Name).Run()
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
	o.Status = "cancelled"
	return db.DB.Save(&o).Error
}

// deleteLocalContainer 本机删除容器/VM
func deleteLocalContainer(name string, isVM bool) error {
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

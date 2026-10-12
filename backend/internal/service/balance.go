package service

import (
	"fmt"
	"math"
	"time"

	"lxdapi/internal/db"
	"lxdapi/pkg/logger"
	"lxdapi/models"

	"gorm.io/gorm"
)

func round2(v float64) float64 { return math.Round(v*100) / 100 }

// GetUserBalance 当前余额
func GetUserBalance(userID uint) float64 {
	var u models.User
	if err := db.DB.Select("id", "balance").First(&u, userID).Error; err != nil {
		return 0
	}
	return round2(u.Balance)
}

// RechargeMin / RechargeMax 单笔在线充值金额范围（元）
const (
	RechargeMin = 1.0
	RechargeMax = 50000.0
)

// CreateRechargeOrder 创建在线充值订单（type=recharge，product_id=0）
func CreateRechargeOrder(userID uint, amount float64) (*models.Order, error) {
	amount = round2(amount)
	if amount < RechargeMin || amount > RechargeMax {
		return nil, fmt.Errorf("充值金额需在 ￥%.0f ~ ￥%.0f 之间", RechargeMin, RechargeMax)
	}
	o := &models.Order{OrderNo: genOrderNo(), UserID: userID, Type: "recharge", Period: "", Amount: amount, Status: "pending"}
	if err := db.DB.Create(o).Error; err != nil {
		return nil, err
	}
	return o, nil
}

// creditRecharge 充值订单支付成功后入账（由 MarkOrderPaid 的条件更新保证只执行一次）
func creditRecharge(o *models.Order) (float64, error) {
	var exists int64
	db.DB.Model(&models.BalanceLog{}).Where("order_no = ? AND type = ?", o.OrderNo, "recharge").Count(&exists)
	if exists > 0 {
		return GetUserBalance(o.UserID), nil
	}
	return adjustBalance(o.UserID, o.Amount, "recharge", "在线充值", o.OrderNo)
}

// ListAllBalanceLogs 管理端：某用户的余额流水
func ListAllBalanceLogs(userID uint, limit int) []models.BalanceLog { return ListBalanceLogs(userID, limit) }

// AdjustBalance 管理员调整余额（充值为正，扣减为负），记录流水
func AdjustBalance(userID uint, amount float64, typ, remark string) (float64, error) {
	return adjustBalance(userID, amount, typ, remark, "")
}

func adjustBalance(userID uint, amount float64, typ, remark, orderNo string) (float64, error) {
	amount = round2(amount)
	if amount == 0 {
		return 0, fmt.Errorf("金额不能为 0")
	}
	if typ == "" {
		if amount > 0 {
			typ = "recharge"
		} else {
			typ = "adjust"
		}
	}
	var after float64
	err := db.DB.Transaction(func(tx *gorm.DB) error {
		q := tx.Model(&models.User{}).Where("id = ?", userID)
		if amount < 0 {
			q = q.Where("balance >= ?", -amount)
		}
		res := q.UpdateColumn("balance", gorm.Expr("balance + ?", amount))
		if res.Error != nil {
			return res.Error
		}
		if res.RowsAffected == 0 {
			return fmt.Errorf("用户不存在或余额不足")
		}
		var u models.User
		if err := tx.Select("id", "balance").First(&u, userID).Error; err != nil {
			return err
		}
		after = round2(u.Balance)
		return tx.Create(&models.BalanceLog{UserID: userID, Amount: amount, Balance: after, Type: typ, Remark: remark, OrderNo: orderNo}).Error
	})
	if err != nil {
		return 0, err
	}
	if amount > 0 {
		NotifyUser(userID, "order", "余额到账", fmt.Sprintf("账户余额增加 ￥%.2f，当前余额 ￥%.2f。%s", amount, after, remark), 0)
	}
	return after, nil
}

// PayOrderByBalance 使用余额支付订单：原子扣款 → 标记已支付 → 开通/续费；失败自动退回
func PayOrderByBalance(userID uint, o *models.Order) (*models.Order, error) {
	if o.UserID != userID {
		return nil, fmt.Errorf("订单不存在")
	}
	if o.Status != "pending" {
		return nil, fmt.Errorf("订单已支付或已关闭")
	}
	if o.Type == "recharge" {
		return nil, fmt.Errorf("充值订单不能使用余额支付")
	}
	amount := round2(o.Amount)
	var after float64
	err := db.DB.Transaction(func(tx *gorm.DB) error {
		res := tx.Model(&models.User{}).Where("id = ? AND balance >= ?", userID, amount).
			UpdateColumn("balance", gorm.Expr("balance - ?", amount))
		if res.Error != nil {
			return res.Error
		}
		if res.RowsAffected == 0 {
			return fmt.Errorf("余额不足，当前余额 ￥%.2f", GetUserBalance(userID))
		}
		var u models.User
		if err := tx.Select("id", "balance").First(&u, userID).Error; err != nil {
			return err
		}
		after = round2(u.Balance)
		remark := "订单支付"
		if o.Type == "renew" {
			remark = "实例续费"
		}
		return tx.Create(&models.BalanceLog{UserID: userID, Amount: -amount, Balance: after, Type: "pay", OrderNo: o.OrderNo, Remark: remark}).Error
	})
	if err != nil {
		return nil, err
	}
	paid, err := MarkOrderPaid(o.OrderNo, "balance", "BAL"+time.Now().Format("20060102150405"))
	if err != nil || paid == nil || paid.Status != "paid" {
		// 标记失败：退回余额
		db.DB.Model(&models.User{}).Where("id = ?", userID).UpdateColumn("balance", gorm.Expr("balance + ?", amount))
		db.DB.Create(&models.BalanceLog{UserID: userID, Amount: amount, Balance: GetUserBalance(userID), Type: "refund", OrderNo: o.OrderNo, Remark: "支付失败退回"})
		if err == nil {
			err = fmt.Errorf("订单状态异常")
		}
		return nil, err
	}
	return paid, nil
}

// ListBalanceLogs 用户余额流水（最近 limit 条）
func ListBalanceLogs(userID uint, limit int) []models.BalanceLog {
	if limit <= 0 || limit > 500 {
		limit = 50
	}
	var logs []models.BalanceLog
	db.DB.Where("user_id = ?", userID).Order("id DESC").Limit(limit).Find(&logs)
	return logs
}

// RenewOption 续费周期选项
type RenewOption struct {
	Period    string    `json:"period"`
	Text      string    `json:"text"`
	Price     float64   `json:"price"`
	NewExpire time.Time `json:"new_expire"`
}

var renewPeriods = []struct{ Key, Text string }{
	{"monthly", "月付"}, {"quarterly", "季付"}, {"half_year", "半年付"}, {"yearly", "年付"},
}

// GetRenewOptions 实例可选续费周期与价格、续费后到期时间
func GetRenewOptions(up *models.UserProduct) ([]RenewOption, error) {
	p, err := GetProduct(up.ProductID)
	if err != nil {
		return nil, fmt.Errorf("商品不存在，无法续费")
	}
	if p.Status == "disabled" {
		return nil, fmt.Errorf("该商品已停止续费")
	}
	base := time.Now()
	if up.ExpireAt != nil && up.ExpireAt.After(base) {
		base = *up.ExpireAt
	}
	out := []RenewOption{}
	for _, per := range renewPeriods {
		price := PeriodPrice(p, per.Key)
		if price < 0 || (price == 0 && !IsFreePeriod(p, per.Key)) {
			continue
		}
		out = append(out, RenewOption{Period: per.Key, Text: per.Text, Price: round2(price), NewExpire: base.Add(periodFactor[per.Key])})
	}
	return out, nil
}

// SetAutoRenew 设置实例自动续费（到期前 24 小时从余额扣款续费）
func SetAutoRenew(userID, id uint, enabled bool, period string) error {
	up, err := GetUserProduct(userID, id)
	if err != nil {
		return err
	}
	if period == "" {
		period = "monthly"
	}
	if _, ok := periodFactor[period]; !ok {
		return fmt.Errorf("无效的续费周期")
	}
	return db.DB.Model(up).Updates(map[string]interface{}{"auto_renew": enabled, "renew_period": period}).Error
}

// AutoRenewDueProducts 自动续费：到期前 24 小时内、已开启自动续费的实例，用余额创建并支付续费订单
func AutoRenewDueProducts() {
	defer func() {
		if r := recover(); r != nil {
			logger.Error("自动续费异常: %v", r)
		}
	}()
	soon := time.Now().Add(24 * time.Hour)
	var ups []models.UserProduct
	db.DB.Where("auto_renew = ? AND status IN ? AND expire_at IS NOT NULL AND expire_at < ?", true, []string{"active", "suspended"}, soon).Find(&ups)
	for i := range ups {
		up := &ups[i]
		// 已有待支付/处理中的续费订单则跳过，避免重复
		var pending int64
		db.DB.Model(&models.Order{}).Where("user_product_id = ? AND type = ? AND status = ? AND created_at > ?", up.ID, "renew", "pending", time.Now().Add(-24*time.Hour)).Count(&pending)
		if pending > 0 {
			continue
		}
		period := up.RenewPeriod
		if _, ok := periodFactor[period]; !ok {
			period = "monthly"
		}
		p, err := GetProduct(up.ProductID)
		if err != nil {
			continue
		}
		price := round2(PeriodPrice(p, period))
		if GetUserBalance(up.UserID) < price {
			// 余额不足：每个实例 24 小时内只提醒一次（通过关闭自动续费避免反复提醒）
			db.DB.Model(up).Update("auto_renew", false)
			NotifyUser(up.UserID, "product", "自动续费失败",
				fmt.Sprintf("实例 %s 自动续费需 ￥%.2f，余额不足，已关闭自动续费，请充值后手动续费。", up.Name, price), up.ID)
			continue
		}
		o, err := CreateOrder(up.UserID, up.ProductID, period, "renew", up.ID)
		if err != nil {
			logger.Warn("自动续费下单失败 instance=%d: %v", up.ID, err)
			continue
		}
		if o.Status == "paid" {
			continue
		}
		if _, err := PayOrderByBalance(up.UserID, o); err != nil {
			logger.Warn("自动续费扣款失败 order=%s: %v", o.OrderNo, err)
			continue
		}
		logger.Info("自动续费成功 instance=%d order=%s", up.ID, o.OrderNo)
	}
}

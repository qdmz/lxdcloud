package user

import (
	"strconv"

	"github.com/gin-gonic/gin"

	"lxdapi/internal/service"
	"lxdapi/pkg/response"
)

// ListInstances 我的实例
// @Summary 我的实例
// @Tags User API - 实例
// @Router /api/user/instances [get]
func ListInstances(c *gin.Context) {
	uid := currentUserID(c)
	if uid == 0 {
		return
	}
	ups, err := service.ListUserProducts(uid)
	if err != nil {
		response.Error(c, 500, "获取实例失败")
		return
	}
	response.Success(c, gin.H{"instances": ups})
}

// GetInstance 实例详情
// @Summary 实例详情
// @Tags User API - 实例
// @Router /api/user/instances/:id [get]
func GetInstance(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		response.Error(c, 400, "参数错误")
		return
	}
	uid := currentUserID(c)
	if uid == 0 {
		return
	}
	up, err := service.GetUserProduct(uid, uint(id))
	if err != nil {
		response.Error(c, 404, err.Error())
		return
	}
	response.Success(c, gin.H{"instance": up})
}

// RenewInstance 续费下单
// @Summary 续费下单
// @Tags User API - 实例
// @Router /api/user/instances/:id/renew [post]
func RenewInstance(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		response.Error(c, 400, "参数错误")
		return
	}
	var req struct {
		Period string `json:"period" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, 400, "参数错误")
		return
	}
	uid := currentUserID(c)
	if uid == 0 {
		return
	}
	up, err := service.GetUserProduct(uid, uint(id))
	if err != nil {
		response.Error(c, 404, err.Error())
		return
	}
	order, err := service.CreateOrder(uid, up.ProductID, req.Period, "renew", up.ID)
	if err != nil {
		response.Error(c, 400, err.Error())
		return
	}
	response.Success(c, gin.H{"order": order})
}

// DeleteInstance 删除实例
// @Summary 删除实例
// @Tags User API - 实例
// @Router /api/user/instances/:id [delete]
func DeleteInstance(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		response.Error(c, 400, "参数错误")
		return
	}
	uid := currentUserID(c)
	if uid == 0 {
		return
	}
	if err := service.DeleteUserProduct(uid, uint(id)); err != nil {
		response.Error(c, 400, err.Error())
		return
	}
	response.Success(c, gin.H{"deleted": true})
}

// RenewOptions 续费选项：可选周期与价格、续费后到期时间、余额与支付方式
// @Summary 续费选项
// @Tags User API - 实例
// @Router /api/user/instances/:id/renew-options [get]
func RenewOptions(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		response.Error(c, 400, "参数错误")
		return
	}
	uid := currentUserID(c)
	if uid == 0 {
		return
	}
	up, err := service.GetUserProduct(uid, uint(id))
	if err != nil {
		response.Error(c, 404, err.Error())
		return
	}
	opts, err := service.GetRenewOptions(up)
	if err != nil {
		response.Error(c, 400, err.Error())
		return
	}
	cfg := service.GetPayConfig()
	epay := cfg.Enabled && cfg.GatewayURL != "" && cfg.PID != "" && cfg.Key != ""
	response.Success(c, gin.H{
		"instance":     up,
		"options":      opts,
		"balance":      service.GetUserBalance(uid),
		"epay_enabled": epay,
	})
}

// SetAutoRenew 开关自动续费
// @Summary 自动续费设置
// @Tags User API - 实例
// @Router /api/user/instances/:id/auto-renew [post]
func SetAutoRenew(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		response.Error(c, 400, "参数错误")
		return
	}
	var req struct {
		Enabled bool   `json:"enabled"`
		Period  string `json:"period"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, 400, "参数错误")
		return
	}
	uid := currentUserID(c)
	if uid == 0 {
		return
	}
	if err := service.SetAutoRenew(uid, uint(id), req.Enabled, req.Period); err != nil {
		response.Error(c, 400, err.Error())
		return
	}
	response.Success(c, gin.H{"auto_renew": req.Enabled, "renew_period": req.Period})
}

// BalanceInfo 余额与流水
// @Summary 余额与流水
// @Tags User API - 账户
// @Router /api/user/balance [get]
func BalanceInfo(c *gin.Context) {
	uid := currentUserID(c)
	if uid == 0 {
		return
	}
	cfg := service.GetPayConfig()
	epay := cfg.Enabled && cfg.GatewayURL != "" && cfg.PID != "" && cfg.Key != ""
	response.Success(c, gin.H{"balance": service.GetUserBalance(uid), "logs": service.ListBalanceLogs(uid, 200),
		"epay_enabled": epay, "recharge_min": service.RechargeMin, "recharge_max": service.RechargeMax})
}

// RechargeBalance 在线充值：创建充值订单并返回易支付跳转地址
// @Summary 余额充值
// @Tags User API - 账户
// @Router /api/user/balance/recharge [post]
func RechargeBalance(c *gin.Context) {
	var req struct {
		Amount float64 `json:"amount"`
		Type   string  `json:"type"` // alipay / wxpay / qqpay
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, 400, "参数错误")
		return
	}
	uid := currentUserID(c)
	if uid == 0 {
		return
	}
	if req.Type == "balance" {
		response.Error(c, 400, "请选择在线支付方式")
		return
	}
	o, err := service.CreateRechargeOrder(uid, req.Amount)
	if err != nil {
		response.Error(c, 400, err.Error())
		return
	}
	payURL, _, err := service.BuildPayParams(o.OrderNo, "账户充值 "+o.OrderNo, twoDecimal(o.Amount), req.Type)
	if err != nil {
		response.Error(c, 400, err.Error())
		return
	}
	response.Success(c, gin.H{"order": o, "pay_url": payURL})
}

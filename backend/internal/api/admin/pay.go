package admin

import (
	"github.com/gin-gonic/gin"

	"lxdapi/internal/service"
	"lxdapi/pkg/response"
)

// GetPay 读取易支付配置
// @Summary 易支付配置
// @Tags Admin API - 支付
// @Router /api/admin/pay [get]
func GetPay(c *gin.Context) {
	response.Success(c, gin.H{"config": service.GetPayConfig()})
}

// SavePay 保存易支付配置
// @Summary 保存易支付配置
// @Tags Admin API - 支付
// @Router /api/admin/pay [post]
func SavePay(c *gin.Context) {
	var req struct {
		GatewayURL string `json:"gateway_url"`
		PID        string `json:"pid"`
		Key        string `json:"key"`
		NotifyURL  string `json:"notify_url"`
		ReturnURL  string `json:"return_url"`
		Enabled    bool   `json:"enabled"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, 400, "参数错误")
		return
	}
	if err := service.SavePayConfig(&service.PayInput{
		GatewayURL: req.GatewayURL, PID: req.PID, Key: req.Key,
		NotifyURL: req.NotifyURL, ReturnURL: req.ReturnURL, Enabled: req.Enabled,
	}); err != nil {
		response.Error(c, 400, err.Error())
		return
	}
	response.Success(c, gin.H{"saved": true})
}

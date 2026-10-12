package admin

import (
	"strconv"

	"github.com/gin-gonic/gin"
	"lxdapi/internal/service"
	"lxdapi/pkg/response"
)

// AdjustUserBalance 管理员调整用户余额（正数充值，负数扣减）
// @Summary 调整用户余额
// @Tags Admin API - 用户
// @Router /api/admin/users/:id/balance [post]
func AdjustUserBalance(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		response.Error(c, 400, "参数错误")
		return
	}
	var req struct {
		Amount float64 `json:"amount"`
		Remark string  `json:"remark"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, 400, "参数错误")
		return
	}
	remark := req.Remark
	if remark == "" {
		remark = "管理员调整"
	}
	after, err := service.AdjustBalance(uint(id), req.Amount, "", remark)
	if err != nil {
		response.Error(c, 400, err.Error())
		return
	}
	response.Success(c, gin.H{"balance": after, "logs": service.ListBalanceLogs(uint(id), 20)})
}

// UserBalanceLogs 管理员查看用户余额与流水
// @Summary 用户余额流水
// @Tags Admin API - 用户
// @Router /api/admin/users/:id/balance [get]
func UserBalanceLogs(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		response.Error(c, 400, "参数错误")
		return
	}
	response.Success(c, gin.H{"balance": service.GetUserBalance(uint(id)), "logs": service.ListAllBalanceLogs(uint(id), 200)})
}

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

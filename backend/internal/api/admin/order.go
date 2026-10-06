package admin

import (
	"github.com/gin-gonic/gin"

	"lxdapi/internal/db"
	"lxdapi/internal/service"
	"lxdapi/models"
	"lxdapi/pkg/response"
)

// ListOrders 订单列表
// @Summary 订单列表
// @Tags Admin API - 订单
// @Router /api/admin/orders [get]
func ListOrders(c *gin.Context) {
	status := c.DefaultQuery("status", "")
	var orders []models.Order
	q := db.DB.Order("id DESC").Limit(500)
	if status != "" {
		q = q.Where("status = ?", status)
	}
	if err := q.Find(&orders).Error; err != nil {
		response.Error(c, 500, "获取订单失败")
		return
	}
	response.Success(c, gin.H{"orders": orders})
}

// ListInstances 实例列表
// @Summary 实例列表
// @Tags Admin API - 订单
// @Router /api/admin/instances [get]
func ListInstances(c *gin.Context) {
	status := c.DefaultQuery("status", "")
	ups, err := service.ListAllUserProducts(status)
	if err != nil {
		response.Error(c, 500, "获取实例失败")
		return
	}
	response.Success(c, gin.H{"instances": ups})
}

// DeleteInstance 管理员删除实例
// @Summary 删除实例
// @Tags Admin API - 订单
// @Router /api/admin/instances/:id [delete]
func DeleteInstance(c *gin.Context) {
	var id uint
	if _, err := parseID(c.Param("id"), &id); err != nil {
		response.Error(c, 400, "参数错误")
		return
	}
	if err := service.AdminDeleteUserProduct(id); err != nil {
		response.Error(c, 400, err.Error())
		return
	}
	response.Success(c, gin.H{"deleted": true})
}

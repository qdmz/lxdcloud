package user

import (
	"strconv"

	"github.com/gin-gonic/gin"

	"lxdapi/internal/service"
	"lxdapi/pkg/response"
)

// CreateOrder 创建订单（新购/续费）
// @Summary 创建订单
// @Description 新购或续费下单
// @Tags User API - 订单
// @Accept json
// @Produce json
// @Param request body object{product_id=uint,period=string,type=string,user_product_id=uint} true "下单信息"
// @Success 200 {object} response.Response "订单信息"
// @Router /api/user/orders [post]
func CreateOrder(c *gin.Context) {
	var req struct {
		ProductID     uint   `json:"product_id" binding:"required"`
		Period        string `json:"period" binding:"required"`
		Type          string `json:"type"`
		UserProductID uint   `json:"user_product_id"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, 400, "参数错误")
		return
	}
	uid := currentUserID(c)
	if uid == 0 {
		return
	}
	if req.Type == "renew" && req.UserProductID == 0 {
		response.Error(c, 400, "续费订单缺少实例")
		return
	}
	order, err := service.CreateOrder(uid, req.ProductID, req.Period, req.Type, req.UserProductID)
	if err != nil {
		response.Error(c, 400, err.Error())
		return
	}
	response.Success(c, gin.H{"order": order})
}

// ListOrders 我的订单
// @Summary 我的订单
// @Tags User API - 订单
// @Router /api/user/orders [get]
func ListOrders(c *gin.Context) {
	uid := currentUserID(c)
	if uid == 0 {
		return
	}
	orders, err := service.ListUserOrders(uid)
	if err != nil {
		response.Error(c, 500, "获取订单失败")
		return
	}
	response.Success(c, gin.H{"orders": orders})
}

// PayOrder 发起支付（返回易支付跳转地址）
// @Summary 发起支付
// @Description 返回易支付下单跳转 URL 与参数
// @Tags User API - 订单
// @Router /api/user/orders/:id/pay [post]
func PayOrder(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		response.Error(c, 400, "参数错误")
		return
	}
	uid := currentUserID(c)
	if uid == 0 {
		return
	}
	var req struct {
		Type string `json:"type"` // alipay / wxpay / qqpay
	}
	_ = c.ShouldBindJSON(&req)

	var orders = mustGetUserOrder(uid, uint(id))
	if orders == nil {
		response.Error(c, 404, "订单不存在")
		return
	}
	if orders.Status != "pending" {
		response.Error(c, 400, "订单已支付或已关闭")
		return
	}
	payURL, params, err := service.BuildPayParams(orders.OrderNo, orders.OrderNo, twoDecimal(orders.Amount), req.Type)
	if err != nil {
		response.Error(c, 400, err.Error())
		return
	}
	response.Success(c, gin.H{"pay_url": payURL, "params": params})
}

// CancelOrder 取消订单
// @Summary 取消订单
// @Tags User API - 订单
// @Router /api/user/orders/:id/cancel [post]
func CancelOrder(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		response.Error(c, 400, "参数错误")
		return
	}
	uid := currentUserID(c)
	if uid == 0 {
		return
	}
	orders := mustGetUserOrder(uid, uint(id))
	if orders == nil {
		response.Error(c, 404, "订单不存在")
		return
	}
	if orders.Status != "pending" {
		response.Error(c, 400, "当前状态不可取消")
		return
	}
	if err := service.CancelUserOrder(uid, orders.ID); err != nil {
		response.Error(c, 400, err.Error())
		return
	}
	response.Success(c, gin.H{"cancelled": true})
}

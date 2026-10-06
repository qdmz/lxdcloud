package admin

import (
	"github.com/gin-gonic/gin"

	"lxdapi/internal/service"
	"lxdapi/pkg/response"
)

// ListTickets 工单列表
// @Summary 工单列表
// @Tags Admin API - 工单
// @Router /api/admin/tickets [get]
func ListTickets(c *gin.Context) {
	status := c.DefaultQuery("status", "")
	tickets, err := service.AdminListTickets(status)
	if err != nil {
		response.Error(c, 500, "获取工单失败")
		return
	}
	response.Success(c, gin.H{"tickets": tickets})
}

// GetTicket 工单详情
// @Summary 工单详情
// @Tags Admin API - 工单
// @Router /api/admin/tickets/:id [get]
func GetTicket(c *gin.Context) {
	var id uint
	if _, err := parseID(c.Param("id"), &id); err != nil {
		response.Error(c, 400, "参数错误")
		return
	}
	t, err := service.AdminGetTicket(id)
	if err != nil {
		response.Error(c, 404, err.Error())
		return
	}
	replies, err := service.ListTicketReplies(t.ID)
	if err != nil {
		response.Error(c, 500, "获取回复失败")
		return
	}
	response.Success(c, gin.H{"ticket": t, "replies": replies})
}

// ReplyTicket 管理员回复工单
// @Summary 回复工单
// @Tags Admin API - 工单
// @Router /api/admin/tickets/:id/reply [post]
func ReplyTicket(c *gin.Context) {
	var id uint
	if _, err := parseID(c.Param("id"), &id); err != nil {
		response.Error(c, 400, "参数错误")
		return
	}
	var req struct {
		Content string `json:"content" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, 400, "参数错误")
		return
	}
	if _, err := service.AdminGetTicket(id); err != nil {
		response.Error(c, 404, err.Error())
		return
	}
	if err := service.ReplyTicket(id, 0, true, req.Content); err != nil {
		response.Error(c, 400, err.Error())
		return
	}
	response.Success(c, gin.H{"replied": true})
}

// CloseTicket 管理员关闭工单
// @Summary 关闭工单
// @Tags Admin API - 工单
// @Router /api/admin/tickets/:id/close [post]
func CloseTicket(c *gin.Context) {
	var id uint
	if _, err := parseID(c.Param("id"), &id); err != nil {
		response.Error(c, 400, "参数错误")
		return
	}
	if _, err := service.AdminGetTicket(id); err != nil {
		response.Error(c, 404, err.Error())
		return
	}
	if err := service.AdminCloseTicket(id); err != nil {
		response.Error(c, 400, err.Error())
		return
	}
	response.Success(c, gin.H{"closed": true})
}

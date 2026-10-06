package user

import (
	"strconv"

	"github.com/gin-gonic/gin"

	"lxdapi/internal/service"
	"lxdapi/pkg/response"
)

// CreateTicket 提交工单
// @Summary 提交工单
// @Tags User API - 工单
// @Router /api/user/tickets [post]
func CreateTicket(c *gin.Context) {
	var req struct {
		Subject  string `json:"subject" binding:"required"`
		Category string `json:"category"`
		Priority string `json:"priority"`
		Content  string `json:"content" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, 400, "参数错误")
		return
	}
	uid := currentUserID(c)
	if uid == 0 {
		return
	}
	t, err := service.CreateTicket(uid, req.Subject, req.Category, req.Priority, req.Content)
	if err != nil {
		response.Error(c, 400, err.Error())
		return
	}
	response.Success(c, gin.H{"ticket": t})
}

// ListTickets 我的工单
// @Summary 我的工单
// @Tags User API - 工单
// @Router /api/user/tickets [get]
func ListTickets(c *gin.Context) {
	uid := currentUserID(c)
	if uid == 0 {
		return
	}
	tickets, err := service.ListUserTickets(uid)
	if err != nil {
		response.Error(c, 500, "获取工单失败")
		return
	}
	response.Success(c, gin.H{"tickets": tickets})
}

// GetTicket 工单详情（含回复）
// @Summary 工单详情
// @Tags User API - 工单
// @Router /api/user/tickets/:id [get]
func GetTicket(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		response.Error(c, 400, "参数错误")
		return
	}
	uid := currentUserID(c)
	if uid == 0 {
		return
	}
	t, err := service.GetUserTicket(uid, uint(id))
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

// ReplyTicket 回复工单
// @Summary 回复工单
// @Tags User API - 工单
// @Router /api/user/tickets/:id/reply [post]
func ReplyTicket(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
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
	uid := currentUserID(c)
	if uid == 0 {
		return
	}
	if _, err := service.GetUserTicket(uid, uint(id)); err != nil {
		response.Error(c, 404, err.Error())
		return
	}
	if err := service.ReplyTicket(uint(id), uid, false, req.Content); err != nil {
		response.Error(c, 400, err.Error())
		return
	}
	response.Success(c, gin.H{"replied": true})
}

// CloseTicket 关闭工单
// @Summary 关闭工单
// @Tags User API - 工单
// @Router /api/user/tickets/:id/close [post]
func CloseTicket(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		response.Error(c, 400, "参数错误")
		return
	}
	uid := currentUserID(c)
	if uid == 0 {
		return
	}
	if _, err := service.GetUserTicket(uid, uint(id)); err != nil {
		response.Error(c, 404, err.Error())
		return
	}
	if err := service.CloseTicket(uint(id), uid); err != nil {
		response.Error(c, 400, err.Error())
		return
	}
	response.Success(c, gin.H{"closed": true})
}

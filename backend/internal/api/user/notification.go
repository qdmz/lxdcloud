package user

import (
	"strconv"

	"github.com/gin-gonic/gin"

	"lxdapi/internal/db"
	"lxdapi/internal/service"
	"lxdapi/models"
	"lxdapi/pkg/response"
)

// ListNotifications 站内消息
// @Summary 站内消息
// @Tags User API - 消息
// @Router /api/user/notifications [get]
func ListNotifications(c *gin.Context) {
	uid := currentUserID(c)
	if uid == 0 {
		return
	}
	var list []models.Notification
	if err := db.DB.Where("user_id = ?", uid).Order("id DESC").Limit(100).Find(&list).Error; err != nil {
		response.Error(c, 500, "获取消息失败")
		return
	}
	unread := service.GetUnreadNotificationCount(uid)
	response.Success(c, gin.H{"notifications": list, "unread": unread})
}

// MarkNotificationRead 标记已读
// @Summary 标记已读
// @Tags User API - 消息
// @Router /api/user/notifications/:id/read [post]
func MarkNotificationRead(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		response.Error(c, 400, "参数错误")
		return
	}
	uid := currentUserID(c)
	if uid == 0 {
		return
	}
	if err := service.MarkNotificationRead(uid, uint(id)); err != nil {
		response.Error(c, 400, err.Error())
		return
	}
	response.Success(c, gin.H{"read": true})
}

// MarkAllNotificationsRead 全部已读
// @Summary 全部已读
// @Tags User API - 消息
// @Router /api/user/notifications/read-all [post]
func MarkAllNotificationsRead(c *gin.Context) {
	uid := currentUserID(c)
	if uid == 0 {
		return
	}
	if err := service.MarkAllNotificationsRead(uid); err != nil {
		response.Error(c, 400, err.Error())
		return
	}
	response.Success(c, gin.H{"read": true})
}

// DeleteNotification 删除消息
// @Summary 删除消息
// @Tags User API - 消息
// @Router /api/user/notifications/:id [delete]
func DeleteNotification(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		response.Error(c, 400, "参数错误")
		return
	}
	uid := currentUserID(c)
	if uid == 0 {
		return
	}
	if err := service.DeleteNotification(uid, uint(id)); err != nil {
		response.Error(c, 400, err.Error())
		return
	}
	response.Success(c, gin.H{"deleted": true})
}

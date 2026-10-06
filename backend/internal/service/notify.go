package service

import (
	"lxdapi/internal/db"
	"lxdapi/models"
	"lxdapi/pkg/logger"
)

// NotifyUser 发送站内消息
func NotifyUser(userID uint, typ, title, content string, relatedID uint) {
	n := &models.Notification{
		UserID:    userID,
		Type:      typ,
		Title:     title,
		Content:   content,
		RelatedID: relatedID,
	}
	if err := db.DB.Create(n).Error; err != nil {
		logger.Error("发送站内消息失败: %v", err)
	}
}

// GetUnreadNotificationCount 未读消息数
func GetUnreadNotificationCount(userID uint) int64 {
	var cnt int64
	db.DB.Model(&models.Notification{}).Where("user_id = ? AND is_read = ?", userID, false).Count(&cnt)
	return cnt
}

// MarkNotificationRead 标记消息已读
func MarkNotificationRead(userID, id uint) error {
	return db.DB.Model(&models.Notification{}).
		Where("user_id = ? AND id = ?", userID, id).
		Update("is_read", true).Error
}

// MarkAllNotificationsRead 全部已读
func MarkAllNotificationsRead(userID uint) error {
	return db.DB.Model(&models.Notification{}).
		Where("user_id = ? AND is_read = ?", userID, false).
		Update("is_read", true).Error
}

// DeleteNotification 删除消息
func DeleteNotification(userID, id uint) error {
	return db.DB.Where("user_id = ? AND id = ?", userID, id).Delete(&models.Notification{}).Error
}

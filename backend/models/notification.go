package models

import (
	"time"

	"gorm.io/gorm"
)

// Notification 站内消息
type Notification struct {
	gorm.Model
	UserID    uint       `gorm:"index" json:"user_id"`
	Type      string     `gorm:"size:30" json:"type"` // order / product / ticket / system
	Title     string     `gorm:"size:200" json:"title"`
	Content   string     `gorm:"type:text" json:"content"`
	RelatedID uint       `json:"related_id"`
	IsRead    bool       `gorm:"default:false" json:"is_read"`
	ReadAt    *time.Time `json:"read_at"`
}

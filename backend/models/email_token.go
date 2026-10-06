package models

import (
	"time"

	"gorm.io/gorm"
)

// EmailToken 邮件令牌（激活 / 找回密码）
type EmailToken struct {
	gorm.Model
	UserID   uint       `gorm:"index" json:"user_id"`
	Token    string     `gorm:"uniqueIndex;size:128" json:"token"`
	Type     string     `gorm:"size:20" json:"type"` // activate / reset
	ExpireAt time.Time  `json:"expire_at"`
	UsedAt   *time.Time `json:"used_at"`
}

package models

import "gorm.io/gorm"

// SMTPConfig SMTP 邮件配置（单例，固定 ID=1）
type SMTPConfig struct {
	gorm.Model
	Host      string `gorm:"size:255" json:"host"`
	Port      int    `json:"port"`
	Username  string `gorm:"size:255" json:"username"`
	Password  string `gorm:"size:255" json:"password"`
	FromEmail string `gorm:"size:255" json:"from_email"`
	FromName  string `gorm:"size:100" json:"from_name"`
	EnableSSL bool   `json:"enable_ssl"`
	Enabled   bool   `gorm:"default:false" json:"enabled"`
}

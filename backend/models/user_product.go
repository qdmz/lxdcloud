package models

import (
	"time"

	"gorm.io/gorm"
)

// UserProduct 用户实例（开通的容器/VM）
type UserProduct struct {
	gorm.Model
	UserID    uint   `gorm:"index" json:"user_id"`
	ProductID uint   `gorm:"index" json:"product_id"`
	NodeID    uint   `gorm:"index" json:"node_id"`
	Name      string `gorm:"size:100" json:"name"` // 容器/VM 名
	Type      string `gorm:"size:20;default:'container'" json:"type"`

	Status string `gorm:"size:20;default:'active'" json:"status"` // creating / active / suspended / expired / deleting
	// 实例配置快照（创建时按产品固化，防止产品改价后实例变）
	ConfigJSON string `gorm:"type:text" json:"config_json"`

	ExpireAt      *time.Time `gorm:"index" json:"expire_at"`
	AutoRenew     bool       `gorm:"default:false" json:"auto_renew"`
	ProvisionedAt *time.Time `json:"provisioned_at"`
}

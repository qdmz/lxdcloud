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
	RenewPeriod   string     `gorm:"size:20;default:'monthly'" json:"renew_period"` // 自动续费周期
	ProvisionedAt *time.Time `json:"provisioned_at"`

	// ProvisionTaskID 节点开通任务 ID（主控在开通等待期间重启后，据此向节点核对结果）
	ProvisionTaskID uint `gorm:"default:0" json:"provision_task_id,omitempty"`
	// PendingAction 待重试的节点操作（如续费恢复时节点离线未能启动：start），由定时任务重试
	PendingAction string `gorm:"size:20;default:''" json:"pending_action,omitempty"`
}

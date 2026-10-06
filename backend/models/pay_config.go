package models

import "gorm.io/gorm"

// PayConfig 易支付配置（单例，固定 ID=1）
type PayConfig struct {
	gorm.Model
	GatewayURL string `gorm:"size:255" json:"gateway_url"` // 易支付网关地址，如 https://pay.xxx.com
	PID        string `gorm:"size:64" json:"pid"`          // 商户 ID
	Key        string `gorm:"size:64" json:"key"`          // 商户密钥
	NotifyURL  string `gorm:"size:255" json:"notify_url"`  // 异步回调地址（可留空，按站点自动拼）
	ReturnURL  string `gorm:"size:255" json:"return_url"`  // 同步跳转地址（可留空，按站点自动拼）
	Enabled    bool   `gorm:"default:false" json:"enabled"`
}

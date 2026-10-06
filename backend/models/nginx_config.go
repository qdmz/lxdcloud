package models

import (
	"time"

	"gorm.io/gorm"
)

// NginxConfig Nginx全局配置
type NginxConfig struct {
	gorm.Model

	Enabled          bool       `json:"enabled" gorm:"default:false"`
	LastApplied      *time.Time `json:"last_applied"`
	RestrictedDomains string    `json:"restricted_domains" gorm:"type:text"`
	RestrictedIPs     string    `json:"restricted_ips" gorm:"type:text"`
}

// ReverseProxy 反向代理规则
type ReverseProxy struct {
	ID            uint      `gorm:"primaryKey" json:"id"`
	ContainerName string    `gorm:"index;size:255;not null" json:"container_name"`
	Protocol      string    `gorm:"size:10;not null" json:"protocol"` // http 或 https
	Domain        string    `gorm:"uniqueIndex;size:255;not null" json:"domain"`
	PublicPort    int       `gorm:"not null" json:"public_port"` // 80 或 443
	TargetIP      string    `gorm:"size:50;not null" json:"target_ip"`
	TargetPort    int       `gorm:"not null" json:"target_port"`
	EnableSSL     bool      `gorm:"default:false" json:"enable_ssl"`
	SSLCert       string    `gorm:"type:text" json:"ssl_cert"` // PEM格式证书内容
	SSLKey        string    `gorm:"type:text" json:"ssl_key"`  // PEM格式密钥内容
	CustomConfig  string    `gorm:"type:text" json:"custom_config"` // 自定义Nginx location配置
	Description   string    `gorm:"size:500" json:"description"`
	Status        string    `gorm:"size:20;default:active" json:"status"` // active, disabled
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

func (ReverseProxy) TableName() string {
	return "reverse_proxies"
}

// NginxStatus Nginx运行状态
type NginxStatus struct {
	Running          bool   `json:"running"`
	Version          string `json:"version"`
	WorkerProcesses  int    `json:"worker_processes"`
	ActiveProxies    int    `json:"active_proxies"`
	TotalProxies     int    `json:"total_proxies"`
	ConfigLastUpdate string `json:"config_last_update"`
	Uptime           string `json:"uptime"`
}

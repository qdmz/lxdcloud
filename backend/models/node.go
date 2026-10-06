package models

import (
	"time"

	"gorm.io/gorm"
)

// Node 服务器节点。子节点安装 lxdapi 后端，主控通过 system API（api_hash 鉴权）代理管理。
type Node struct {
	gorm.Model
	Name       string     `gorm:"size:100" json:"name"`
	APIBaseURL string     `gorm:"size:255" json:"api_base_url"` // 例 http://192.168.1.10:8123
	APIHash    string     `gorm:"size:255" json:"api_hash"`     // 节点 api_hash
	Status     string     `gorm:"size:20;default:'unknown'" json:"status"` // online/offline/unknown
	CPU        int        `json:"cpu"`
	Memory     int64      `json:"memory"` // MB
	Disk       int64      `json:"disk"`   // GB
	Region     string     `gorm:"size:100" json:"region"`
	Remark     string     `gorm:"size:255" json:"remark"`
	LastPingAt *time.Time `json:"last_ping_at"`
}

package models

import (
	"time"

	"gorm.io/gorm"
)

type User struct {
	gorm.Model
	Username          string `gorm:"uniqueIndex;size:255"`
	Email             string `gorm:"uniqueIndex;size:255"`
	PasswordHash      string `gorm:"size:255"` // bcrypt 哈希；老用户为空时用 APIKey 明文兼容
	Nickname          string `gorm:"size:100"`
	APIKey            string `gorm:"uniqueIndex;size:255"`
	Status            string `gorm:"size:50;default:'active'"` // pending / active / disabled
	EmailVerified     bool   `gorm:"default:false"`
	ActivatedAt       *time.Time
	LastLoginAt       *time.Time
	CPUQuota          int
	MemoryQuota       int
	DiskQuota         int
	MaxCPUPerContainer int
	TrafficLimit      int
	TrafficUsed       float64 `gorm:"default:0"`
	TrafficLocked     bool    `gorm:"default:false"`
	IPv4PoolLimit     int
	IPv4MappingLimit  int
	IPv6PoolLimit     int
	IPv6MappingLimit  int
	ReverseProxyLimit int
	Ingress           int
	Egress            int
	CPUAllowance      int
	IORead            int
	IOWrite           int
	ProcessesLimit    int
	AllowNesting      bool
	MemorySwap        bool
}


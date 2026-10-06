package models

import (
	"time"

	"gorm.io/gorm"
)

type FirewallConfig struct {
	gorm.Model

	Rules string `json:"rules" gorm:"type:text"`

	Enabled     bool       `json:"enabled" gorm:"default:false"`
	LastApplied *time.Time `json:"last_applied"`
}

type FirewallStats struct {
	Running          bool   `json:"running"`
	Version          string `json:"version"`
	Workers          int    `json:"workers"`
	PacketsProcessed uint64 `json:"packets_processed"`
	Uptime           string `json:"uptime"`
	ActiveRules      int    `json:"active_rules"`
	BlockedToday     int64  `json:"blocked_today"`
}

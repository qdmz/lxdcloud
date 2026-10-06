package models

import "time"

type CacheAutoRefreshSettings struct {
	ID            uint      `gorm:"primaryKey" json:"id"`
	Enabled       bool      `gorm:"default:false" json:"enabled"`
	Interval      int       `gorm:"default:30" json:"interval"`
	BatchSize     int       `gorm:"default:10" json:"batch_size"`
	BatchInterval int       `gorm:"default:5" json:"batch_interval"`
	LastRunTime   time.Time `json:"last_run_time"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

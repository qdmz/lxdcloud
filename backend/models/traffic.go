package models

import "time"

type Traffic struct {
	ContainerName string    `gorm:"primaryKey;size:255"`
	RxBytes       int64     `gorm:"default:0"`
	TxBytes       int64     `gorm:"default:0"`
	TotalGB       float64   `gorm:"default:0"`
	LimitGB       int       `gorm:"default:0"`
	ResetDay      int       `gorm:"default:1"`
	Locked        bool      `gorm:"default:false"`
	LastUpdate    time.Time
	LastReset     time.Time
}


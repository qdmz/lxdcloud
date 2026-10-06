package models

import "time"

type PortRangeConfig struct {
	ID               uint      `gorm:"primaryKey" json:"id"`
	V4PortStart      int       `gorm:"default:10000" json:"v4_port_start"`
	V4PortEnd        int       `gorm:"default:65535" json:"v4_port_end"`
	V6PortStart      int       `gorm:"default:10000" json:"v6_port_start"`
	V6PortEnd        int       `gorm:"default:65535" json:"v6_port_end"`
	V4AutoAllocate22 bool      `gorm:"default:false" json:"v4_auto_allocate_22"`
	V6AutoAllocate22 bool      `gorm:"default:false" json:"v6_auto_allocate_22"`
	CreatedAt        time.Time `json:"created_at"`
	UpdatedAt        time.Time `json:"updated_at"`
}

func (PortRangeConfig) TableName() string {
	return "port_range_config"
}

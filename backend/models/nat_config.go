package models

import "time"

type NATConfigV4 struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	Interface string    `gorm:"size:50;not null" json:"interface"`
	IP        string    `gorm:"size:50;not null" json:"ip"`
	DisplayIP string    `gorm:"size:200;not null;default:''" json:"display_ip"`
	Protocol  string    `gorm:"size:10;default:tcp;not null" json:"protocol"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type NATConfigV6 struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	Interface string    `gorm:"size:50;not null" json:"interface"`
	IP        string    `gorm:"size:50;not null" json:"ip"`
	DisplayIP string    `gorm:"size:200;not null;default:''" json:"display_ip"`
	Protocol  string    `gorm:"size:10;default:tcp;not null" json:"protocol"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

func (NATConfigV4) TableName() string {
	return "nat_config_v4"
}

func (NATConfigV6) TableName() string {
	return "nat_config_v6"
}

package models

import (
	"time"
)

type PortMappingV4 struct {
	ID               uint      `gorm:"primaryKey" json:"id"`
	ForwardIP        string    `gorm:"size:50;not null;default:''" json:"forward_ip"`
	PublicIP         string    `gorm:"index;size:50;not null" json:"public_ip"`
	PublicPort       int       `gorm:"index;not null" json:"public_port"`
	PublicPortEnd    int       `gorm:"not null;default:0" json:"public_port_end"`
	ContainerName    string    `gorm:"index;size:255;not null" json:"container_name"`
	ContainerIP      string    `gorm:"size:50" json:"container_ip"`
	ContainerPort    int       `gorm:"not null" json:"container_port"`
	ContainerPortEnd int       `gorm:"not null;default:0" json:"container_port_end"`
	Protocol         string    `gorm:"size:10;default:tcp;not null" json:"protocol"`
	Status           string    `gorm:"size:20;default:active" json:"status"`
	UserID           string    `gorm:"index;size:255" json:"user_id"`
	Interface        string    `gorm:"size:50" json:"interface"`
	Description      string    `gorm:"size:255" json:"description"`
	CreatedAt        time.Time `json:"created_at"`
	UpdatedAt        time.Time `json:"updated_at"`
}

type PortMappingV6 struct {
	ID               uint      `gorm:"primaryKey" json:"id"`
	ForwardIP        string    `gorm:"size:50;not null;default:''" json:"forward_ip"`
	PublicIP         string    `gorm:"index;size:50;not null" json:"public_ip"`
	PublicPort       int       `gorm:"index;not null" json:"public_port"`
	PublicPortEnd    int       `gorm:"not null;default:0" json:"public_port_end"`
	ContainerName    string    `gorm:"index;size:255;not null" json:"container_name"`
	ContainerIP      string    `gorm:"size:50" json:"container_ip"`
	ContainerPort    int       `gorm:"not null" json:"container_port"`
	ContainerPortEnd int       `gorm:"not null;default:0" json:"container_port_end"`
	Protocol         string    `gorm:"size:10;default:tcp;not null" json:"protocol"`
	Status           string    `gorm:"size:20;default:active" json:"status"`
	UserID           string    `gorm:"index;size:255" json:"user_id"`
	Interface        string    `gorm:"size:50" json:"interface"`
	Description      string    `gorm:"size:255" json:"description"`
	CreatedAt        time.Time `json:"created_at"`
	UpdatedAt        time.Time `json:"updated_at"`
}

func (PortMappingV4) TableName() string {
	return "port_mapping_v4"
}

func (PortMappingV6) TableName() string {
	return "port_mapping_v6"
}

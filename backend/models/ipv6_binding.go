package models

import "gorm.io/gorm"

type IPv6Binding struct {
	gorm.Model
	IPAddress     string `gorm:"uniqueIndex;size:50" json:"ip_address"`
	ContainerName string `gorm:"index;size:255" json:"container_name"`
	UserID        string `gorm:"index;size:255" json:"user_id"`
	Status        string `gorm:"size:50" json:"status"`
}

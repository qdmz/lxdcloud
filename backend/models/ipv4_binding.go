package models

import "gorm.io/gorm"

type IPv4Binding struct {
	gorm.Model
	IPAddress     string `gorm:"uniqueIndex;size:50"`
	ContainerName string `gorm:"index;size:255"`
	UserID        string `gorm:"index;size:255"`
	Status        string `gorm:"size:50"`
}

package models

import "gorm.io/gorm"

type IPv6Pool struct {
	gorm.Model
	IPAddress string `gorm:"uniqueIndex;size:50"`
	Interface string `gorm:"size:50"`
	Status    string `gorm:"size:50;default:'available'"`
	Note      string `gorm:"size:255"`
}

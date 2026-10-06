package models

import "gorm.io/gorm"

type IPv4Pool struct {
	gorm.Model
	IPAddress string `gorm:"uniqueIndex;size:50"`
	Interface string `gorm:"size:50"`
	Status    string `gorm:"size:50;default:'available'"`
	Note      string `gorm:"size:255"`
}

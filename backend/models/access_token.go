package models

import (
	"gorm.io/gorm"
	"time"
)

type AccessToken struct {
	gorm.Model
	Token     string    `gorm:"uniqueIndex;size:64"`
	Type      string    `gorm:"index;size:20"`
	Target    string    `gorm:"size:255"`
	ExpiresAt time.Time `gorm:"index"`
}

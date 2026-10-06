package models

import "gorm.io/gorm"

type ContainerCredential struct {
	gorm.Model
	ContainerName string `gorm:"index;size:255"`
	Hash          string `gorm:"uniqueIndex;size:255"`
	CreatedBy     string `gorm:"size:255"`
}


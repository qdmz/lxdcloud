package models

import "gorm.io/gorm"

type StoragePool struct {
	gorm.Model
	Name        string `gorm:"uniqueIndex;size:100"`
	Driver      string `gorm:"size:50"`
	Description string `gorm:"size:255"`
	Status      string `gorm:"size:50"`
	UsedBy      int
	TotalSpace  int64
	UsedSpace   int64
	Priority    int `gorm:"default:0"` // 0=禁用, 数字越小优先级越高
}

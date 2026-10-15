package models

import "gorm.io/gorm"

// Region 机房/地域：节点按机房分组，商品按机房展示与筛选（v1.1.17）
type Region struct {
	gorm.Model
	Name        string `gorm:"uniqueIndex;size:100" json:"name"`
	Code        string `gorm:"size:50" json:"code"`
	Description string `gorm:"size:255" json:"description"`
	SortOrder   int    `gorm:"default:0" json:"sort_order"`
	Status      string `gorm:"size:20;default:'active'" json:"status"` // active / hidden
}

// AppMigration 记录已执行的一次性数据迁移（幂等保护）
type AppMigration struct {
	MigrationKey string `gorm:"primaryKey;column:migration_key;size:100"`
	AppliedAt    int64
	Note         string `gorm:"size:500"`
}

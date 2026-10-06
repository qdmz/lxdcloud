package models

import "time"

type IPPoolSettings struct {
	ID                        int       `gorm:"primaryKey" json:"id"`
	RandomAssign              bool      `gorm:"default:false" json:"random_assign"`
	AutoAssign                bool      `gorm:"default:false" json:"auto_assign"`
	AllowUserReleaseIPv4      bool      `gorm:"default:false" json:"allow_user_release_ipv4"`
	AllowUserReleaseIPv6      bool      `gorm:"default:false" json:"allow_user_release_ipv6"`
	AllowContainerReleaseIPv4 bool      `gorm:"default:false" json:"allow_container_release_ipv4"`
	AllowContainerReleaseIPv6 bool      `gorm:"default:false" json:"allow_container_release_ipv6"`
	UpdatedAt                 time.Time `json:"updated_at"`
	CreatedAt                 time.Time `json:"created_at"`
}

package models

import (
	"time"

	"gorm.io/gorm"
)

// Ticket 工单
type Ticket struct {
	gorm.Model
	UserID       uint       `gorm:"index" json:"user_id"`
	Subject      string     `gorm:"size:200" json:"subject"`
	Category     string     `gorm:"size:50" json:"category"` // 故障/咨询/财务/其他
	Priority     string     `gorm:"size:20;default:'normal'" json:"priority"` // low / normal / high / urgent
	Status       string     `gorm:"size:20;default:'open'" json:"status"`     // open / replied / closed
	ClosedAt     *time.Time `json:"closed_at"`
	LastReplyAt  *time.Time `json:"last_reply_at"`
	StaffID      uint       `gorm:"default:0" json:"staff_id"`
}

// TicketReply 工单回复
type TicketReply struct {
	gorm.Model
	TicketID uint   `gorm:"index" json:"ticket_id"`
	UserID   uint   `gorm:"index" json:"user_id"` // 0 = 管理员
	IsStaff  bool   `gorm:"default:false" json:"is_staff"`
	Content  string `gorm:"type:text" json:"content"`
}

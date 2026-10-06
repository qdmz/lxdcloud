package models

import (
	"time"

	"gorm.io/gorm"
)

// Order 订单
type Order struct {
	gorm.Model
	OrderNo       string     `gorm:"uniqueIndex;size:64" json:"order_no"`
	UserID        uint       `gorm:"index" json:"user_id"`
	ProductID     uint       `gorm:"index" json:"product_id"`
	UserProductID uint       `gorm:"index" json:"user_product_id"` // 续费订单关联实例
	Type          string     `gorm:"size:20;default:'new'" json:"type"` // new / renew
	Period        string     `gorm:"size:20" json:"period"` // monthly / quarterly / halfyear / yearly
	Amount        float64    `json:"amount"`
	Status        string     `gorm:"size:20;default:'pending'" json:"status"` // pending / paid / cancelled / failed
	PayChannel    string     `gorm:"size:20" json:"pay_channel"`
	PayTradeNo    string     `gorm:"size:100" json:"pay_trade_no"`
	PaidAt        *time.Time `json:"paid_at"`
	Extra         string     `gorm:"type:text" json:"extra"`
}

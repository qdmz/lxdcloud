package models

import "gorm.io/gorm"

// BalanceLog 余额流水
type BalanceLog struct {
	gorm.Model
	UserID  uint    `gorm:"index" json:"user_id"`
	Amount  float64 `json:"amount"`                 // 变动金额：正数入账，负数扣款
	Balance float64 `json:"balance"`                // 变动后余额
	Type    string  `gorm:"size:20" json:"type"`    // recharge / pay / refund / adjust
	OrderNo string  `gorm:"size:64;index" json:"order_no"`
	Remark  string  `gorm:"size:255" json:"remark"`
}

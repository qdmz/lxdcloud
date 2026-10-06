package models

import "gorm.io/gorm"

// MailTemplate 邮件模板。Body 支持 Go template 语法，变量 {{.SiteName}} {{.Username}} {{.Link}} 等。
type MailTemplate struct {
	gorm.Model
	Code    string `gorm:"uniqueIndex;size:50" json:"code"` // activate / reset_password / order_created / instance_created / expiring / suspended / ticket_reply
	Name    string `gorm:"size:100" json:"name"`
	Subject string `gorm:"size:255" json:"subject"`
	Body    string `gorm:"type:text" json:"body"`
}

package service

import (
	"fmt"
	"time"

	"lxdapi/internal/db"
	"lxdapi/models"
)

// CreateTicket 用户提交工单
func CreateTicket(userID uint, subject, category, priority, content string) (*models.Ticket, error) {
	if subject == "" || content == "" {
		return nil, fmt.Errorf("主题与内容不能为空")
	}
	if category == "" {
		category = "其他"
	}
	if priority == "" {
		priority = "normal"
	}
	t := &models.Ticket{
		UserID:   userID,
		Subject:  subject,
		Category: category,
		Priority: priority,
		Status:   "open",
	}
	if err := db.DB.Create(t).Error; err != nil {
		return nil, err
	}
	reply := &models.TicketReply{
		TicketID: t.ID,
		UserID:   userID,
		Content:  content,
	}
	if err := db.DB.Create(reply).Error; err != nil {
		return nil, err
	}
	now := time.Now()
	t.LastReplyAt = &now
	db.DB.Save(t)
	return t, nil
}

// ListUserTickets 用户工单列表
func ListUserTickets(userID uint) ([]models.Ticket, error) {
	var tickets []models.Ticket
	if err := db.DB.Where("user_id = ?", userID).Order("id DESC").Find(&tickets).Error; err != nil {
		return nil, err
	}
	return tickets, nil
}

// GetUserTicket 用户工单详情（校验归属）
func GetUserTicket(userID, id uint) (*models.Ticket, error) {
	var t models.Ticket
	if err := db.DB.Where("id = ? AND user_id = ?", id, userID).First(&t).Error; err != nil {
		return nil, fmt.Errorf("工单不存在")
	}
	return &t, nil
}

// ListTicketReplies 工单回复列表
func ListTicketReplies(ticketID uint) ([]models.TicketReply, error) {
	var replies []models.TicketReply
	if err := db.DB.Where("ticket_id = ?", ticketID).Order("id ASC").Find(&replies).Error; err != nil {
		return nil, err
	}
	return replies, nil
}

// ReplyTicket 用户/管理员回复工单
func ReplyTicket(ticketID, userID uint, isStaff bool, content string) error {
	if content == "" {
		return fmt.Errorf("回复内容不能为空")
	}
	var t models.Ticket
	if err := db.DB.First(&t, ticketID).Error; err != nil {
		return fmt.Errorf("工单不存在")
	}
	if t.Status == "closed" {
		return fmt.Errorf("工单已关闭，无法回复")
	}
	r := &models.TicketReply{
		TicketID: ticketID,
		UserID:   userID,
		IsStaff:  isStaff,
		Content:  content,
	}
	if err := db.DB.Create(r).Error; err != nil {
		return err
	}
	now := time.Now()
	t.LastReplyAt = &now
	if isStaff {
		t.Status = "replied"
		t.StaffID = userID
		// 通知用户
		var user models.User
		if err := db.DB.First(&user, t.UserID).Error; err == nil {
			NotifyUser(t.UserID, "ticket", "工单有新回复", fmt.Sprintf("您的工单「%s」收到管理员回复，请前往查看。", t.Subject), t.ID)
			_ = SendMailWithTemplate(user.Email, MailTemplateTicketReply, MailData{
				"SiteName":      SiteName(),
				"Username":      user.Username,
				"TicketSubject": t.Subject,
			})
		}
	} else {
		t.Status = "open"
	}
	return db.DB.Save(&t).Error
}

// CloseTicket 关闭工单
func CloseTicket(ticketID uint, userID uint) error {
	var t models.Ticket
	if err := db.DB.First(&t, ticketID).Error; err != nil {
		return fmt.Errorf("工单不存在")
	}
	now := time.Now()
	t.Status = "closed"
	t.ClosedAt = &now
	return db.DB.Save(&t).Error
}

// AdminCloseTicket 管理员关闭工单
func AdminCloseTicket(id uint) error {
	var t models.Ticket
	if err := db.DB.First(&t, id).Error; err != nil {
		return fmt.Errorf("工单不存在")
	}
	now := time.Now()
	t.Status = "closed"
	t.ClosedAt = &now
	if err := db.DB.Save(&t).Error; err != nil {
		return err
	}
	// 通知用户
	var user models.User
	if err := db.DB.First(&user, t.UserID).Error; err == nil {
		NotifyUser(user.ID, "ticket", "工单已关闭", fmt.Sprintf("工单「%s」已被管理员关闭。", t.Subject), t.ID)
	}
	return nil
}

// AdminListTickets 管理端工单列表
func AdminListTickets(status string) ([]models.Ticket, error) {
	var tickets []models.Ticket
	q := db.DB
	if status != "" {
		q = q.Where("status = ?", status)
	}
	if err := q.Order("id DESC").Find(&tickets).Error; err != nil {
		return nil, err
	}
	return tickets, nil
}

// AdminGetTicket 管理端工单详情
func AdminGetTicket(id uint) (*models.Ticket, error) {
	var t models.Ticket
	if err := db.DB.First(&t, id).Error; err != nil {
		return nil, fmt.Errorf("工单不存在")
	}
	return &t, nil
}

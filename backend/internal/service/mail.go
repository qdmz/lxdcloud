package service

import (
	"bytes"
	"crypto/tls"
	"fmt"
	"net/smtp"
	"strings"
	"text/template"

	"lxdapi/internal/db"
	"lxdapi/models"
	"lxdapi/pkg/logger"

	"gorm.io/gorm"
)

const (
	MailTemplateActivate         = "activate"
	MailTemplateResetPwd         = "reset_password"
	MailTemplateOrderCreated     = "order_created"
	MailTemplateInstanceCreated  = "instance_created"
	MailTemplateExpiring         = "expiring"
	MailTemplateSuspended        = "suspended"
	MailTemplateTicketReply      = "ticket_reply"
)

// MailData 模板变量
type MailData map[string]interface{}

// GetSMTPConfig 获取 SMTP 配置（单例 ID=1，不存在则初始化占位）
func GetSMTPConfig() *models.SMTPConfig {
	var cfg models.SMTPConfig
	if err := db.DB.First(&cfg, 1).Error; err != nil {
		cfg = models.SMTPConfig{Host: "smtp.example.com", Port: 465, EnableSSL: true, FromEmail: "noreply@example.com", FromName: "系统通知"}
		cfg.ID = 1
		if err := db.DB.Create(&cfg).Error; err != nil {
			logger.Error("初始化 SMTP 配置失败: %v", err)
		}
	}
	return &cfg
}

// SMTPInput SMTP 配置输入
type SMTPInput struct {
	Host     string `json:"host"`
	Port     int    `json:"port"`
	Username string `json:"username"`
	Password string `json:"password"`
	From     string `json:"from"`
	FromName string `json:"from_name"`
	SSL      bool   `json:"ssl"`
	Enabled  bool   `json:"enabled"`
}

// SaveSMTPConfig 保存 SMTP 配置
func SaveSMTPConfig(in *SMTPInput) error {
	var existing models.SMTPConfig
	if err := db.DB.First(&existing, 1).Error; err != nil {
		cfg := models.SMTPConfig{Model: gorm.Model{ID: 1}, Host: in.Host, Port: in.Port, Username: in.Username, Password: in.Password,
			FromEmail: in.From, FromName: in.FromName, EnableSSL: in.SSL, Enabled: in.Enabled}
		return db.DB.Create(&cfg).Error
	}
	return db.DB.Model(&existing).Updates(map[string]interface{}{
		"host":       in.Host,
		"port":       in.Port,
		"username":   in.Username,
		"password":   in.Password,
		"from_email": in.From,
		"from_name":  in.FromName,
		"enable_ssl": in.SSL,
		"enabled":    in.Enabled,
	}).Error
}

// ListMailTemplates 邮件模板列表
func ListMailTemplates() ([]models.MailTemplate, error) {
	var list []models.MailTemplate
	if err := db.DB.Order("code ASC").Find(&list).Error; err != nil {
		return nil, err
	}
	return list, nil
}

// SaveMailTemplate 保存邮件模板
func SaveMailTemplate(code, subject, body string) error {
	var tpl models.MailTemplate
	if err := db.DB.Where("code = ?", code).First(&tpl).Error; err != nil {
		tpl = models.MailTemplate{Code: code, Subject: subject, Body: body}
		return db.DB.Create(&tpl).Error
	}
	tpl.Subject = subject
	tpl.Body = body
	return db.DB.Save(&tpl).Error
}

// GetMailTemplate 获取邮件模板
func GetMailTemplate(code string) (*models.MailTemplate, error) {
	var tpl models.MailTemplate
	if err := db.DB.Where("code = ?", code).First(&tpl).Error; err != nil {
		return nil, err
	}
	return &tpl, nil
}

func renderText(src string, data MailData) (string, error) {
	tpl, err := template.New("mail").Parse(src)
	if err != nil {
		return "", err
	}
	var buf bytes.Buffer
	if err := tpl.Execute(&buf, data); err != nil {
		return "", err
	}
	return buf.String(), nil
}

// SendMail 发送邮件（HTML）
func SendMail(to, subject, body string) error {
	cfg := GetSMTPConfig()
	if !cfg.Enabled || cfg.Host == "" {
		return fmt.Errorf("SMTP 未启用，请在后台配置")
	}
	from := cfg.FromEmail
	fromName := cfg.FromName
	if fromName == "" {
		fromName = from
	}
	msg := fmt.Sprintf("From: %s <%s>\r\nTo: %s\r\nSubject: %s\r\nMIME-Version: 1.0\r\nContent-Type: text/html; charset=UTF-8\r\n\r\n%s",
		fromName, from, to, subject, body)

	addr := fmt.Sprintf("%s:%d", cfg.Host, cfg.Port)
	auth := smtp.PlainAuth("", cfg.Username, cfg.Password, cfg.Host)

	useTLS := cfg.EnableSSL || cfg.Port == 465

	if useTLS {
		conn, err := tls.Dial("tcp", addr, &tls.Config{InsecureSkipVerify: true, ServerName: cfg.Host})
		if err != nil {
			return fmt.Errorf("连接 SMTP 失败: %v", err)
		}
		defer conn.Close()
		client, err := smtp.NewClient(conn, cfg.Host)
		if err != nil {
			return err
		}
		defer client.Close()
		if cfg.Username != "" {
			if err := client.Auth(auth); err != nil {
				return fmt.Errorf("SMTP 认证失败: %v", err)
			}
		}
		return writeMail(client, from, to, msg)
	}

	client, err := smtp.Dial(addr)
	if err != nil {
		return fmt.Errorf("连接 SMTP 失败: %v", err)
	}
	defer client.Close()
	if cfg.Username != "" {
		if err := client.Auth(auth); err != nil {
			return fmt.Errorf("SMTP 认证失败: %v", err)
		}
	}
	return writeMail(client, from, to, msg)
}

func writeMail(client *smtp.Client, from, to, msg string) error {
	if err := client.Mail(from); err != nil {
		return err
	}
	if err := client.Rcpt(to); err != nil {
		return err
	}
	w, err := client.Data()
	if err != nil {
		return err
	}
	if _, err := w.Write([]byte(msg)); err != nil {
		return err
	}
	return w.Close()
}

// SendMailWithTemplate 按模板发送邮件
func SendMailWithTemplate(to, code string, data MailData) error {
	tpl, err := GetMailTemplate(code)
	if err != nil {
		return fmt.Errorf("邮件模板不存在: %s", code)
	}
	subject, err := renderText(tpl.Subject, data)
	if err != nil {
		return err
	}
	body, err := renderText(tpl.Body, data)
	if err != nil {
		return err
	}
	return SendMail(to, subject, body)
}

// SendTestMail 发送测试邮件
func SendTestMail(to string) error {
	return SendMail(to, "测试邮件", "<h3>这是一封测试邮件</h3><p>SMTP 配置正常。</p>")
}

// EnsureMailTemplates 初始化默认邮件模板（不存在时创建）
func EnsureMailTemplates() {
	defaults := []models.MailTemplate{
		{Code: MailTemplateActivate, Name: "账号激活", Subject: "【{{.SiteName}}】请激活您的账号",
			Body: `<h3>欢迎注册 {{.SiteName}}</h3><p>{{.Username}}，您好！</p><p>请点击以下链接激活您的账号（24小时内有效）：</p><p><a href="{{.Link}}">{{.Link}}</a></p><p>如果这不是您的操作，请忽略本邮件。</p>`},
		{Code: MailTemplateResetPwd, Name: "找回密码", Subject: "【{{.SiteName}}】重置密码",
			Body: `<h3>重置密码</h3><p>{{.Username}}，您好！</p><p>请点击以下链接重置密码（24小时内有效）：</p><p><a href="{{.Link}}">{{.Link}}</a></p><p>如果这不是您的操作，请忽略本邮件。</p>`},
		{Code: MailTemplateOrderCreated, Name: "订单创建", Subject: "【{{.SiteName}}】订单创建成功",
			Body: `<h3>订单已创建</h3><p>订单号：{{.OrderNo}}</p><p>商品：{{.ProductName}}</p><p>金额：￥{{.Amount}}</p><p>请尽快完成支付。</p>`},
		{Code: MailTemplateInstanceCreated, Name: "开通成功", Subject: "【{{.SiteName}}】实例开通成功",
			Body: `<h3>实例开通成功</h3><p>实例名称：{{.InstanceName}}</p><p>到期时间：{{.ExpireAt}}</p><p>祝您使用愉快！</p>`},
		{Code: MailTemplateExpiring, Name: "到期提醒", Subject: "【{{.SiteName}}】实例即将到期",
			Body: `<h3>到期提醒</h3><p>您的实例 {{.InstanceName}} 将于 {{.ExpireAt}} 到期，请及时续费以免服务中断。</p>`},
		{Code: MailTemplateSuspended, Name: "实例暂停", Subject: "【{{.SiteName}}】实例已暂停",
			Body: `<h3>实例已暂停</h3><p>您的实例 {{.InstanceName}} 因到期未续费已被暂停，如需恢复请尽快续费。</p>`},
		{Code: MailTemplateTicketReply, Name: "工单回复", Subject: "【{{.SiteName}}】您的工单有新回复",
			Body: `<h3>工单有新回复</h3><p>工单主题：{{.TicketSubject}}</p><p>请登录用户中心查看回复内容。</p>`},
	}
	for _, t := range defaults {
		var cnt int64
		db.DB.Model(&models.MailTemplate{}).Where("code = ?", t.Code).Count(&cnt)
		if cnt == 0 {
			db.DB.Create(&t)
		}
	}
}

// SiteName 获取站点名称（品牌设置）
func SiteName() string {
	bs := NewBrandService()
	s, err := bs.GetSettings()
	if err != nil || s == nil {
		return "云服务器管理平台"
	}
	name := strings.TrimSpace(s.UserSystemName)
	if name == "" || name == "LXD API - 用户中心" {
		if s.FooterText != "" {
			return s.FooterText
		}
	}
	return name
}

package admin

import (
	"github.com/gin-gonic/gin"

	"lxdapi/internal/service"
	"lxdapi/pkg/response"
)

// GetSMTP 读取 SMTP 配置
// @Summary SMTP 配置
// @Tags Admin API - 邮件
// @Router /api/admin/smtp [get]
func GetSMTP(c *gin.Context) {
	response.Success(c, gin.H{"config": service.GetSMTPConfig()})
}

// SaveSMTP 保存 SMTP 配置
// @Summary 保存 SMTP 配置
// @Tags Admin API - 邮件
// @Router /api/admin/smtp [post]
func SaveSMTP(c *gin.Context) {
	var req struct {
		Host     string `json:"host"`
		Port     int    `json:"port"`
		Username string `json:"username"`
		Password string `json:"password"`
		From     string `json:"from"`
		FromName string `json:"from_name"`
		SSL      bool   `json:"ssl"`
		Enabled  bool   `json:"enabled"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, 400, "参数错误")
		return
	}
	if err := service.SaveSMTPConfig(&service.SMTPInput{
		Host: req.Host, Port: req.Port, Username: req.Username,
		Password: req.Password, From: req.From, FromName: req.FromName,
		SSL: req.SSL, Enabled: req.Enabled,
	}); err != nil {
		response.Error(c, 400, err.Error())
		return
	}
	response.Success(c, gin.H{"saved": true})
}

// TestMail 发送测试邮件
// @Summary 发送测试邮件
// @Tags Admin API - 邮件
// @Router /api/admin/smtp/test [post]
func TestMail(c *gin.Context) {
	var req struct {
		To string `json:"to" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, 400, "参数错误")
		return
	}
	if err := service.SendTestMail(req.To); err != nil {
		response.Error(c, 400, err.Error())
		return
	}
	response.Success(c, gin.H{"sent": true})
}

// ListMailTemplates 邮件模板列表
// @Summary 邮件模板
// @Tags Admin API - 邮件
// @Router /api/admin/mail-templates [get]
func ListMailTemplates(c *gin.Context) {
	tpls, err := service.ListMailTemplates()
	if err != nil {
		response.Error(c, 500, "获取模板失败")
		return
	}
	response.Success(c, gin.H{"templates": tpls})
}

// SaveMailTemplate 保存邮件模板
// @Summary 保存邮件模板
// @Tags Admin API - 邮件
// @Router /api/admin/mail-templates/:code [post]
func SaveMailTemplate(c *gin.Context) {
	code := c.Param("code")
	var req struct {
		Subject string `json:"subject"`
		Body    string `json:"body"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, 400, "参数错误")
		return
	}
	if err := service.SaveMailTemplate(code, req.Subject, req.Body); err != nil {
		response.Error(c, 400, err.Error())
		return
	}
	response.Success(c, gin.H{"saved": true})
}

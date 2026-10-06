package public

import (
	"strings"

	"github.com/gin-gonic/gin"

	"lxdapi/internal/service"
	"lxdapi/pkg/response"
)

// Register 用户注册
// @Summary 用户注册
// @Description 注册新用户，若 SMTP 已启用则发送激活邮件，否则直接激活
// @Tags Public API - 认证
// @Accept json
// @Produce json
// @Param request body object{username=string,email=string,password=string} true "注册信息"
// @Success 200 {object} response.Response "注册成功"
// @Router /api/public/register [post]
func Register(c *gin.Context) {
	var req struct {
		Username string `json:"username" binding:"required"`
		Email    string `json:"email" binding:"required"`
		Password string `json:"password" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, 400, "参数错误")
		return
	}
	req.Username = strings.TrimSpace(req.Username)
	req.Email = strings.TrimSpace(req.Email)

	user, err := service.RegisterUser(req.Username, req.Email, req.Password)
	if err != nil {
		response.Error(c, 400, err.Error())
		return
	}

	// 发送激活邮件；SMTP 未启用则直接激活
	smtpOK := false
	if token, terr := service.CreateEmailToken(user.ID, service.TokenActivate); terr == nil {
		link := absURL(c, "/api/public/activate?token="+token.Token)
		if merr := service.SendMailWithTemplate(user.Email, service.MailTemplateActivate, service.MailData{
			"SiteName": service.SiteName(),
			"Username": user.Username,
			"Link":     link,
		}); merr == nil {
			smtpOK = true
		}
	}

	if !smtpOK {
		// 无邮件环境直接激活（演示/内网部署）
		_ = service.ActivateDirect(user.ID)
		response.Success(c, gin.H{"activated": true, "tip": "注册成功（邮件未配置，账户已直接激活）"})
		return
	}
	response.Success(c, gin.H{"activated": false, "tip": "注册成功，请查收激活邮件"})
}

// Activate 激活账号
// @Summary 激活账号
// @Description 通过邮件链接激活账号
// @Tags Public API - 认证
// @Accept json
// @Produce json
// @Param token query string true "激活令牌"
// @Success 200 {object} response.Response "激活成功"
// @Router /api/public/activate [get]
func Activate(c *gin.Context) {
	token := c.Query("token")
	if token == "" {
		response.Error(c, 400, "缺少激活令牌")
		return
	}
	if err := service.ActivateUser(token); err != nil {
		response.Error(c, 400, err.Error())
		return
	}
	response.Success(c, gin.H{"activated": true})
}

// ForgotPassword 发送重置密码邮件
// @Summary 找回密码
// @Description 向注册邮箱发送重置密码链接
// @Tags Public API - 认证
// @Accept json
// @Produce json
// @Param request body object{email=string} true "注册邮箱"
// @Success 200 {object} response.Response "发送成功"
// @Router /api/public/forgot [post]
func ForgotPassword(c *gin.Context) {
	var req struct {
		Email string `json:"email" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, 400, "参数错误")
		return
	}
	user, err := service.GetUserByEmail(req.Email)
	if err != nil {
		// 不暴露邮箱是否存在
		response.Success(c, gin.H{"sent": true})
		return
	}
	token, err := service.CreateEmailToken(user.ID, service.TokenReset)
	if err != nil {
		response.Error(c, 500, "生成重置链接失败")
		return
	}
	link := absURL(c, "/api/public/reset?token="+token.Token)
	if err := service.SendMailWithTemplate(user.Email, service.MailTemplateResetPwd, service.MailData{
		"SiteName": service.SiteName(),
		"Username": user.Username,
		"Link":     link,
	}); err != nil {
		response.Error(c, 500, "邮件发送失败: "+err.Error())
		return
	}
	response.Success(c, gin.H{"sent": true})
}

// ResetPassword 重置密码
// @Summary 重置密码
// @Description 通过邮件链接重置密码
// @Tags Public API - 认证
// @Accept json
// @Produce json
// @Param request body object{token=string,password=string} true "重置信息"
// @Success 200 {object} response.Response "重置成功"
// @Router /api/public/reset [post]
func ResetPassword(c *gin.Context) {
	var req struct {
		Token    string `json:"token" binding:"required"`
		Password string `json:"password" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, 400, "参数错误")
		return
	}
	if err := service.ResetPasswordByToken(req.Token, req.Password); err != nil {
		response.Error(c, 400, err.Error())
		return
	}
	response.Success(c, gin.H{"reset": true})
}

// SiteInfo 站点信息（注册/支付/邮件状态）
// @Summary 站点信息
// @Description 返回注册开关、易支付与 SMTP 是否已配置
// @Tags Public API - 认证
// @Produce json
// @Success 200 {object} response.Response "站点信息"
// @Router /api/public/site-info [get]
func SiteInfo(c *gin.Context) {
	pay := service.GetPayConfig()
	smtp := service.GetSMTPConfig()
	response.Success(c, gin.H{
		"register_enabled": true,
		"pay_enabled":      pay.Enabled,
		"smtp_enabled":     smtp.Enabled,
	})
}

// PayNotify 易支付异步回调
func PayNotify(c *gin.Context) {
	raw := map[string]string{}
	c.Request.ParseForm()
	for k, v := range c.Request.Form {
		if len(v) > 0 {
			raw[k] = v[0]
		}
	}
	orderNo, err := service.VerifyPayCallback(raw)
	if err != nil {
		response.Error(c, 400, err.Error())
		return
	}
	if _, err := service.MarkOrderPaid(orderNo, "epay", raw["trade_no"]); err != nil {
		response.Error(c, 400, err.Error())
		return
	}
	c.String(200, "success")
}

// PayReturn 易支付同步跳转
func PayReturn(c *gin.Context) {
	raw := map[string]string{}
	for k, v := range c.Request.URL.Query() {
		if len(v) > 0 {
			raw[k] = v[0]
		}
	}
	orderNo, err := service.VerifyPayCallback(raw)
	if err != nil {
		c.Redirect(302, "/user/orders")
		return
	}
	_, _ = service.MarkOrderPaid(orderNo, "epay", raw["trade_no"])
	c.Redirect(302, "/user/orders")
}

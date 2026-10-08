package admin

import (
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
	"github.com/mojocn/base64Captcha"
	"golang.org/x/crypto/bcrypt"
	"lxdapi/internal/core"
	"lxdapi/pkg/logger"
	"lxdapi/pkg/response"
)

var captchaStore = base64Captcha.DefaultMemStore

var loginAttempts = struct {
	sync.RWMutex
	attempts map[string]*LoginAttempt
}{
	attempts: make(map[string]*LoginAttempt),
}

type LoginAttempt struct {
	Count      int
	LastAttempt time.Time
	LockedUntil time.Time
}

const (
	maxAttempts = 5
	lockDuration = 15 * time.Minute
	resetDuration = 5 * time.Minute
)

// GetCaptcha 获取验证码
// @Summary 获取登录验证码
// @Description 生成图片验证码用于管理员登录
// @Tags Admin API - 认证
// @Accept json
// @Produce json
// @Success 200 {object} object{code=int,captcha_id=string,image=string} "生成成功"
// @Failure 500 {object} response.Response "生成失败"
// @Router /api/admin/captcha [get]
func GetCaptcha(c *gin.Context) {
	driver := base64Captcha.NewDriverDigit(80, 240, 4, 0.7, 80)
	captcha := base64Captcha.NewCaptcha(driver, captchaStore)
	id, b64s, _, err := captcha.Generate()
	if err != nil {
		response.Error(c, 500, "生成验证码失败")
		return
	}

	session := sessions.Default(c)
	session.Set("captcha_id", id)
	session.Save()

	c.JSON(200, gin.H{
		"code":       200,
		"captcha_id": id,
		"image":      b64s,
	})
}

// checkAdminPassword 校验管理员用户名与密码。
// config 中的 password 字段应存放 bcrypt 哈希；若为明文（历史配置），
// 仍允许登录但会打一条警告日志，提示尽快换成哈希。
func checkAdminPassword(username, password string) bool {
	if username != core.GlobalConfig.Admin.Username {
		return false
	}
	stored := core.GlobalConfig.Admin.Password
	if strings.HasPrefix(stored, "$2a$") || strings.HasPrefix(stored, "$2b$") || strings.HasPrefix(stored, "$2y$") {
		return bcrypt.CompareHashAndPassword([]byte(stored), []byte(password)) == nil
	}
	if stored != "" {
		logger.Warn("管理员密码为明文存储，建议换成 bcrypt 哈希（python3 -c \"import bcrypt; print(bcrypt.hashpw(b'新密码', bcrypt.gensalt()).decode())\"）")
	}
	return password == stored
}

// Login 管理员登录
// @Summary 管理员登录
// @Description 使用用户名、密码和验证码登录管理后台
// @Tags Admin API - 认证
// @Accept json
// @Produce json
// @Param request body object{username=string,password=string,captcha=string} true "登录信息"
// @Success 200 {object} response.Response "登录成功"
// @Failure 400 {object} response.Response "参数错误或验证码错误"
// @Failure 401 {object} response.Response "用户名或密码错误"
// @Router /api/admin/login [post]
func Login(c *gin.Context) {
	var req struct {
		Username string `json:"username" binding:"required"`
		Password string `json:"password" binding:"required"`
		Captcha  string `json:"captcha" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, 400, "参数错误")
		return
	}

	clientIP := c.ClientIP()
	
	loginAttempts.Lock()
	attempt, exists := loginAttempts.attempts[clientIP]
	if !exists {
		attempt = &LoginAttempt{}
		loginAttempts.attempts[clientIP] = attempt
	}
	
	now := time.Now()
	
	if !attempt.LockedUntil.IsZero() && now.Before(attempt.LockedUntil) {
		remaining := int(attempt.LockedUntil.Sub(now).Minutes())
		loginAttempts.Unlock()
		response.Error(c, 429, fmt.Sprintf("登录已被锁定，请在%d分钟后重试", remaining+1))
		return
	}
	
	if attempt.LockedUntil.Before(now) && !attempt.LockedUntil.IsZero() {
		attempt.Count = 0
		attempt.LockedUntil = time.Time{}
	}
	
	if now.Sub(attempt.LastAttempt) > resetDuration {
		attempt.Count = 0
	}
	loginAttempts.Unlock()

	session := sessions.Default(c)
	captchaID := session.Get("captcha_id")
	if core.GlobalConfig.System.Security.EnableCaptcha {
		if captchaID == nil {
			response.Error(c, 400, "验证码已过期，请刷新")
			return
		}

		if !captchaStore.Verify(captchaID.(string), req.Captcha, true) {
			response.Error(c, 400, "验证码错误")
			return
		}
	}

	session.Delete("captcha_id")
	session.Save()

	if !checkAdminPassword(req.Username, req.Password) {
		loginAttempts.Lock()
		attempt.Count++
		attempt.LastAttempt = now
		
		if attempt.Count >= maxAttempts {
			attempt.LockedUntil = now.Add(lockDuration)
			loginAttempts.Unlock()
			response.Error(c, 429, fmt.Sprintf("登录失败次数过多，账户已被锁定%d分钟", int(lockDuration.Minutes())))
			return
		}
		loginAttempts.Unlock()
		
		response.Error(c, 401, fmt.Sprintf("用户名或密码错误，剩余尝试次数: %d", maxAttempts-attempt.Count))
		return
	}

	loginAttempts.Lock()
	delete(loginAttempts.attempts, clientIP)
	loginAttempts.Unlock()

	session.Options(sessions.Options{
		MaxAge:   86400,
		HttpOnly: true,
		// 固定 false：nginx 以 http 对外提供面板时，Secure cookie 会被浏览器丢弃导致所有接口 401
		Secure:   false,
		SameSite: 3,
		Path:     "/",
	})
	
	session.Set("admin_logged_in", true)
	session.Set("admin_username", req.Username)
	session.Save()

	response.Success(c, gin.H{
		"username": req.Username,
		"redirect": "/admin/dashboard",
	})
}

// Logout 管理员登出
// @Summary 管理员登出
// @Description 退出管理后台登录
// @Tags Admin API - 认证
// @Accept json
// @Produce json
// @Success 200 {object} response.Response "登出成功"
// @Security SessionAuth
// @Router /api/admin/logout [post]
func Logout(c *gin.Context) {
	session := sessions.Default(c)
	session.Clear()
	session.Save()
	
	response.Success(c, gin.H{
		"redirect": "/admin/login",
	})
}


package container

import (
	"fmt"
	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
	"github.com/mojocn/base64Captcha"
	"lxdapi/internal/core"
	"lxdapi/internal/service"
	"lxdapi/pkg/response"
	"sync"
	"time"
)

var captchaStore = base64Captcha.DefaultMemStore

var containerVerifyAttempts = struct {
	sync.RWMutex
	attempts map[string]*ContainerVerifyAttempt
}{
	attempts: make(map[string]*ContainerVerifyAttempt),
}

type ContainerVerifyAttempt struct {
	Count      int
	LastAttempt time.Time
	LockedUntil time.Time
}

const (
	containerMaxAttempts = 5
	containerLockDuration = 15 * time.Minute
	containerResetDuration = 5 * time.Minute
)

// GetCaptcha 获取验证码
// @Summary 获取验证码
// @Description 获取容器登录验证码
// @Tags Container API - 认证
// @Accept json
// @Produce json
// @Success 200 {object} object{code=int,captcha_id=string,image=string} "生成成功"
// @Failure 500 {object} response.Response "生成失败"
// @Router /api/container/captcha [get]
func GetCaptcha(c *gin.Context) {
	driver := base64Captcha.NewDriverDigit(80, 240, 4, 0.7, 80)
	captcha := base64Captcha.NewCaptcha(driver, captchaStore)
	id, b64s, _, err := captcha.Generate()
	if err != nil {
		response.Error(c, 500, "生成验证码失败")
		return
	}

	session := sessions.Default(c)
	session.Set("container_captcha_id", id)
	session.Save()

	c.JSON(200, gin.H{
		"code":       200,
		"captcha_id": id,
		"image":      b64s,
	})
}

// VerifyAccess 验证访问
// @Summary 验证访问
// @Description 使用Hash和验证码验证容器访问
// @Tags Container API - 认证
// @Accept json
// @Produce json
// @Param request body object{hash=string,captcha=string} true "验证信息"
// @Success 200 {object} response.Response "验证成功"
// @Failure 400 {object} response.Response "参数错误"
// @Failure 401 {object} response.Response "验证失败"
// @Router /api/container/verify [post]
func VerifyAccess(c *gin.Context) {
	var req struct {
		Hash    string `json:"hash" binding:"required"`
		Captcha string `json:"captcha"` // 关闭验证码时可不传
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, 400, "参数错误")
		return
	}

	clientIP := c.ClientIP()
	
	containerVerifyAttempts.Lock()
	attempt, exists := containerVerifyAttempts.attempts[clientIP]
	if !exists {
		attempt = &ContainerVerifyAttempt{}
		containerVerifyAttempts.attempts[clientIP] = attempt
	}
	
	now := time.Now()
	
	if !attempt.LockedUntil.IsZero() && now.Before(attempt.LockedUntil) {
		remaining := int(attempt.LockedUntil.Sub(now).Minutes())
		containerVerifyAttempts.Unlock()
		response.Error(c, 429, fmt.Sprintf("验证已被锁定，请在%d分钟后重试", remaining+1))
		return
	}
	
	if attempt.LockedUntil.Before(now) && !attempt.LockedUntil.IsZero() {
		attempt.Count = 0
		attempt.LockedUntil = time.Time{}
	}
	
	if now.Sub(attempt.LastAttempt) > containerResetDuration {
		attempt.Count = 0
	}
	containerVerifyAttempts.Unlock()

	session := sessions.Default(c)
	captchaID := session.Get("container_captcha_id")
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

	session.Delete("container_captcha_id")

	cred, err := service.GetContainerByHash(req.Hash)
	if err != nil {
		containerVerifyAttempts.Lock()
		attempt.Count++
		attempt.LastAttempt = now
		
		if attempt.Count >= containerMaxAttempts {
			attempt.LockedUntil = now.Add(containerLockDuration)
			containerVerifyAttempts.Unlock()
			response.Error(c, 429, fmt.Sprintf("验证失败次数过多，已被锁定%d分钟", int(containerLockDuration.Minutes())))
			return
		}
		containerVerifyAttempts.Unlock()
		
		response.Error(c, 401, fmt.Sprintf("访问码无效，剩余尝试次数: %d", containerMaxAttempts-attempt.Count))
		return
	}

	containerVerifyAttempts.Lock()
	delete(containerVerifyAttempts.attempts, clientIP)
	containerVerifyAttempts.Unlock()

	response.Success(c, gin.H{
		"container_name": cred.ContainerName,
		"message":        "验证成功",
	})
}


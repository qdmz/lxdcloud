package middleware

import (
	"net/http"
	"strings"

	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
	"lxdapi/internal/core"
)

// IsHTTPS 判断当前请求是否经由 HTTPS 访问（直连 TLS 或反向代理转发头）
func IsHTTPS(c *gin.Context) bool {
	if c.Request.TLS != nil {
		return true
	}
	if strings.EqualFold(c.GetHeader("X-Forwarded-Proto"), "https") {
		return true
	}
	return strings.EqualFold(c.GetHeader("X-Forwarded-Ssl"), "on")
}

// SessionCookieOptions 按请求协议设置会话 Cookie 属性：
// HTTPS 下带 Secure，纯 HTTP（内网/调试）下不带，避免浏览器丢弃 Cookie 导致全部 401。
// 可通过 system.server.secure_cookie 强制 true/false。
func SessionCookieOptions() gin.HandlerFunc {
	return func(c *gin.Context) {
		secure := IsHTTPS(c)
		switch strings.ToLower(strings.TrimSpace(core.GlobalConfig.System.Server.SecureCookie)) {
		case "true", "1", "yes", "on":
			secure = true
		case "false", "0", "no", "off":
			secure = false
		}
		sessions.Default(c).Options(sessions.Options{
			Path:     "/",
			MaxAge:   86400,
			HttpOnly: true,
			Secure:   secure,
			SameSite: http.SameSiteLaxMode,
		})
		c.Next()
	}
}

// SecurityHeaders 基础安全响应头
func SecurityHeaders() gin.HandlerFunc {
	return func(c *gin.Context) {
		h := c.Writer.Header()
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("X-Frame-Options", "SAMEORIGIN")
		h.Set("Referrer-Policy", "strict-origin-when-cross-origin")
		c.Next()
	}
}

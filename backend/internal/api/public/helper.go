package public

import (
	"strings"

	"github.com/gin-gonic/gin"
	"lxdapi/internal/core"
)

// absURL 基于当前请求构造绝对 URL
func absURL(c *gin.Context, path string) string {
	// 优先使用配置的对外地址，避免 Host 头注入导致邮件链接指向攻击者域名
	if base := strings.TrimRight(strings.TrimSpace(core.GlobalConfig.System.Server.PublicURL), "/"); base != "" {
		return base + path
	}
	scheme := "http"
	if c.Request.TLS != nil || strings.EqualFold(c.GetHeader("X-Forwarded-Proto"), "https") {
		scheme = "https"
	}
	return scheme + "://" + c.Request.Host + path
}

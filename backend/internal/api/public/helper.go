package public

import "github.com/gin-gonic/gin"

// absURL 基于当前请求构造绝对 URL
func absURL(c *gin.Context, path string) string {
	scheme := "http"
	if c.Request.TLS != nil {
		scheme = "https"
	}
	return scheme + "://" + c.Request.Host + path
}

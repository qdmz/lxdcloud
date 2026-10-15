package user

import (
	"github.com/gin-gonic/gin"

	"lxdapi/internal/service"
	"lxdapi/pkg/response"
)

// ListNodeReverseProxies 用户在各节点实例上的反向代理（主控未启用 nginx 插件时使用）
func ListNodeReverseProxies(c *gin.Context) {
	username := c.GetString("username")
	if username == "" {
		response.Error(c, 401, "未授权")
		return
	}
	response.Success(c, service.UserNodeReverseProxies(username))
}

package system

import (
	"github.com/gin-gonic/gin"
	"lxdapi/internal/service"
	"lxdapi/pkg/response"
	"time"
)

func GetAdminAccessToken(c *gin.Context) {
	token, err := service.CreateAccessToken("admin", "admin", 30*time.Minute)
	if err != nil {
		response.Error(c, 500, "生成令牌失败")
		return
	}

	response.Success(c, gin.H{
		"token":      token.Token,
		"jump_url":   "/admin/dashboard?token=" + token.Token,
		"expires_at": token.ExpiresAt,
	})
}

func GetUserAccessToken(c *gin.Context) {
	username := c.Param("username")
	if username == "" {
		response.Error(c, 400, "缺少用户名")
		return
	}

	user, err := service.GetUserByUsername(username)
	if err != nil {
		response.Error(c, 404, "用户不存在")
		return
	}

	token, err := service.CreateAccessToken("user", user.Username, 30*time.Minute)
	if err != nil {
		response.Error(c, 500, "生成令牌失败")
		return
	}

	response.Success(c, gin.H{
		"username":   user.Username,
		"token":      token.Token,
		"jump_url":   "/user/dashboard?token=" + token.Token,
		"expires_at": token.ExpiresAt,
	})
}

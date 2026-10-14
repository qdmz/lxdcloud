package admin

import (
	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
	"lxdapi/internal/service"
	"lxdapi/pkg/logger"
	"lxdapi/pkg/response"
)

// ImpersonateUser 管理员以该用户身份登录用户中心（写入同一会话，不影响管理员登录状态）
func ImpersonateUser(c *gin.Context) {
	u, err := service.GetUser(c.Param("id"))
	if err != nil || u == nil {
		response.Error(c, 404, "用户不存在")
		return
	}
	if u.Status == "disabled" {
		response.Error(c, 400, "该用户已被禁用")
		return
	}
	session := sessions.Default(c)
	session.Set("user_logged_in", true)
	session.Set("user_username", u.Username)
	session.Set("user_id", u.ID)
	if err := session.Save(); err != nil {
		response.Error(c, 500, "会话保存失败")
		return
	}
	admin, _ := c.Get("admin_username")
	logger.Info("管理员 %v 以用户 %s 身份登录用户中心", admin, u.Username)
	response.Success(c, gin.H{"username": u.Username, "redirect": "/user/index.html"})
}

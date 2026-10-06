package user

import (
	"github.com/gin-gonic/gin"

	"lxdapi/internal/service"
	"lxdapi/pkg/response"
)

// ChangePassword 修改密码
// @Summary 修改密码
// @Tags User API - 认证
// @Router /api/user/change-password [post]
func ChangePassword(c *gin.Context) {
	var req struct {
		OldPassword string `json:"old_password" binding:"required"`
		NewPassword string `json:"new_password" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, 400, "参数错误")
		return
	}
	uid := currentUserID(c)
	if uid == 0 {
		return
	}
	if err := service.ChangePassword(uid, req.OldPassword, req.NewPassword); err != nil {
		response.Error(c, 400, err.Error())
		return
	}
	response.Success(c, gin.H{"changed": true})
}

// UpdateProfile 更新资料（昵称/邮箱）
// @Summary 更新资料
// @Tags User API - 认证
// @Router /api/user/profile [post]
func UpdateProfile(c *gin.Context) {
	var req struct {
		Nickname string `json:"nickname"`
		Email    string `json:"email"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, 400, "参数错误")
		return
	}
	uid := currentUserID(c)
	if uid == 0 {
		return
	}
	if err := service.UpdateProfile(uid, req.Nickname, req.Email); err != nil {
		response.Error(c, 400, err.Error())
		return
	}
	response.Success(c, gin.H{"updated": true})
}

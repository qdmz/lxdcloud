package user

import (
	"fmt"

	"github.com/gin-gonic/gin"

	"lxdapi/internal/service"
	"lxdapi/models"
	"lxdapi/pkg/response"
)

// currentUser 从会话取当前登录用户
func currentUser(c *gin.Context) (*models.User, error) {
	username, _ := c.Get("username")
	if username == nil {
		return nil, nil
	}
	return service.GetUserByUsername(username.(string))
}

// currentUserID 当前用户 ID
func currentUserID(c *gin.Context) uint {
	u, err := currentUser(c)
	if err != nil || u == nil {
		response.Error(c, 401, "未登录")
		return 0
	}
	return u.ID
}

// twoDecimal 金额转两位小数字符串
func twoDecimal(v float64) string {
	return fmt.Sprintf("%.2f", v)
}

// mustGetUserOrder 获取用户自己的订单
func mustGetUserOrder(userID, id uint) *models.Order {
	o, err := service.GetUserOrderByID(userID, id)
	if err != nil {
		return nil
	}
	return o
}


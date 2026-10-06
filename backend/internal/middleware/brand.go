package middleware

import (
	"github.com/gin-gonic/gin"
	"lxdapi/internal/service"
	"lxdapi/pkg/logger"
)

func BrandMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		svc := service.NewBrandService()
		settings, err := svc.GetSettings()
		
		path := c.Request.URL.Path
		
		if err != nil {
			logger.Warn("获取品牌设置失败，使用默认值: %v", err)
			if len(path) >= 6 && path[:6] == "/admin" {
				c.Set("SystemName", "LXD API - 管理后台")
				c.Set("SystemTitle", "管理后台 - LXD容器管理系统")
				c.Set("LoginTitle", "管理员登录")
			} else if len(path) >= 5 && path[:5] == "/user" {
				c.Set("SystemName", "LXD API - 用户中心")
				c.Set("SystemTitle", "用户中心 - LXD容器管理系统")
				c.Set("LoginTitle", "用户登录")
			} else if len(path) >= 10 && path[:10] == "/container" {
				c.Set("SystemName", "容器控制面板")
				c.Set("SystemTitle", "容器管理 - LXD容器管理系统")
				c.Set("LoginTitle", "容器登录")
			}
			c.Set("FaviconUrl", "")
			c.Set("FooterText", "LXD API 容器管理平台")
		} else {
			if len(path) >= 6 && path[:6] == "/admin" {
				c.Set("SystemName", settings.AdminSystemName)
				c.Set("SystemTitle", settings.AdminSystemTitle)
				c.Set("LoginTitle", settings.AdminLoginTitle)
				c.Set("BgImage", settings.AdminBgImage)
				c.Set("BgOpacity", settings.AdminBgOpacity)
				c.Set("ContentOpacity", settings.AdminContentOpacity)
			} else if len(path) >= 5 && path[:5] == "/user" {
				c.Set("SystemName", settings.UserSystemName)
				c.Set("SystemTitle", settings.UserSystemTitle)
				c.Set("LoginTitle", settings.UserLoginTitle)
				c.Set("BgImage", settings.UserBgImage)
				c.Set("BgOpacity", settings.UserBgOpacity)
				c.Set("ContentOpacity", settings.UserContentOpacity)
				c.Set("UserNoticeOpacity", settings.UserNoticeOpacity)
			} else if len(path) >= 10 && path[:10] == "/container" {
				c.Set("SystemName", settings.ContainerSystemName)
				c.Set("SystemTitle", settings.ContainerSystemTitle)
				c.Set("LoginTitle", settings.ContainerLoginTitle)
				c.Set("BgImage", settings.ContainerBgImage)
				c.Set("BgOpacity", settings.ContainerBgOpacity)
				c.Set("ContentOpacity", settings.ContainerContentOpacity)
				c.Set("ContainerNoticeOpacity", settings.ContainerNoticeOpacity)
			}
			c.Set("FaviconUrl", settings.FaviconUrl)
			c.Set("FooterText", settings.FooterText)
		}
		
		c.Next()
	}
}

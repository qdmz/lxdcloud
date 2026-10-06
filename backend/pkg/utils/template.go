package utils

import (
	"github.com/gin-gonic/gin"
	"lxdapi/internal/service"
)

func MergeTemplateData(c *gin.Context, data gin.H) gin.H {
	if systemName, exists := c.Get("SystemName"); exists {
		data["SystemName"] = systemName
	}
	if systemTitle, exists := c.Get("SystemTitle"); exists {
		data["title"] = systemTitle
	}
	if loginTitle, exists := c.Get("LoginTitle"); exists {
		data["LoginTitle"] = loginTitle
	}
	if bgImage, exists := c.Get("BgImage"); exists {
		data["BgImage"] = bgImage
	}
	if bgOpacity, exists := c.Get("BgOpacity"); exists {
		data["BgOpacity"] = bgOpacity
	}
	if contentOpacity, exists := c.Get("ContentOpacity"); exists {
		data["ContentOpacity"] = contentOpacity
	}
	if userNoticeOpacity, exists := c.Get("UserNoticeOpacity"); exists {
		data["UserNoticeOpacity"] = userNoticeOpacity
	}
	if containerNoticeOpacity, exists := c.Get("ContainerNoticeOpacity"); exists {
		data["ContainerNoticeOpacity"] = containerNoticeOpacity
	}
	if footerText, exists := c.Get("FooterText"); exists {
		data["FooterText"] = footerText
	}
	if faviconUrl, exists := c.Get("FaviconUrl"); exists {
		data["FaviconUrl"] = faviconUrl
	}
	if useLocalCDN, exists := c.Get("UseLocalCDN"); exists {
		data["UseLocalCDN"] = useLocalCDN
	}
	
	if _, exists := data["title"]; !exists {
		path := c.Request.URL.Path
		svc := service.NewBrandService()
		if settings, err := svc.GetSettings(); err == nil {
			if len(path) >= 6 && path[:6] == "/admin" {
				data["SystemName"] = settings.AdminSystemName
				data["title"] = settings.AdminSystemTitle
				data["LoginTitle"] = settings.AdminLoginTitle
				data["BgImage"] = settings.AdminBgImage
				data["BgOpacity"] = settings.AdminBgOpacity
				data["ContentOpacity"] = settings.AdminContentOpacity
			} else if len(path) >= 5 && path[:5] == "/user" {
				data["SystemName"] = settings.UserSystemName
				data["title"] = settings.UserSystemTitle
				data["LoginTitle"] = settings.UserLoginTitle
				data["BgImage"] = settings.UserBgImage
				data["BgOpacity"] = settings.UserBgOpacity
				data["ContentOpacity"] = settings.UserContentOpacity
				data["UserNoticeOpacity"] = settings.UserNoticeOpacity
			} else if len(path) >= 10 && path[:10] == "/container" {
				data["SystemName"] = settings.ContainerSystemName
				data["title"] = settings.ContainerSystemTitle
				data["LoginTitle"] = settings.ContainerLoginTitle
				data["BgImage"] = settings.ContainerBgImage
				data["BgOpacity"] = settings.ContainerBgOpacity
				data["ContentOpacity"] = settings.ContainerContentOpacity
				data["ContainerNoticeOpacity"] = settings.ContainerNoticeOpacity
			}
			data["FaviconUrl"] = settings.FaviconUrl
			data["FooterText"] = settings.FooterText
			data["UseLocalCDN"] = settings.UseLocalCDN
		} else {
			data["SystemName"] = "LXD API - 管理后台"
			data["title"] = "管理后台 - LXD容器管理系统"
			data["LoginTitle"] = "管理员登录"
			data["FooterText"] = "LXD API 容器管理平台"
		}
	}
	
	// 确保 UseLocalCDN 始终从数据库获取最新值
	if _, exists := data["UseLocalCDN"]; !exists {
		svc := service.NewBrandService()
		if settings, err := svc.GetSettings(); err == nil {
			data["UseLocalCDN"] = settings.UseLocalCDN
		}
	}
	
	return data
}

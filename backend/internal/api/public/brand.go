package public

import (
	"encoding/json"

	"github.com/gin-gonic/gin"
	"lxdapi/internal/service"
	"lxdapi/pkg/logger"
	"lxdapi/pkg/response"
)

// GetBrandSettings 获取品牌设置
// @Summary 获取品牌设置
// @Description 获取公开的品牌设置
// @Tags Public API
// @Accept json
// @Produce json
// @Success 200 {object} response.Response "获取成功"
// @Router /api/public/brand [get]
func GetBrandSettings(c *gin.Context) {
	svc := service.NewBrandService()
	settings, err := svc.GetSettings()
	if err != nil {
		logger.Error("获取品牌设置失败: %v", err)
		response.Success(c, gin.H{
			"system_name":  "LXD API",
			"system_title": "LXD容器管理系统",
			"footer_text":  "LXD API 容器管理平台",
		})
		return
	}
	
	b, _ := json.Marshal(settings)
	m := gin.H{}
	_ = json.Unmarshal(b, &m)
	if m == nil {
		m = gin.H{}
	}
	// 安全：公开接口绝不能返回 TLS 证书私钥（v1.1.0 修复私钥泄露）
	delete(m, "tls_cert_content")
	delete(m, "tls_key_content")
	// 附加旧版前端兼容字段（logo_url 原先误映射为后台背景图，已更正为空，前端回退为首字母 Logo）
	m["site_name"] = settings.AdminSystemName
	m["page_title"] = settings.AdminSystemTitle
	m["logo_url"] = ""
	m["favicon_url"] = settings.FaviconUrl
	response.Success(c, m)
}

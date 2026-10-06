package handlers

import (
	"fmt"
	"github.com/gin-gonic/gin"
	"lxdapi/internal/service"
	"lxdapi/pkg/utils"
	"net/http"
)

func ContainerLogin(c *gin.Context) {
	c.HTML(http.StatusOK, "container/container_login.html", utils.MergeTemplateData(c, gin.H{}))
}

func ContainerDashboard(c *gin.Context) {
	hash := c.Query("hash")
	c.HTML(http.StatusOK, "container/container_dashboard.html", utils.MergeTemplateData(c, gin.H{
		"hash": hash,
	}))
}

func ContainerDashboardBase(c *gin.Context) {
	hash := c.Query("hash")
	template := "container/container_dashboard_base1.html"
	
	svc := service.NewBrandService()
	if settings, err := svc.GetSettings(); err == nil && settings.ContainerBaseTemplate != "" {
		template = fmt.Sprintf("container/container_dashboard_%s.html", settings.ContainerBaseTemplate)
	}
	
	c.HTML(http.StatusOK, template, utils.MergeTemplateData(c, gin.H{
		"hash": hash,
	}))
}

func ContainerDashboardLite(c *gin.Context) {
	hash := c.Query("hash")
	template := "container/container_dashboard_lite1.html"
	
	svc := service.NewBrandService()
	if settings, err := svc.GetSettings(); err == nil && settings.ContainerLiteTemplate != "" {
		template = fmt.Sprintf("container/container_dashboard_%s.html", settings.ContainerLiteTemplate)
	}
	
	c.HTML(http.StatusOK, template, utils.MergeTemplateData(c, gin.H{
		"hash": hash,
	}))
}


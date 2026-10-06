package admin

import (
	"github.com/gin-gonic/gin"
	"lxdapi/internal/db"
	"lxdapi/internal/service"
	"lxdapi/models"
	"lxdapi/pkg/response"
	"lxdapi/pkg/system"
)

// GetDashboard 获取仪表盘数据
// @Summary 获取仪表盘数据
// @Description 获取系统概览数据，包括容器、用户、系统信息等
// @Tags Admin API - 仪表盘
// @Accept json
// @Produce json
// @Success 200 {object} response.Response "获取成功"
// @Security SessionAuth
// @Router /api/admin/dashboard [get]
func GetDashboard(c *gin.Context) {
	var containerCount int64
	db.DB.Model(&models.Container{}).Count(&containerCount)

	var runningCount int64
	db.DB.Model(&models.Container{}).Where("status = ?", "running").Count(&runningCount)

	var stoppedCount int64
	db.DB.Model(&models.Container{}).Where("status = ?", "stopped").Count(&stoppedCount)

	var portMappingV4Count int64
	db.DB.Model(&models.PortMappingV4{}).Count(&portMappingV4Count)
	
	var portMappingV6Count int64
	db.DB.Model(&models.PortMappingV6{}).Count(&portMappingV6Count)

	var userCount int64
	db.DB.Model(&models.User{}).Count(&userCount)

	sysInfo := system.GetSystemInfo()
	
	response.Success(c, gin.H{
		"containers": gin.H{
			"total":   containerCount,
			"running": runningCount,
			"stopped": stoppedCount,
		},
		"port_mappings": gin.H{
			"ipv4": portMappingV4Count,
			"ipv6": portMappingV6Count,
		},
		"users": userCount,
		"system": gin.H{
			"os":           sysInfo.OS,
			"arch":         sysInfo.Arch,
			"kernel":       sysInfo.Kernel,
			"distribution": sysInfo.Distribution,
			"lxd_version":  sysInfo.LXDVersion,
		},
	})
}

// GetHostStats 获取主机状态
// @Summary 获取主机状态
// @Description 获取主机CPU、内存、磁盘等实时状态
// @Tags Admin API - 仪表盘
// @Accept json
// @Produce json
// @Success 200 {object} response.Response "获取成功"
// @Failure 500 {object} response.Response "获取失败"
// @Security SessionAuth
// @Router /api/admin/host/stats [get]
func GetHostStats(c *gin.Context) {
	stats, err := system.GetHostStats()
	if err != nil {
		response.Error(c, 500, "获取主机状态失败: "+err.Error())
		return
	}
	
	response.Success(c, stats)
}


// Dashboard 管理端商业化概览统计
// @Summary 管理端概览（商业化）
// @Tags Admin API - 仪表盘
// @Router /api/admin/dashboard [get]
func Dashboard(c *gin.Context) {
	var userCnt, nodeCnt, orderCnt, paidCnt, productCnt, ticketCnt, openTicketCnt int64
	db.DB.Model(&models.User{}).Count(&userCnt)
	db.DB.Model(&models.Node{}).Count(&nodeCnt)
	db.DB.Model(&models.Order{}).Count(&orderCnt)
	db.DB.Model(&models.Order{}).Where("status = ?", "paid").Count(&paidCnt)
	db.DB.Model(&models.Product{}).Count(&productCnt)
	db.DB.Model(&models.Ticket{}).Count(&ticketCnt)
	db.DB.Model(&models.Ticket{}).Where("status = ?", "open").Count(&openTicketCnt)

	var income float64
	db.DB.Model(&models.Order{}).Where("status = ?", "paid").Select("COALESCE(SUM(amount),0)").Scan(&income)

	var containerTotal, containerRunning, containerStopped int64
	db.DB.Model(&models.Container{}).Count(&containerTotal)
	db.DB.Model(&models.Container{}).Where("status = ?", "running").Count(&containerRunning)
	db.DB.Model(&models.Container{}).Where("status = ?", "stopped").Count(&containerStopped)

	response.Success(c, gin.H{
		"users":           userCnt,
		"nodes":           nodeCnt,
		"orders":          orderCnt,
		"paid_orders":     paidCnt,
		"income":          income,
		"products":        productCnt,
		"tickets":         ticketCnt,
		"open_tickets":    openTicketCnt,
		"active_products": service.CountActiveUserProducts(),
		"containers": gin.H{
			"total":   containerTotal,
			"running": containerRunning,
			"stopped": containerStopped,
		},
	})
}

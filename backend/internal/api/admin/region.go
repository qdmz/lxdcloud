package admin

import (
	"github.com/gin-gonic/gin"

	"lxdapi/internal/service"
	"lxdapi/pkg/response"
)

// ListRegions 机房列表
// @Summary 机房列表
// @Tags Admin API - 节点
// @Router /api/admin/regions [get]
func ListRegions(c *gin.Context) {
	list, err := service.ListRegions(false)
	if err != nil {
		response.Error(c, 500, "获取机房失败")
		return
	}
	response.Success(c, gin.H{"regions": list})
}

// CreateRegion 新增机房
// @Summary 新增机房
// @Tags Admin API - 节点
// @Router /api/admin/regions [post]
func CreateRegion(c *gin.Context) {
	var in service.RegionInput
	if err := c.ShouldBindJSON(&in); err != nil {
		response.Error(c, 400, "参数错误")
		return
	}
	r, err := service.CreateRegion(in)
	if err != nil {
		response.Error(c, 400, err.Error())
		return
	}
	response.Success(c, gin.H{"region": r})
}

// UpdateRegion 更新机房
// @Summary 更新机房
// @Tags Admin API - 节点
// @Router /api/admin/regions/:id [put]
func UpdateRegion(c *gin.Context) {
	var id uint
	if _, err := parseID(c.Param("id"), &id); err != nil {
		response.Error(c, 400, "参数错误")
		return
	}
	var in service.RegionInput
	if err := c.ShouldBindJSON(&in); err != nil {
		response.Error(c, 400, "参数错误")
		return
	}
	if err := service.UpdateRegion(id, in); err != nil {
		response.Error(c, 400, err.Error())
		return
	}
	response.Success(c, gin.H{"updated": true})
}

// DeleteRegion 删除机房
// @Summary 删除机房
// @Tags Admin API - 节点
// @Router /api/admin/regions/:id [delete]
func DeleteRegion(c *gin.Context) {
	var id uint
	if _, err := parseID(c.Param("id"), &id); err != nil {
		response.Error(c, 400, "参数错误")
		return
	}
	if err := service.DeleteRegion(id); err != nil {
		response.Error(c, 400, err.Error())
		return
	}
	response.Success(c, gin.H{"deleted": true})
}

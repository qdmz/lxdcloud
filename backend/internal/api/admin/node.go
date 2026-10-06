package admin

import (
	"github.com/gin-gonic/gin"

	"lxdapi/internal/service"
	"lxdapi/pkg/response"
)

// ListNodes 节点列表
// @Summary 节点列表
// @Tags Admin API - 节点
// @Router /api/admin/nodes [get]
func ListNodes(c *gin.Context) {
	nodes, err := service.ListNodes()
	if err != nil {
		response.Error(c, 500, "获取节点失败")
		return
	}
	response.Success(c, gin.H{"nodes": nodes})
}

// CreateNode 添加节点
// @Summary 添加节点
// @Tags Admin API - 节点
// @Router /api/admin/nodes [post]
func CreateNode(c *gin.Context) {
	var req service.NodeInput
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, 400, "参数错误")
		return
	}
	n, err := service.CreateNode(req)
	if err != nil {
		response.Error(c, 400, err.Error())
		return
	}
	response.Success(c, gin.H{"node": n})
}

// UpdateNode 更新节点
// @Summary 更新节点
// @Tags Admin API - 节点
// @Router /api/admin/nodes/:id [put]
func UpdateNode(c *gin.Context) {
	var id uint
	if _, err := parseID(c.Param("id"), &id); err != nil {
		response.Error(c, 400, "参数错误")
		return
	}
	var req service.NodeInput
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, 400, "参数错误")
		return
	}
	if err := service.UpdateNode(id, req); err != nil {
		response.Error(c, 400, err.Error())
		return
	}
	response.Success(c, gin.H{"updated": true})
}

// DeleteNode 删除节点
// @Summary 删除节点
// @Tags Admin API - 节点
// @Router /api/admin/nodes/:id [delete]
func DeleteNode(c *gin.Context) {
	var id uint
	if _, err := parseID(c.Param("id"), &id); err != nil {
		response.Error(c, 400, "参数错误")
		return
	}
	if err := service.DeleteNode(id); err != nil {
		response.Error(c, 400, err.Error())
		return
	}
	response.Success(c, gin.H{"deleted": true})
}

// TestNode 测试节点连通性
// @Summary 测试节点
// @Tags Admin API - 节点
// @Router /api/admin/nodes/:id/test [post]
func TestNode(c *gin.Context) {
	var id uint
	if _, err := parseID(c.Param("id"), &id); err != nil {
		response.Error(c, 400, "参数错误")
		return
	}
	status, err := service.TestNodeByID(id)
	if err != nil {
		response.Error(c, 400, err.Error())
		return
	}
	response.Success(c, gin.H{"status": status})
}

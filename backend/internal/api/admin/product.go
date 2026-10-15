package admin

import (
	"lxdapi/models"
	"github.com/gin-gonic/gin"

	"lxdapi/internal/service"
	"lxdapi/pkg/response"
)

// ListProducts 商品列表
// @Summary 商品列表
// @Tags Admin API - 商品
// @Router /api/admin/products [get]
func ListProducts(c *gin.Context) {
	// 可选筛选：?filter_node=<id>&filter_region=<id>（不用 node_id，避免被节点转发中间件当成转发请求）
	products, err := service.ListProductsFiltered(service.ProductFilter{
		NodeID:   service.ParseOptionalUint(c.Query("filter_node")),
		RegionID: service.ParseOptionalUint(c.Query("filter_region")),
	})
	if err != nil {
		response.Error(c, 500, "获取商品失败")
		return
	}
	response.Success(c, gin.H{"products": service.AnnotateNodes(products, func(p *models.Product) uint { return p.NodeID })})
}

// CreateProduct 新增商品
// @Summary 新增商品
// @Tags Admin API - 商品
// @Router /api/admin/products [post]
func CreateProduct(c *gin.Context) {
	var p service.ProductInput
	if err := c.ShouldBindJSON(&p); err != nil {
		response.Error(c, 400, "参数错误")
		return
	}
	if err := service.CreateProduct(&p); err != nil {
		response.Error(c, 400, err.Error())
		return
	}
	response.Success(c, gin.H{"created": true})
}

// UpdateProduct 更新商品
// @Summary 更新商品
// @Tags Admin API - 商品
// @Router /api/admin/products/:id [put]
func UpdateProduct(c *gin.Context) {
	var id uint
	if _, err := parseID(c.Param("id"), &id); err != nil {
		response.Error(c, 400, "参数错误")
		return
	}
	var p service.ProductInput
	if err := c.ShouldBindJSON(&p); err != nil {
		response.Error(c, 400, "参数错误")
		return
	}
	if err := service.UpdateProduct(id, &p); err != nil {
		response.Error(c, 400, err.Error())
		return
	}
	response.Success(c, gin.H{"updated": true})
}

// DeleteProduct 删除商品
// @Summary 删除商品
// @Tags Admin API - 商品
// @Router /api/admin/products/:id [delete]
func DeleteProduct(c *gin.Context) {
	var id uint
	if _, err := parseID(c.Param("id"), &id); err != nil {
		response.Error(c, 400, "参数错误")
		return
	}
	if err := service.DeleteProduct(id); err != nil {
		response.Error(c, 400, err.Error())
		return
	}
	response.Success(c, gin.H{"deleted": true})
}

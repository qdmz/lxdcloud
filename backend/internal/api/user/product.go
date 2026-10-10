package user

import (
	"strconv"

	"github.com/gin-gonic/gin"

	"lxdapi/internal/service"
	"lxdapi/pkg/response"
)

// ListProducts 商品列表
// @Summary 商品列表
// @Description 获取全部上架商品
// @Tags User API - 商店
// @Produce json
// @Success 200 {object} response.Response "商品列表"
// @Router /api/user/products [get]
func ListProducts(c *gin.Context) {
	products, err := service.GetActiveProducts()
	if err != nil {
		response.Error(c, 500, "获取商品失败")
		return
	}
	// purchased：当前用户已持有的各商品数量（用于前台显示"您已购买 X/N"）
	purchased := map[uint]int64{}
	if u, err := currentUser(c); err == nil && u != nil {
		uid := u.ID
		for _, p := range products {
			if p.PerUserLimit > 0 {
				purchased[p.ID] = service.CountUserHolding(uid, p.ID)
			}
		}
	}
	response.Success(c, gin.H{"products": products, "purchased": purchased})
}

// GetProduct 商品详情
// @Summary 商品详情
// @Tags User API - 商店
// @Router /api/user/products/:id [get]
func GetProduct(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		response.Error(c, 400, "参数错误")
		return
	}
	p, err := service.GetProduct(uint(id))
	if err != nil {
		response.Error(c, 404, err.Error())
		return
	}
	if p.Status != "active" {
		response.Error(c, 404, "商品已下架")
		return
	}
	response.Success(c, gin.H{"product": p})
}

package public

import (
	"github.com/gin-gonic/gin"
	"lxdapi/internal/service"
	"lxdapi/pkg/response"
)

// publicProduct 首页展示用的商品字段（不含节点、镜像、存储池等内部配置）
type publicProduct struct {
	ID             uint    `json:"id"`
	Name           string  `json:"name"`
	Description    string  `json:"description"`
	Type           string  `json:"type"`
	CPU            int     `json:"cpu"`
	Memory         int     `json:"memory"`
	Disk           int     `json:"disk"`
	TrafficLimit   int64   `json:"traffic_limit"`
	Ingress        int     `json:"ingress"`
	Egress         int     `json:"egress"`
	IPv4Count      int     `json:"ipv4_count"`
	IPv6Count      int     `json:"ipv6_count"`
	PortMappings   int     `json:"port_mappings"`
	Backups        int     `json:"backups"`
	PriceMonthly   float64 `json:"price_monthly"`
	PriceQuarterly float64 `json:"price_quarterly"`
	PriceHalfYear  float64 `json:"price_half_year"`
	PriceYearly    float64 `json:"price_yearly"`
	Stock          int     `json:"stock"`
	PerUserLimit   int     `json:"per_user_limit"`
	RegionID       uint    `json:"region_id"`
	RegionName     string  `json:"region_name"`
}

// ListProducts 公开商品列表（首页价格展示）
// @Summary 公开商品列表
// @Description 返回上架商品的公开字段，供首页价格方案展示，无需登录
// @Tags Public API
// @Produce json
// @Success 200 {object} response.Response "商品列表"
// @Router /api/public/products [get]
func ListProducts(c *gin.Context) {
	var regionID uint
	if v := service.ParseOptionalUint(c.Query("region_id")); v != nil {
		regionID = *v
	}
	list, err := service.GetActiveProductsInRegion(regionID)
	if err != nil {
		response.Error(c, 500, "获取商品失败")
		return
	}
	regionNames := service.RegionNameMap()
	out := make([]publicProduct, 0, len(list))
	for _, p := range list {
		out = append(out, publicProduct{
			ID: p.ID, Name: p.Name, Description: p.Description, Type: p.Type,
			CPU: p.CPU, Memory: p.Memory, Disk: p.Disk, TrafficLimit: p.TrafficLimit,
			Ingress: p.Ingress, Egress: p.Egress, IPv4Count: p.IPv4Count, IPv6Count: p.IPv6Count,
			PortMappings: p.PortMappings, Backups: p.Backups,
			PriceMonthly: p.PriceMonthly, PriceQuarterly: p.PriceQuarterly,
			PriceHalfYear: p.PriceHalfYear, PriceYearly: p.PriceYearly,
			Stock: p.Stock, PerUserLimit: p.PerUserLimit,
			RegionID: p.RegionID, RegionName: regionNames[p.RegionID],
		})
	}
	regions, _ := service.ListRegions(true)
	pubRegions := make([]gin.H, 0, len(regions))
	for _, r := range regions {
		if r.ProductCount > 0 {
			pubRegions = append(pubRegions, gin.H{"id": r.ID, "name": r.Name, "description": r.Description})
		}
	}
	response.Success(c, gin.H{"products": out, "regions": pubRegions})
}

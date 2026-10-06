package service

import (
	"fmt"

	"lxdapi/internal/db"
	"lxdapi/models"
)

// ProductInput 商品入参
type ProductInput struct {
	Name        string  `json:"name"`
	Description string  `json:"description"`
	NodeID      uint    `json:"node_id"`
	Type        string  `json:"type"`
	Image       string  `json:"image"`
	StoragePool string  `json:"storage_pool"`
	NetworkMode string  `json:"network_mode"`
	CPU         int     `json:"cpu"`
	Memory      int     `json:"memory"`
	Disk        int     `json:"disk"`
	Ingress     int     `json:"ingress"`
	Egress      int     `json:"egress"`
	TrafficLimit int64  `json:"traffic_limit"`
	IPv4Count   int     `json:"ipv4_count"`
	IPv6Count   int     `json:"ipv6_count"`
	PortMappings int    `json:"port_mappings"`
	AllowNesting bool   `json:"allow_nesting"`
	MemorySwap  bool    `json:"memory_swap"`
	Privileged  bool    `json:"privileged"`
	Backups     int     `json:"backups"`
	PriceMonthly   float64 `json:"price_monthly"`
	PriceQuarterly float64 `json:"price_quarterly"`
	PriceHalfYear  float64 `json:"price_half_year"`
	PriceYearly    float64 `json:"price_yearly"`
	Stock       int     `json:"stock"`
	Status      string  `json:"status"`
	SortOrder   int     `json:"sort_order"`
}

// ListProducts 商品列表（后台全部）
func ListProducts() ([]models.Product, error) {
	var products []models.Product
	if err := db.DB.Order("sort_order ASC, id ASC").Find(&products).Error; err != nil {
		return nil, err
	}
	return products, nil
}

// GetActiveProducts 上架商品
func GetActiveProducts() ([]models.Product, error) {
	var products []models.Product
	if err := db.DB.Where("status = ?", "active").Order("sort_order ASC, id ASC").Find(&products).Error; err != nil {
		return nil, err
	}
	return products, nil
}

// GetProduct 获取商品
func GetProduct(id uint) (*models.Product, error) {
	var p models.Product
	if err := db.DB.First(&p, id).Error; err != nil {
		return nil, fmt.Errorf("商品不存在")
	}
	return &p, nil
}

// CreateProduct 新增商品
func CreateProduct(in *ProductInput) error {
	if in.Name == "" {
		return fmt.Errorf("商品名称不能为空")
	}
	if in.Memory <= 0 || in.Disk <= 0 {
		return fmt.Errorf("内存与磁盘必须大于 0")
	}
	p := models.Product{
		Name:         in.Name,
		Description:  in.Description,
		NodeID:       in.NodeID,
		Type:         in.Type,
		Image:        in.Image,
		StoragePool:  in.StoragePool,
		NetworkMode:  in.NetworkMode,
		CPU:          in.CPU,
		Memory:       in.Memory,
		Disk:         in.Disk,
		Ingress:      in.Ingress,
		Egress:       in.Egress,
		TrafficLimit: in.TrafficLimit,
		IPv4Count:    in.IPv4Count,
		IPv6Count:    in.IPv6Count,
		PortMappings: in.PortMappings,
		AllowNesting: in.AllowNesting,
		MemorySwap:   in.MemorySwap,
		Privileged:   in.Privileged,
		Backups:      in.Backups,
		PriceMonthly: in.PriceMonthly,
		PriceQuarterly: in.PriceQuarterly,
		PriceHalfYear: in.PriceHalfYear,
		PriceYearly:  in.PriceYearly,
		Stock:        in.Stock,
		Status:       in.Status,
		SortOrder:    in.SortOrder,
	}
	if p.Type == "" {
		p.Type = "container"
	}
	if p.NetworkMode == "" {
		p.NetworkMode = "bridge"
	}
	if p.Status == "" {
		p.Status = "active"
	}
	if p.Stock == 0 {
		p.Stock = 9999
	}
	return db.DB.Create(&p).Error
}

// UpdateProduct 更新商品
func UpdateProduct(id uint, in *ProductInput) error {
	var p models.Product
	if err := db.DB.First(&p, id).Error; err != nil {
		return fmt.Errorf("商品不存在")
	}
	updates := map[string]interface{}{
		"name":           in.Name,
		"description":    in.Description,
		"node_id":        in.NodeID,
		"type":           in.Type,
		"image":          in.Image,
		"storage_pool":   in.StoragePool,
		"network_mode":   in.NetworkMode,
		"cpu":            in.CPU,
		"memory":         in.Memory,
		"disk":           in.Disk,
		"ingress":        in.Ingress,
		"egress":         in.Egress,
		"traffic_limit":  in.TrafficLimit,
		"ipv4_count":     in.IPv4Count,
		"ipv6_count":     in.IPv6Count,
		"port_mappings":  in.PortMappings,
		"allow_nesting":  in.AllowNesting,
		"memory_swap":    in.MemorySwap,
		"privileged":     in.Privileged,
		"backups":        in.Backups,
		"price_monthly":  in.PriceMonthly,
		"price_quarterly": in.PriceQuarterly,
		"price_half_year": in.PriceHalfYear,
		"price_yearly":   in.PriceYearly,
		"stock":          in.Stock,
		"status":         in.Status,
		"sort_order":     in.SortOrder,
	}
	return db.DB.Model(&p).Updates(updates).Error
}

// DeleteProduct 删除商品（存在未完成订单或实例时拒绝）
func DeleteProduct(id uint) error {
	var orders int64
	db.DB.Model(&models.Order{}).Where("product_id = ? AND status IN ?", id, []string{"pending", "paid"}).Count(&orders)
	if orders > 0 {
		return fmt.Errorf("该商品存在未完成订单，无法删除")
	}
	var ups int64
	db.DB.Model(&models.UserProduct{}).Where("product_id = ?", id).Count(&ups)
	if ups > 0 {
		return fmt.Errorf("该商品存在 %d 个用户实例，无法删除", ups)
	}
	return db.DB.Delete(&models.Product{}, id).Error
}

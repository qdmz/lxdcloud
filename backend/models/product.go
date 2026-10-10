package models

import "gorm.io/gorm"

// Product 商品（产品套餐）
type Product struct {
	gorm.Model
	Name        string `gorm:"size:100" json:"name"`
	Description string `gorm:"type:text" json:"description"`
	NodeID      uint   `gorm:"index" json:"node_id"`
	Type        string `gorm:"size:20;default:'container'" json:"type"` // container / vm
	Image       string `gorm:"size:255" json:"image"`                   // 系统模板/镜像
	StoragePool string `gorm:"size:100" json:"storage_pool"`
	NetworkMode string `gorm:"size:50;default:'bridge'" json:"network_mode"`

	// 容器/VM 通用资源配置
	CPU           int   `json:"cpu"`
	Memory        int   `json:"memory"` // MB
	Disk          int   `json:"disk"`   // GB
	Ingress       int   `json:"ingress"`
	Egress        int   `json:"egress"`
	TrafficLimit  int64 `json:"traffic_limit"` // GB
	IPv4Count     int   `json:"ipv4_count"`
	IPv6Count     int   `json:"ipv6_count"`
	PortMappings  int   `json:"port_mappings"`
	AllowNesting  bool  `json:"allow_nesting"`
	MemorySwap    bool  `json:"memory_swap"`
	Privileged    bool  `json:"privileged"`
	Backups       int   `json:"backups"` // 免费备份数

	// 价格（元）
	PriceMonthly   float64 `json:"price_monthly"`
	PriceQuarterly float64 `json:"price_quarterly"`
	PriceHalfYear  float64 `json:"price_half_year"`
	PriceYearly    float64 `json:"price_yearly"`

	// Stock 库存：-1（或任意负数）= 不限；0 = 已售罄；>0 = 剩余数量。新购下单时原子扣减（预占），未支付订单取消时归还
	Stock    int    `gorm:"default:-1" json:"stock"`
	// PerUserLimit 每人限购：0 = 不限；>0 = 每个用户最多同时持有的该商品实例数（含待支付订单）
	PerUserLimit int `gorm:"default:0" json:"per_user_limit"`
	Status   string `gorm:"size:20;default:'active'" json:"status"` // active/hidden/disabled
	SortOrder int   `gorm:"default:0" json:"sort_order"`
}

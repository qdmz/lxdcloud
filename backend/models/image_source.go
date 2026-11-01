package models

import "gorm.io/gorm"

// ImageSource 自定义远程镜像源（v1.1.22；simplestreams 或 LXD 公共远程）。
// 只是镜像源地址列表，存在主控库；具体下载由前端带 node_id 发到目标节点执行。
type ImageSource struct {
	gorm.Model
	Name     string `gorm:"size:100" json:"name"`
	URL      string `gorm:"size:500" json:"url"`
	Protocol string `gorm:"size:20" json:"protocol"` // simplestreams / lxd
}

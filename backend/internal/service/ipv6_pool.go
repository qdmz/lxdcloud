package service

import (
	"context"
	"fmt"
	"lxdapi/internal/db"
	"lxdapi/internal/ipv6"
	"lxdapi/models"
	"lxdapi/pkg/logger"
)

type IPv6Service struct {
}

func NewIPv6Service() *IPv6Service {
	return &IPv6Service{}
}

func (s *IPv6Service) AllocateIPv6(ctx context.Context, containerName, userID string, count int) ([]string, error) {
	if ipv6.GlobalManager == nil {
		return nil, fmt.Errorf("IPv6功能未启用")
	}
	
	var container models.Container
	if err := db.DB.Where("name = ?", containerName).First(&container).Error; err != nil {
		return nil, fmt.Errorf("容器不存在")
	}
	
	if container.IPv6PoolLimit == 0 {
		return nil, fmt.Errorf("容器未启用IPv6地址池功能")
	}
	
	currentIPs, _ := ipv6.GlobalManager.GetContainerIPs(containerName)
	if len(currentIPs)+count > container.IPv6PoolLimit {
		return nil, fmt.Errorf("超过IPv6地址池限制，最多%d个，已有%d个", container.IPv6PoolLimit, len(currentIPs))
	}
	
	ips, err := ipv6.GlobalManager.AllocateIPs(containerName, userID, count)
	if err != nil {
		return nil, err
	}
	
	containerIPv6 := container.PrivateIPv6
	if containerIPv6 == "" {
		for _, ip := range ips {
			db.DB.Unscoped().Where("ip_address = ?", ip).Delete(&models.IPv6Binding{})
		}
		return nil, fmt.Errorf("容器未配置内网IPv6地址")
	}
	
	for _, ip := range ips {
		if err := ipv6.GlobalManager.BindIP(containerName, ip, containerIPv6); err != nil {
			logger.Error("IPv6绑定失败: %v", err)
		}
	}
	
	return ips, nil
}

func (s *IPv6Service) ReleaseIPv6(containerName, ipAddress string) error {
	if ipv6.GlobalManager == nil {
		return fmt.Errorf("IPv6功能未启用")
	}
	
	if err := ipv6.GlobalManager.ReleaseIP(containerName, ipAddress); err != nil {
		return err
	}
	
	logger.OK("释放IPv6成功: %s -> %s", containerName, ipAddress)
	return nil
}

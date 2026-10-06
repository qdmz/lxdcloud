package service

import (
	"context"
	"fmt"
	"lxdapi/internal/db"
	"lxdapi/internal/ipv4"
	"lxdapi/models"
	"lxdapi/pkg/logger"
)

type IPv4Service struct {
}

func NewIPv4Service() *IPv4Service {
	return &IPv4Service{}
}

func (s *IPv4Service) AllocateIPv4(ctx context.Context, containerName, userID string, count int) ([]string, error) {
	if ipv4.GlobalManager == nil {
		return nil, fmt.Errorf("IPv4功能未启用")
	}
	
	var container models.Container
	if err := db.DB.Where("name = ?", containerName).First(&container).Error; err != nil {
		return nil, fmt.Errorf("容器不存在")
	}
	
	if container.IPv4PoolLimit == 0 {
		return nil, fmt.Errorf("容器未启用IPv4地址池功能")
	}
	
	currentIPs, _ := ipv4.GlobalManager.GetContainerIPs(containerName)
	if len(currentIPs)+count > container.IPv4PoolLimit {
		return nil, fmt.Errorf("超过IPv4地址池限制，最多%d个，已有%d个", container.IPv4PoolLimit, len(currentIPs))
	}
	
	ips, err := ipv4.GlobalManager.AllocateIPs(containerName, userID, count)
	if err != nil {
		return nil, err
	}
	
	containerIP := container.PrivateIP
	if containerIP == "" {
		for _, ip := range ips {
			db.DB.Unscoped().Where("ip_address = ?", ip).Delete(&models.IPv4Binding{})
		}
		return nil, fmt.Errorf("容器未配置内网IP")
	}
	
	for _, ip := range ips {
		if err := ipv4.GlobalManager.BindIP(containerName, ip, containerIP); err != nil {
			logger.Error("IPv4绑定失败: %v", err)
		}
	}
	
	return ips, nil
}

func (s *IPv4Service) ReleaseIPv4(containerName, ipAddress string) error {
	if ipv4.GlobalManager == nil {
		return fmt.Errorf("IPv4功能未启用")
	}
	
	if err := ipv4.GlobalManager.ReleaseIP(containerName, ipAddress); err != nil {
		return err
	}
	
	logger.OK("释放IPv4成功: %s -> %s", containerName, ipAddress)
	return nil
}


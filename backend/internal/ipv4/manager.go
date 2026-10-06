package ipv4

import (
	"fmt"
	"lxdapi/internal/db"
	"lxdapi/models"
	"lxdapi/pkg/logger"
	"os/exec"
	"sync"
	"time"
)

type Manager struct {
	mu sync.Mutex
}

var GlobalManager *Manager

func InitManager() error {
	GlobalManager = &Manager{}
	
	if err := GlobalManager.initNftTable(); err != nil {
		logger.Warn("初始化nftables表失败: %v", err)
	}
	
	var count int64
	db.DB.Model(&models.IPv4Pool{}).Count(&count)
	logger.OK("IPv4管理器初始化成功，地址池: %d个IP", count)
	
	if err := GlobalManager.restoreBindings(); err != nil {
		logger.Warn("恢复IPv4绑定失败: %v", err)
	}
	
	return nil
}

func (m *Manager) initNftTable() error {
	exec.Command("nft", "add", "table", "inet", "lxdnat").Run()
	exec.Command("nft", "add", "chain", "inet", "lxdnat", "prerouting", "{", "type", "nat", "hook", "prerouting", "priority", "-100", ";", "}").Run()
	exec.Command("nft", "add", "chain", "inet", "lxdnat", "postrouting", "{", "type", "nat", "hook", "postrouting", "priority", "100", ";", "}").Run()

	exec.Command("nft", "add", "table", "inet", "lxdip").Run()
	exec.Command("nft", "add", "chain", "inet", "lxdip", "prerouting", "{", "type", "nat", "hook", "prerouting", "priority", "-100", ";", "}").Run()
	exec.Command("nft", "add", "chain", "inet", "lxdip", "postrouting", "{", "type", "nat", "hook", "postrouting", "priority", "100", ";", "}").Run()
	
	logger.OK("nftables表初始化完成: lxdnat, lxdip")
	return nil
}

func (m *Manager) AddPortMapping(publicIP string, publicPort, publicPortEnd int, containerIP string, containerPort, containerPortEnd int, protocol, iface string) error {
	return m.addPortDNAT(publicIP, publicPort, publicPortEnd, containerIP, containerPort, containerPortEnd, protocol, iface)
}

func (m *Manager) RemovePortMapping(publicIP string, publicPort, publicPortEnd int, containerIP string, containerPort, containerPortEnd int, protocol, iface string) error {
	return m.removePortDNAT(publicIP, publicPort, publicPortEnd, containerIP, containerPort, containerPortEnd, protocol, iface)
}

func (m *Manager) AllocateIPs(containerName, userID string, count int) ([]string, error) {
	if count == 0 {
		logger.Info("容器 %s 不需要分配IPv4", containerName)
		return nil, nil
	}
	
	m.mu.Lock()
	defer m.mu.Unlock()
	
	var settings models.IPPoolSettings
	db.DB.First(&settings)
	
	var availablePools []models.IPv4Pool
	query := db.DB.Where("status = ?", "available")
	if settings.RandomAssign {
		query = query.Order("RANDOM()")
	}
	if err := query.Limit(count).Find(&availablePools).Error; err != nil {
		return nil, err
	}
	
	if len(availablePools) < count {
		return nil, fmt.Errorf("可用IPv4不足，需要%d个，只有%d个", count, len(availablePools))
	}
	
	var result []string
	for _, pool := range availablePools {
		binding := &models.IPv4Binding{
			IPAddress:     pool.IPAddress,
			ContainerName: containerName,
			UserID:        userID,
			Status:        "allocated",
		}
		if err := db.DB.Create(binding).Error; err != nil {
			logger.Error("保存IPv4绑定记录失败: %v", err)
			continue
		}
		
		db.DB.Model(&pool).Update("status", "used")
		
		result = append(result, pool.IPAddress)
		logger.OK("分配IPv4: %s -> %s", pool.IPAddress, containerName)
	}
	
	return result, nil
}

func (m *Manager) BindIP(containerName, ipAddress, containerIP string) error {
	var pool models.IPv4Pool
	if err := db.DB.Where("ip_address = ?", ipAddress).First(&pool).Error; err != nil {
		return fmt.Errorf("IP不在地址池中: %v", err)
	}
	
	if err := m.addIPToInterface(ipAddress, pool.Interface); err != nil {
		return fmt.Errorf("添加IP到网卡失败: %v", err)
	}
	
	if err := m.addSNAT(ipAddress, containerIP, pool.Interface); err != nil {
		m.removeIPFromInterface(ipAddress, pool.Interface)
		return fmt.Errorf("添加SNAT规则失败: %v", err)
	}
	
	if err := m.addDNAT(ipAddress, containerIP, pool.Interface); err != nil {
		m.removeSNAT(ipAddress, containerIP, pool.Interface)
		m.removeIPFromInterface(ipAddress, pool.Interface)
		return fmt.Errorf("添加DNAT规则失败: %v", err)
	}
	
	db.DB.Model(&models.IPv4Binding{}).
		Where("container_name = ? AND ip_address = ?", containerName, ipAddress).
		Update("status", "bound")
	
	logger.OK("IPv4绑定成功: %s <-> %s", ipAddress, containerIP)
	return nil
}

func (m *Manager) ReleaseIPs(containerName string) error {
	var bindings []models.IPv4Binding
	if err := db.DB.Where("container_name = ?", containerName).Find(&bindings).Error; err != nil {
		return err
	}
	
	var natConfig models.NATConfigV4
	db.DB.First(&natConfig)
	
	for _, binding := range bindings {
		var pool models.IPv4Pool
		if err := db.DB.Where("ip_address = ?", binding.IPAddress).First(&pool).Error; err == nil {
			m.removeAllRules(binding.IPAddress)
			m.removeIPFromInterface(binding.IPAddress, pool.Interface)
		} else {
			logger.Warn("未找到IPv4地址池记录: %s，尝试使用默认网卡删除", binding.IPAddress)
			m.removeAllRules(binding.IPAddress)
			if natConfig.Interface != "" {
				m.removeIPFromInterface(binding.IPAddress, natConfig.Interface)
			}
		}
		db.DB.Model(&models.IPv4Pool{}).Where("ip_address = ?", binding.IPAddress).Update("status", "available")
		logger.Info("释放IPv4: %s (容器: %s)", binding.IPAddress, containerName)
	}
	
	db.DB.Unscoped().Where("container_name = ?", containerName).Delete(&models.IPv4Binding{})
	logger.OK("容器 %s 的所有IPv4已释放", containerName)
	return nil
}

func (m *Manager) ReleaseIP(containerName, ipAddress string) error {
	var binding models.IPv4Binding
	if err := db.DB.Where("container_name = ? AND ip_address = ?", containerName, ipAddress).First(&binding).Error; err != nil {
		return fmt.Errorf("IPv4绑定不存在")
	}
	
	var pool models.IPv4Pool
	if err := db.DB.Where("ip_address = ?", ipAddress).First(&pool).Error; err == nil {
		m.removeAllRules(ipAddress)
		logger.Info("删除 %s 的所有nftables规则", ipAddress)
		
		m.removeIPFromInterface(ipAddress, pool.Interface)
	}
	
	if err := db.DB.Unscoped().Where("container_name = ? AND ip_address = ?", containerName, ipAddress).Delete(&models.IPv4Binding{}).Error; err != nil {
		return fmt.Errorf("删除IPv4绑定记录失败: %v", err)
	}
	
	db.DB.Model(&models.IPv4Pool{}).Where("ip_address = ?", ipAddress).Update("status", "available")
	
	logger.OK("释放IPv4成功: %s (容器: %s)", ipAddress, containerName)
	return nil
}

func (m *Manager) GetContainerIPs(containerName string) ([]string, error) {
	var bindings []models.IPv4Binding
	if err := db.DB.Where("container_name = ?", containerName).Find(&bindings).Error; err != nil {
		return nil, err
	}
	
	var ips []string
	for _, b := range bindings {
		ips = append(ips, b.IPAddress)
	}
	return ips, nil
}

func (m *Manager) restoreBindings() error {
	var bindings []models.IPv4Binding
	if err := db.DB.Where("status = ?", "bound").Find(&bindings).Error; err != nil {
		return err
	}
	
	if len(bindings) == 0 {
		logger.Info("没有需要恢复的IPv4绑定")
		go m.restorePortMappings()
		return nil
	}

	go m.restoreBindingsWithRetry(bindings)
	return nil
}

func (m *Manager) restoreBindingsWithRetry(bindings []models.IPv4Binding) {
	for i := 0; i < 30; i++ {
		if m.checkLxdReady() {
			break
		}
		time.Sleep(2 * time.Second)
	}

	logger.Info("开始恢复 %d 个IPv4绑定...", len(bindings))
	restored := 0
	
	for _, binding := range bindings {
		var pool models.IPv4Pool
		if err := db.DB.Where("ip_address = ?", binding.IPAddress).First(&pool).Error; err != nil {
			logger.Warn("IP %s 不在地址池中: %v", binding.IPAddress, err)
			continue
		}
		
		if err := m.addIPToInterface(binding.IPAddress, pool.Interface); err != nil {
			logger.Warn("恢复IP到网卡失败 %s: %v", binding.IPAddress, err)
			continue
		}

		var container models.Container
		if err := db.DB.Where("name = ?", binding.ContainerName).First(&container).Error; err == nil && container.PrivateIP != "" {
			if !m.checkDNATExists(binding.IPAddress, container.PrivateIP, pool.Interface) {
				m.addSNAT(binding.IPAddress, container.PrivateIP, pool.Interface)
				m.addDNAT(binding.IPAddress, container.PrivateIP, pool.Interface)
			}
		}

		restored++
	}
	
	logger.OK("IPv4绑定恢复完成: %d/%d", restored, len(bindings))

	m.restorePortMappings()
}

func (m *Manager) restorePortMappings() {
	var mappings []models.PortMappingV4
	if err := db.DB.Where("status = ?", "active").Find(&mappings).Error; err != nil {
		logger.Warn("查询端口映射失败: %v", err)
		return
	}

	if len(mappings) == 0 {
		return
	}

	logger.Info("检查 %d 个IPv4端口映射...", len(mappings))
	restored := 0

	for _, mapping := range mappings {
		proto := mapping.Protocol
		if proto == "both" {
			proto = "tcp"
		}
		if m.checkPortDNATExists(mapping.ForwardIP, mapping.PublicPort, mapping.ContainerIP, mapping.ContainerPort, proto, mapping.Interface) {
			continue
		}
		publicPortEnd := mapping.PublicPortEnd
		if publicPortEnd == 0 {
			publicPortEnd = mapping.PublicPort
		}
		containerPortEnd := mapping.ContainerPortEnd
		if containerPortEnd == 0 {
			containerPortEnd = mapping.ContainerPort
		}
		if err := m.addPortDNAT(mapping.ForwardIP, mapping.PublicPort, publicPortEnd, mapping.ContainerIP, mapping.ContainerPort, containerPortEnd, mapping.Protocol, mapping.Interface); err != nil {
			logger.Warn("恢复端口映射失败 %s:%d: %v", mapping.ForwardIP, mapping.PublicPort, err)
			continue
		}
		restored++
	}

	if restored > 0 {
		logger.OK("IPv4端口映射恢复: %d 条", restored)
	}
}

func (m *Manager) checkLxdReady() bool {
	cmd := exec.Command("nft", "list", "table", "inet", "lxdapi")
	return cmd.Run() == nil
}


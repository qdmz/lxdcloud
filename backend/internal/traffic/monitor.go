package traffic

import (
	"context"
	"encoding/json"
	"fmt"
	"lxdapi/internal/core"
	"lxdapi/internal/db"
	"lxdapi/internal/ipv4"
	"lxdapi/internal/ipv6"
	"lxdapi/internal/lxc"
	"lxdapi/models"
	"lxdapi/pkg/logger"
	"lxdapi/pkg/plugin"
	"os/exec"
	"strings"
	"time"
)

type Monitor struct {
	lxcClient *lxc.Client
	interval  time.Duration
	batchSize int
}

var GlobalMonitor *Monitor

func InitMonitor() error {
	cfg := core.GlobalConfig.Traffic
	if !cfg.Enabled {
		logger.Info("流量监控未启用")
		return nil
	}
	
	GlobalMonitor = &Monitor{
		lxcClient: lxc.NewClient(),
		interval:  time.Duration(cfg.Interval) * time.Second,
		batchSize: cfg.BatchSize,
	}
	
	logger.OK("流量监控器初始化成功")
	return nil
}

func (m *Monitor) Start(ctx context.Context) {
	if m == nil {
		return
	}
	
	ticker := time.NewTicker(m.interval)
	defer ticker.Stop()
	
	logger.Info("流量监控已启动，间隔: %v", m.interval)
	
	for {
		select {
		case <-ctx.Done():
			logger.Info("流量监控已停止")
			return
		case <-ticker.C:
			m.collect()
		}
	}
}

func (m *Monitor) collect() {
	m.checkAutoReset()
	m.checkUserTrafficLimits()

	containers, err := m.getRunningContainers()
	if err != nil {
		logger.Error("获取运行中容器列表失败: %v", err)
		return
	}

	if len(containers) == 0 {
		return
	}

	for _, name := range containers {
		m.syncContainerIP(name)

		if m.isUserTrafficLocked(name) {
			m.stopLockedContainer(name)
			continue
		}

		if m.isTrafficLocked(name) {
			m.stopLockedContainer(name)
			continue
		}

		stats := m.getContainerNetStats(name)
		if stats == nil {
			continue
		}

		m.updateTraffic(name, stats)
	}
}

func (m *Monitor) getRunningContainers() ([]string, error) {
	cmd := exec.Command("lxc", "list", "--format=csv", "--columns=n,s")
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("获取容器列表失败: %v", err)
	}
	
	var containers []string
	lines := strings.Split(strings.TrimSpace(string(output)), "\n")
	
	for _, line := range lines {
		if line == "" {
			continue
		}
		
		parts := strings.Split(line, ",")
		if len(parts) >= 2 {
			name := strings.TrimSpace(parts[0])
			status := strings.TrimSpace(parts[1])
			
			if status == "RUNNING" {
				containers = append(containers, name)
			}
		}
	}
	
	return containers, nil
}

type NetworkCounters struct {
	BytesReceived   uint64 `json:"bytes_received"`
	BytesSent       uint64 `json:"bytes_sent"`
	PacketsReceived uint64 `json:"packets_received"`
	PacketsSent     uint64 `json:"packets_sent"`
}

type NetworkInterface struct {
	Counters NetworkCounters `json:"counters"`
}

type ContainerNetworkState struct {
	Eth0 NetworkInterface `json:"eth0"`
}

type ContainerState struct {
	Network ContainerNetworkState `json:"network"`
}

func (m *Monitor) getContainerNetStats(name string) *NetworkCounters {
	cmd := exec.Command("lxc", "query", fmt.Sprintf("/1.0/instances/%s/state", name))
	output, err := cmd.Output()
	if err != nil {
		return nil
	}
	
	var state ContainerState
	if err := json.Unmarshal(output, &state); err != nil {
		return nil
	}
	
	return &state.Network.Eth0.Counters
}

func (m *Monitor) updateTraffic(containerName string, stats *NetworkCounters) {
	var traffic models.Traffic
	err := db.DB.Where("container_name = ?", containerName).First(&traffic).Error
	
	if err != nil {
		traffic = models.Traffic{
			ContainerName: containerName,
			RxBytes:       int64(stats.BytesReceived),
			TxBytes:       int64(stats.BytesSent),
			TotalGB:       float64(stats.BytesReceived+stats.BytesSent) / (1024 * 1024 * 1024),
			LastUpdate:    time.Now(),
		}
		db.DB.Create(&traffic)
		return
	}
	
	newRx := stats.BytesReceived
	newTx := stats.BytesSent
	
	var deltaRx, deltaTx uint64
	
	if newRx >= uint64(traffic.RxBytes) {
		deltaRx = newRx - uint64(traffic.RxBytes)
	} else {
		deltaRx = newRx
	}
	
	if newTx >= uint64(traffic.TxBytes) {
		deltaTx = newTx - uint64(traffic.TxBytes)
	} else {
		deltaTx = newTx
	}
	
	if deltaRx > 0 || deltaTx > 0 {
		incrementGB := float64(deltaRx+deltaTx) / (1024 * 1024 * 1024)
		totalGB := traffic.TotalGB + incrementGB
		
		traffic.RxBytes = int64(newRx)
		traffic.TxBytes = int64(newTx)
		traffic.TotalGB = totalGB
		traffic.LastUpdate = time.Now()
		
		db.DB.Save(&traffic)
		
		if traffic.LimitGB > 0 && totalGB >= float64(traffic.LimitGB) {
			logger.Warn("容器 %s 流量超限: %.2fGB / %dGB", containerName, totalGB, traffic.LimitGB)
			m.handleOverLimit(containerName, totalGB, traffic.LimitGB)
		}
	}
}

func (m *Monitor) handleOverLimit(containerName string, current float64, limit int) {
	db.DB.Model(&models.Traffic{}).Where("container_name = ?", containerName).Update("locked", true)
	
	ctx := context.Background()
	
	if err := m.lxcClient.StopContainer(ctx, containerName); err != nil {
		logger.Error("自动停止超限容器失败 %s: %v", containerName, err)
		return
	}
	
	db.DB.Model(&models.Container{}).Where("name = ?", containerName).Update("status", "stopped")
	logger.OK("容器因流量超限已锁定并停止: %s", containerName)
	
	if mgr := plugin.GetManager(); mgr != nil {
		mgr.GetHookManager().TriggerAsync(plugin.HookTrafficOverLimit, ctx, map[string]interface{}{
			"name":    containerName,
			"current": uint64(current * 1024 * 1024 * 1024),
			"limit":   uint64(limit) * 1024 * 1024 * 1024,
		})
	}
}

func (m *Monitor) isTrafficLocked(containerName string) bool {
	var traffic models.Traffic
	if err := db.DB.Where("container_name = ?", containerName).First(&traffic).Error; err != nil {
		return false
	}
	
	if traffic.Locked {
		if traffic.LimitGB == 0 || traffic.TotalGB < float64(traffic.LimitGB) {
			db.DB.Model(&models.Traffic{}).Where("container_name = ?", containerName).Update("locked", false)
			logger.OK("容器 %s 流量已恢复正常，自动解锁 (使用: %.2fGB / 限制: %dGB)", containerName, traffic.TotalGB, traffic.LimitGB)
			return false
		}
	}
	
	return traffic.Locked
}

func (m *Monitor) stopLockedContainer(containerName string) {
	var container models.Container
	if err := db.DB.Where("name = ?", containerName).First(&container).Error; err != nil {
		return
	}
	
	if container.Status == "stopped" {
		return
	}
	
	ctx := context.Background()
	if err := m.lxcClient.StopContainer(ctx, containerName); err == nil {
		db.DB.Model(&models.Container{}).Where("name = ?", containerName).Update("status", "stopped")
		logger.Warn("流量锁定容器尝试运行已被自动停止: %s", containerName)
	}
}

func (m *Monitor) GetTraffic(containerName string) (*models.Traffic, error) {
	var traffic models.Traffic
	err := db.DB.Where("container_name = ?", containerName).First(&traffic).Error
	if err != nil {
		return nil, err
	}
	return &traffic, nil
}



func (m *Monitor) checkAutoReset() {
	now := time.Now()
	if now.Day() != 1 {
		return
	}
	
	var traffics []models.Traffic
	firstDayOfMonth := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())
	if err := db.DB.Where("last_reset < ? OR last_reset IS NULL", firstDayOfMonth).Find(&traffics).Error; err != nil {
		return
	}
	
	for _, traffic := range traffics {
		m.autoResetTraffic(traffic.ContainerName)
	}
}

func (m *Monitor) autoResetTraffic(containerName string) {
	stats := m.getContainerNetStats(containerName)
	if stats == nil {
		return
	}
	
	var traffic models.Traffic
	if err := db.DB.Where("container_name = ?", containerName).First(&traffic).Error; err != nil {
		return
	}
	
	traffic.RxBytes = int64(stats.BytesReceived)
	traffic.TxBytes = int64(stats.BytesSent)
	traffic.TotalGB = 0
	traffic.Locked = false
	traffic.LastUpdate = time.Now()
	traffic.LastReset = time.Now()
	
	db.DB.Save(&traffic)
	
	db.DB.Model(&models.Container{}).Where("name = ?", containerName).Update("status", "stopped")
	
	logger.OK("流量已自动重置并解锁: %s", containerName)
}

func (m *Monitor) ResetTraffic(containerName string) error {
	stats := m.getContainerNetStats(containerName)
	if stats == nil {
		return fmt.Errorf("获取容器流量数据失败")
	}
	
	var container models.Container
	if err := db.DB.Where("name = ?", containerName).First(&container).Error; err != nil {
		return fmt.Errorf("容器不存在: %v", err)
	}
	
	if err := db.DB.Unscoped().Where("container_name = ?", containerName).Delete(&models.Traffic{}).Error; err != nil {
		return fmt.Errorf("重置流量统计失败: %v", err)
	}
	
	resetDay := container.CreatedAt.Day()
	
	traffic := models.Traffic{
		ContainerName: containerName,
		RxBytes:       int64(stats.BytesReceived),
		TxBytes:       int64(stats.BytesSent),
		TotalGB:       0,
		LimitGB:       container.TrafficLimit,
		ResetDay:      1, // 固定每月1号
		Locked:        false,
		LastUpdate:    time.Now(),
		LastReset:     time.Now(),
	}
	
	if err := db.DB.Create(&traffic).Error; err != nil {
		return fmt.Errorf("创建重置基准记录失败: %v", err)
	}
	
	logger.OK("流量统计已重置: %s (重置日期: 每月%d号)", containerName, resetDay)
	return nil
}

func (m *Monitor) checkUserTrafficLimits() {
	var users []models.User
	if err := db.DB.Where("traffic_limit > 0 AND traffic_locked = ?", false).Find(&users).Error; err != nil {
		return
	}
	
	for _, user := range users {
		var containers []models.Container
		if err := db.DB.Where("user_id = ?", user.Username).Find(&containers).Error; err != nil {
			continue
		}
		
		totalUsage := user.TrafficUsed
		for _, c := range containers {
			var traffic models.Traffic
			if err := db.DB.Where("container_name = ?", c.Name).First(&traffic).Error; err == nil {
				totalUsage += traffic.TotalGB
			}
		}
		
		if totalUsage >= float64(user.TrafficLimit) {
			m.handleUserOverLimit(&user, containers, totalUsage)
		}
	}
}

func (m *Monitor) handleUserOverLimit(user *models.User, containers []models.Container, currentUsage float64) {
	db.DB.Model(user).Update("traffic_locked", true)
	
	ctx := context.Background()
	for _, c := range containers {
		if c.Status == "stopped" {
			continue
		}
		if err := m.lxcClient.StopContainer(ctx, c.Name); err != nil {
			logger.Error("停止用户超限容器失败 %s: %v", c.Name, err)
			continue
		}
		db.DB.Model(&models.Container{}).Where("name = ?", c.Name).Update("status", "stopped")
	}
	
	logger.Warn("用户 %s 流量超限: %.2fGB / %dGB，已停止所有容器", user.Username, currentUsage, user.TrafficLimit)
	
	if mgr := plugin.GetManager(); mgr != nil {
		mgr.GetHookManager().TriggerAsync(plugin.HookTrafficOverLimit, ctx, map[string]interface{}{
			"type":     "user",
			"username": user.Username,
			"current":  uint64(currentUsage * 1024 * 1024 * 1024),
			"limit":    uint64(user.TrafficLimit) * 1024 * 1024 * 1024,
		})
	}
}

func (m *Monitor) isUserTrafficLocked(containerName string) bool {
	var container models.Container
	if err := db.DB.Where("name = ?", containerName).First(&container).Error; err != nil {
		return false
	}
	
	var user models.User
	if err := db.DB.Where("username = ?", container.UserID).First(&user).Error; err != nil {
		return false
	}
	
	if user.TrafficLocked {
		if user.TrafficLimit == 0 {
			db.DB.Model(&user).Update("traffic_locked", false)
			logger.OK("用户 %s 无流量限制，自动解锁", user.Username)
			return false
		}
		
		var containers []models.Container
		db.DB.Where("user_id = ?", user.Username).Find(&containers)
		
		totalUsage := user.TrafficUsed
		for _, c := range containers {
			var traffic models.Traffic
			if err := db.DB.Where("container_name = ?", c.Name).First(&traffic).Error; err == nil {
				totalUsage += traffic.TotalGB
			}
		}
		
		if totalUsage < float64(user.TrafficLimit) {
			db.DB.Model(&user).Update("traffic_locked", false)
			logger.OK("用户 %s 流量已恢复正常，自动解锁 (使用: %.2fGB / 限制: %dGB)", user.Username, totalUsage, user.TrafficLimit)
			return false
		}
	}
	
	return user.TrafficLocked
}

func (m *Monitor) syncContainerIP(containerName string) {
	var container models.Container
	if err := db.DB.Where("name = ?", containerName).First(&container).Error; err != nil {
		return
	}

	ctx := context.Background()

	actualIPv4, _ := m.lxcClient.GetContainerIP(ctx, containerName)
	actualIPv6, _ := m.lxcClient.GetContainerIPv6(ctx, containerName)

	ipv4Changed := actualIPv4 != "" && actualIPv4 != container.PrivateIP
	ipv6Changed := actualIPv6 != "" && actualIPv6 != container.PrivateIPv6

	if ipv4Changed {
		logger.Warn("检测到容器 %s 的IPv4地址变化: %s -> %s", containerName, container.PrivateIP, actualIPv4)

		oldIP := container.PrivateIP
		db.DB.Model(&container).Update("private_ip", actualIPv4)

		m.updateIPv4Rules(containerName, oldIP, actualIPv4)

		logger.OK("容器 %s 的IPv4地址已同步并更新规则", containerName)
	}

	if ipv6Changed {
		logger.Warn("检测到容器 %s 的IPv6地址变化: %s -> %s", containerName, container.PrivateIPv6, actualIPv6)

		oldIP := container.PrivateIPv6
		db.DB.Model(&container).Update("private_ipv6", actualIPv6)

		m.updateIPv6Rules(containerName, oldIP, actualIPv6)

		logger.OK("容器 %s 的IPv6地址已同步并更新规则", containerName)
	}
}

func (m *Monitor) updateIPv4Rules(containerName, oldIP, newIP string) {
	if ipv4.GlobalManager == nil {
		return
	}

	var bindings []models.IPv4Binding
	if err := db.DB.Where("container_name = ?", containerName).Find(&bindings).Error; err == nil {
		for _, binding := range bindings {
			var pool models.IPv4Pool
			if err := db.DB.Where("ip_address = ?", binding.IPAddress).First(&pool).Error; err != nil {
				continue
			}

			ipv4.GlobalManager.RemovePortMapping(binding.IPAddress, 0, 0, oldIP, 0, 0, "", pool.Interface)

			if err := ipv4.GlobalManager.BindIP(containerName, binding.IPAddress, newIP); err != nil {
				logger.Warn("重新绑定IPv4失败 %s: %v", binding.IPAddress, err)
			}
		}
	}

	var mappings []models.PortMappingV4
	if err := db.DB.Where("container_name = ?", containerName).Find(&mappings).Error; err == nil {
		for _, mapping := range mappings {
			publicPortEnd := mapping.PublicPortEnd
			if publicPortEnd == 0 {
				publicPortEnd = mapping.PublicPort
			}
			containerPortEnd := mapping.ContainerPortEnd
			if containerPortEnd == 0 {
				containerPortEnd = mapping.ContainerPort
			}

			if err := ipv4.GlobalManager.RemovePortMapping(mapping.ForwardIP, mapping.PublicPort, publicPortEnd, oldIP, mapping.ContainerPort, containerPortEnd, mapping.Protocol, mapping.Interface); err != nil {
				logger.Warn("删除旧IPv4端口映射失败 %s:%d: %v", mapping.ForwardIP, mapping.PublicPort, err)
				continue
			}

			if err := ipv4.GlobalManager.AddPortMapping(mapping.ForwardIP, mapping.PublicPort, publicPortEnd, newIP, mapping.ContainerPort, containerPortEnd, mapping.Protocol, mapping.Interface); err != nil {
				logger.Warn("重新添加IPv4端口映射失败 %s:%d: %v", mapping.ForwardIP, mapping.PublicPort, err)
				continue
			}

			db.DB.Model(&mapping).Update("container_ip", newIP)
		}
	}
}

func (m *Monitor) updateIPv6Rules(containerName, oldIP, newIP string) {
	if ipv6.GlobalManager == nil {
		return
	}

	var bindings []models.IPv6Binding
	if err := db.DB.Where("container_name = ?", containerName).Find(&bindings).Error; err == nil {
		for _, binding := range bindings {
			var pool models.IPv6Pool
			if err := db.DB.Where("ip_address = ?", binding.IPAddress).First(&pool).Error; err != nil {
				continue
			}

			ipv6.GlobalManager.RemovePortMapping(binding.IPAddress, 0, 0, oldIP, 0, 0, "", pool.Interface)

			if err := ipv6.GlobalManager.BindIP(containerName, binding.IPAddress, newIP); err != nil {
				logger.Warn("重新绑定IPv6失败 %s: %v", binding.IPAddress, err)
			}
		}
	}

	var mappings []models.PortMappingV6
	if err := db.DB.Where("container_name = ?", containerName).Find(&mappings).Error; err == nil {
		for _, mapping := range mappings {
			publicPortEnd := mapping.PublicPortEnd
			if publicPortEnd == 0 {
				publicPortEnd = mapping.PublicPort
			}
			containerPortEnd := mapping.ContainerPortEnd
			if containerPortEnd == 0 {
				containerPortEnd = mapping.ContainerPort
			}

			if err := ipv6.GlobalManager.RemovePortMapping(mapping.ForwardIP, mapping.PublicPort, publicPortEnd, oldIP, mapping.ContainerPort, containerPortEnd, mapping.Protocol, mapping.Interface); err != nil {
				logger.Warn("删除旧IPv6端口映射失败 %s:%d: %v", mapping.ForwardIP, mapping.PublicPort, err)
				continue
			}

			if err := ipv6.GlobalManager.AddPortMapping(mapping.ForwardIP, mapping.PublicPort, publicPortEnd, newIP, mapping.ContainerPort, containerPortEnd, mapping.Protocol, mapping.Interface); err != nil {
				logger.Warn("重新添加IPv6端口映射失败 %s:%d: %v", mapping.ForwardIP, mapping.PublicPort, err)
				continue
			}

			db.DB.Model(&mapping).Update("container_ip", newIP)
		}
	}
}

func (m *Monitor) ResetUserTraffic(username string) error {
	var user models.User
	if err := db.DB.Where("username = ?", username).First(&user).Error; err != nil {
		return fmt.Errorf("用户不存在: %v", err)
	}
	
	var containers []models.Container
	db.DB.Where("user_id = ?", username).Find(&containers)
	
	for _, c := range containers {
		m.ResetTraffic(c.Name)
	}
	
	db.DB.Model(&user).Update("traffic_locked", false)
	logger.OK("用户流量已重置: %s", username)
	return nil
}

package system

import (
	"os/exec"
	"strconv"
	"strings"
	"time"
)

type HostStats struct {
	CPU     CPUStats     `json:"cpu"`
	Memory  MemoryStats  `json:"memory"`
	Disks   []DiskInfo   `json:"disks"`
	Network NetworkStats `json:"network"`
	Load    LoadStats    `json:"load"`
	Uptime  string       `json:"uptime"`
}

type NetworkStats struct {
	RxBytes    uint64 `json:"rx_bytes"`
	TxBytes    uint64 `json:"tx_bytes"`
	RxBytesStr string `json:"rx_bytes_str"`
	TxBytesStr string `json:"tx_bytes_str"`
	RxSpeed    uint64 `json:"rx_speed"`
	TxSpeed    uint64 `json:"tx_speed"`
	RxSpeedStr string `json:"rx_speed_str"`
	TxSpeedStr string `json:"tx_speed_str"`
}

type DiskInfo struct {
	Name         string  `json:"name"`
	Total        uint64  `json:"total"`
	Used         uint64  `json:"used"`
	Free         uint64  `json:"free"`
	UsagePercent float64 `json:"usage_percent"`
	TotalStr     string  `json:"total_str"`
	UsedStr      string  `json:"used_str"`
}

type CPUStats struct {
	Cores       int     `json:"cores"`
	UsagePercent float64 `json:"usage_percent"`
}

type MemoryStats struct {
	Total       uint64  `json:"total"`
	Used        uint64  `json:"used"`
	Free        uint64  `json:"free"`
	UsagePercent float64 `json:"usage_percent"`
	TotalStr    string  `json:"total_str"`
	UsedStr     string  `json:"used_str"`
	FreeStr     string  `json:"free_str"`
}



type LoadStats struct {
	Load1  float64 `json:"load1"`
	Load5  float64 `json:"load5"`
	Load15 float64 `json:"load15"`
}

var startTime = time.Now()
var lastRxBytes uint64
var lastTxBytes uint64
var lastNetworkTime time.Time

func GetHostStats() (*HostStats, error) {
	stats := &HostStats{}
	
	cpuStats, err := getCPUStats()
	if err == nil {
		stats.CPU = cpuStats
	}
	
	memStats, err := getMemoryStats()
	if err == nil {
		stats.Memory = memStats
	}
	
	disks, err := getDisksStats()
	if err == nil {
		stats.Disks = disks
	}

	networkStats, err := getNetworkStats()
	if err == nil {
		stats.Network = networkStats
	}

	loadStats, err := getLoadStats()
	if err == nil {
		stats.Load = loadStats
	}
	
	stats.Uptime = getUptime()
	
	return stats, nil
}

func getCPUStats() (CPUStats, error) {
	stats := CPUStats{}

	output, err := exec.Command("nproc").Output()
	if err == nil {
		cores, _ := strconv.Atoi(strings.TrimSpace(string(output)))
		stats.Cores = cores
	}

	output, err = exec.Command("bash", "-c", `vmstat 1 2 | tail -1 | awk '{print $15}'`).Output()
	if err == nil {
		idleStr := strings.TrimSpace(string(output))
		if idlePercent, err := strconv.ParseFloat(idleStr, 64); err == nil {
			stats.UsagePercent = 100 - idlePercent
			if stats.UsagePercent < 0 {
				stats.UsagePercent = 0
			}
			if stats.UsagePercent > 100 {
				stats.UsagePercent = 100
			}
		}
	}

	return stats, nil
}

func getMemoryStats() (MemoryStats, error) {
	stats := MemoryStats{}
	
	output, err := exec.Command("free", "-b").Output()
	if err != nil {
		return stats, err
	}
	
	lines := strings.Split(string(output), "\n")
	if len(lines) < 2 {
		return stats, nil
	}
	
	fields := strings.Fields(lines[1])
	if len(fields) >= 3 {
		stats.Total, _ = strconv.ParseUint(fields[1], 10, 64)
		stats.Used, _ = strconv.ParseUint(fields[2], 10, 64)
		stats.Free, _ = strconv.ParseUint(fields[3], 10, 64)
		
		if stats.Total > 0 {
			stats.UsagePercent = float64(stats.Used) / float64(stats.Total) * 100
		}
		
		stats.TotalStr = formatBytes(stats.Total)
		stats.UsedStr = formatBytes(stats.Used)
		stats.FreeStr = formatBytes(stats.Free)
	}
	
	return stats, nil
}

func getDisksStats() ([]DiskInfo, error) {
	var disks []DiskInfo

	// 获取物理磁盘列表: lsblk -b -d -n -o NAME,SIZE,TYPE
	output, err := exec.Command("lsblk", "-b", "-d", "-n", "-o", "NAME,SIZE,TYPE").Output()
	if err != nil {
		return nil, err
	}

	// 获取各分区使用情况: df -B1
	dfOutput, _ := exec.Command("df", "-B1").Output()
	dfLines := strings.Split(string(dfOutput), "\n")

	// 解析 df 输出，按磁盘设备汇总已使用空间
	diskUsed := make(map[string]uint64)
	for _, line := range dfLines {
		fields := strings.Fields(line)
		if len(fields) < 4 {
			continue
		}
		device := fields[0]
		// 只处理 /dev/sd* 或 /dev/nvme* 或 /dev/vd* 等真实设备
		if !strings.HasPrefix(device, "/dev/") {
			continue
		}
		// 提取磁盘名（去掉分区号）: /dev/sda1 -> sda, /dev/nvme0n1p1 -> nvme0n1
		diskName := extractDiskName(device)
		if diskName == "" {
			continue
		}
		used, _ := strconv.ParseUint(fields[2], 10, 64)
		diskUsed[diskName] += used
	}

	// 解析 lsblk 输出
	lines := strings.Split(string(output), "\n")
	for _, line := range lines {
		fields := strings.Fields(line)
		if len(fields) < 3 {
			continue
		}
		name := fields[0]
		size, _ := strconv.ParseUint(fields[1], 10, 64)
		dtype := fields[2]

		// 只处理 disk 类型，排除 loop、rom 等
		if dtype != "disk" {
			continue
		}

		// 排除小于 1GB 的设备（可能是USB、虚拟设备等）
		if size < 1024*1024*1024 {
			continue
		}

		used := diskUsed[name]
		free := size - used
		if used > size {
			used = size
			free = 0
		}

		var usagePercent float64
		if size > 0 {
			usagePercent = float64(used) / float64(size) * 100
		}

		disks = append(disks, DiskInfo{
			Name:         name,
			Total:        size,
			Used:         used,
			Free:         free,
			UsagePercent: usagePercent,
			TotalStr:     formatBytes(size),
			UsedStr:      formatBytes(used),
		})
	}

	return disks, nil
}

// extractDiskName 从设备路径提取磁盘名
// /dev/sda1 -> sda, /dev/nvme0n1p1 -> nvme0n1, /dev/vda1 -> vda
func extractDiskName(device string) string {
	device = strings.TrimPrefix(device, "/dev/")

	// nvme 设备: nvme0n1p1 -> nvme0n1
	if strings.HasPrefix(device, "nvme") {
		if idx := strings.Index(device, "p"); idx > 0 {
			// 确保 p 后面是数字（分区号）
			if idx+1 < len(device) && device[idx+1] >= '0' && device[idx+1] <= '9' {
				return device[:idx]
			}
		}
		return device
	}

	// sd/vd/hd 设备: sda1 -> sda
	if strings.HasPrefix(device, "sd") || strings.HasPrefix(device, "vd") || strings.HasPrefix(device, "hd") {
		// 去掉末尾的数字（分区号）
		name := strings.TrimRight(device, "0123456789")
		return name
	}

	return ""
}

func getNetworkStats() (NetworkStats, error) {
	stats := NetworkStats{}

	// 读取 /proc/net/dev 获取网络流量
	output, err := exec.Command("cat", "/proc/net/dev").Output()
	if err != nil {
		return stats, err
	}

	var totalRx, totalTx uint64
	lines := strings.Split(string(output), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if !strings.Contains(line, ":") {
			continue
		}
		// 跳过 lo 回环接口
		if strings.HasPrefix(line, "lo:") {
			continue
		}
		// 解析: eth0: rx_bytes ... tx_bytes ...
		parts := strings.SplitN(line, ":", 2)
		if len(parts) < 2 {
			continue
		}
		fields := strings.Fields(parts[1])
		if len(fields) < 10 {
			continue
		}
		rxBytes, _ := strconv.ParseUint(fields[0], 10, 64)
		txBytes, _ := strconv.ParseUint(fields[8], 10, 64)
		totalRx += rxBytes
		totalTx += txBytes
	}

	stats.RxBytes = totalRx
	stats.TxBytes = totalTx
	stats.RxBytesStr = formatBytes(totalRx)
	stats.TxBytesStr = formatBytes(totalTx)

	// 计算速率
	now := time.Now()
	if !lastNetworkTime.IsZero() && lastRxBytes > 0 {
		duration := now.Sub(lastNetworkTime).Seconds()
		if duration > 0 {
			stats.RxSpeed = uint64(float64(totalRx-lastRxBytes) / duration)
			stats.TxSpeed = uint64(float64(totalTx-lastTxBytes) / duration)
			stats.RxSpeedStr = formatBytes(stats.RxSpeed) + "/s"
			stats.TxSpeedStr = formatBytes(stats.TxSpeed) + "/s"
		}
	}
	lastRxBytes = totalRx
	lastTxBytes = totalTx
	lastNetworkTime = now

	return stats, nil
}

func getLoadStats() (LoadStats, error) {
	stats := LoadStats{}
	
	output, err := exec.Command("cat", "/proc/loadavg").Output()
	if err != nil {
		return stats, err
	}
	
	fields := strings.Fields(string(output))
	if len(fields) >= 3 {
		stats.Load1, _ = strconv.ParseFloat(fields[0], 64)
		stats.Load5, _ = strconv.ParseFloat(fields[1], 64)
		stats.Load15, _ = strconv.ParseFloat(fields[2], 64)
	}
	
	return stats, nil
}

func getUptime() string {
	duration := time.Since(startTime)
	days := int(duration.Hours() / 24)
	hours := int(duration.Hours()) % 24
	minutes := int(duration.Minutes()) % 60
	
	if days > 0 {
		return strconv.Itoa(days) + "天 " + strconv.Itoa(hours) + "小时 " + strconv.Itoa(minutes) + "分钟"
	}
	if hours > 0 {
		return strconv.Itoa(hours) + "小时 " + strconv.Itoa(minutes) + "分钟"
	}
	return strconv.Itoa(minutes) + "分钟"
}

func formatBytes(bytes uint64) string {
	const unit = 1024
	if bytes < unit {
		return strconv.FormatUint(bytes, 10) + " B"
	}
	div, exp := uint64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return strconv.FormatFloat(float64(bytes)/float64(div), 'f', 2, 64) + " " + "KMGTPE"[exp:exp+1] + "B"
}

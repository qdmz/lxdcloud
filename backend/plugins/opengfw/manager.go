package opengfw

import (
	"fmt"
	"lxdapi/models"
	"lxdapi/pkg/logger"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"syscall"
	"time"
)

type ProcessManager struct {
	binPath     string
	configPath  string
	rulesPath   string
	pidFile     string
	logFile     string
	cmd         *exec.Cmd
	startTime   time.Time
}

func NewProcessManager(workDir string) *ProcessManager {
	return &ProcessManager{
		binPath:    filepath.Join(workDir, "bin", getBinaryName()),
		configPath: filepath.Join(workDir, "config.yaml"),
		rulesPath:  filepath.Join(workDir, "rules.yaml"),
		pidFile:    filepath.Join(workDir, "opengfw.pid"),
		logFile:    filepath.Join(workDir, "opengfw.log"),
	}
}

func (m *ProcessManager) Start() error {
	if m.IsRunning() {
		logger.Warn("OpenGFW 进程已在运行，跳过启动")
		pid, _ := m.getPID()
		if pid > 0 {
			if process, err := os.FindProcess(pid); err == nil {
				m.cmd = &exec.Cmd{Process: process}
				m.startTime = time.Now()
			}
		}
		return nil
	}
	
	if _, err := os.Stat(m.binPath); os.IsNotExist(err) {
		return fmt.Errorf("OpenGFW 二进制文件不存在: %s", m.binPath)
	}
	
	if _, err := os.Stat(m.configPath); os.IsNotExist(err) {
		return fmt.Errorf("配置文件不存在: %s", m.configPath)
	}
	
	if _, err := os.Stat(m.rulesPath); os.IsNotExist(err) {
		return fmt.Errorf("规则文件不存在: %s", m.rulesPath)
	}
	
	logFile, err := os.OpenFile(m.logFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return fmt.Errorf("打开日志文件失败: %v", err)
	}
	
	m.cmd = exec.Command(m.binPath, 
		"-c", m.configPath,
		m.rulesPath,
	)
	m.cmd.Stdout = logFile
	m.cmd.Stderr = logFile
	
	logger.Info("启动 OpenGFW: %s", m.binPath)
	if err := m.cmd.Start(); err != nil {
		logFile.Close()
		return fmt.Errorf("启动 OpenGFW 失败: %v", err)
	}
	
	if err := os.WriteFile(m.pidFile, []byte(fmt.Sprintf("%d", m.cmd.Process.Pid)), 0644); err != nil {
		logger.Warn("写入 PID 文件失败: %v", err)
	}
	
	m.startTime = time.Now()
	logger.OK("OpenGFW 已启动，PID: %d", m.cmd.Process.Pid)
	
	go func() {
		m.cmd.Wait()
		logFile.Close()
		logger.Warn("OpenGFW 进程已退出")
	}()
	
	return nil
}

func (m *ProcessManager) Stop() error {
	pid, err := m.getPID()
	if err != nil {
		return fmt.Errorf("获取 PID 失败: %v", err)
	}
	
	if pid == 0 {
		return fmt.Errorf("OpenGFW 未运行")
	}
	
	process, err := os.FindProcess(pid)
	if err != nil {
		return fmt.Errorf("查找进程失败: %v", err)
	}
	
	logger.Info("停止 OpenGFW 进程: PID %d", pid)
	
	if err := process.Signal(syscall.SIGTERM); err != nil {
		if err.Error() == "os: process already finished" {
			os.Remove(m.pidFile)
			return nil
		}
		return fmt.Errorf("发送信号失败: %v", err)
	}
	
	for i := 0; i < 50; i++ {
		if !m.IsRunning() {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}
	
	if m.IsRunning() {
		logger.Warn("进程未正常退出，强制 kill")
		process.Kill()
	}
	
	os.Remove(m.pidFile)
	logger.OK("OpenGFW 已停止")
	
	return nil
}

func (m *ProcessManager) Restart() error {
	if m.IsRunning() {
		if err := m.Stop(); err != nil {
			return fmt.Errorf("停止失败: %v", err)
		}
		time.Sleep(time.Second)
	}
	return m.Start()
}

func (m *ProcessManager) Reload() error {
	pid, err := m.getPID()
	if err != nil {
		return fmt.Errorf("获取 PID 失败: %v", err)
	}
	
	if pid == 0 {
		return fmt.Errorf("OpenGFW 未运行")
	}
	
	process, err := os.FindProcess(pid)
	if err != nil {
		return fmt.Errorf("查找进程失败: %v", err)
	}
	
	logger.Info("热重载 OpenGFW 规则: PID %d", pid)
	if err := process.Signal(syscall.SIGHUP); err != nil {
		return fmt.Errorf("发送 SIGHUP 信号失败: %v", err)
	}
	
	logger.OK("规则已热重载")
	return nil
}

func (m *ProcessManager) IsRunning() bool {
	pid, err := m.getPID()
	if err != nil || pid == 0 {
		return false
	}
	
	process, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	
	err = process.Signal(syscall.Signal(0))
	return err == nil
}

func (m *ProcessManager) GetStatus() (*models.FirewallStats, error) {
	isRunning := m.IsRunning()
	stats := &models.FirewallStats{
		Running: isRunning,
	}
	
	if isRunning && !m.startTime.IsZero() {
		stats.Uptime = time.Since(m.startTime).Round(time.Second).String()
	} else if isRunning {
		stats.Uptime = "未知"
	}
	
	return stats, nil
}

func (m *ProcessManager) getPID() (int, error) {
	if _, err := os.Stat(m.pidFile); os.IsNotExist(err) {
		return 0, nil
	}
	
	data, err := os.ReadFile(m.pidFile)
	if err != nil {
		return 0, err
	}
	
	var pid int
	_, err = fmt.Sscanf(string(data), "%d", &pid)
	if err != nil {
		return 0, err
	}
	
	return pid, nil
}

func (m *ProcessManager) GetLogs(lines int) (string, error) {
	if _, err := os.Stat(m.logFile); os.IsNotExist(err) {
		return "", nil
	}
	
	cmd := exec.Command("tail", "-n", fmt.Sprintf("%d", lines), m.logFile)
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("读取日志失败: %v", err)
	}
	return string(output), nil
}

func (m *ProcessManager) ClearLogs() error {
	if _, err := os.Stat(m.logFile); os.IsNotExist(err) {
		return nil
	}
	
	if err := os.Truncate(m.logFile, 0); err != nil {
		return fmt.Errorf("清空日志文件失败: %v", err)
	}
	
	logger.OK("日志文件已清空: %s", m.logFile)
	return nil
}

func getBinaryName() string {
	arch := runtime.GOARCH
	switch arch {
	case "amd64":
		return "OpenGFW-linux-amd64"
	case "arm64":
		return "OpenGFW-linux-arm64"
	default:
		return "OpenGFW-linux-amd64"
	}
}

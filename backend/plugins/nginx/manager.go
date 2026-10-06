package nginx

import (
	"fmt"
	"lxdapi/pkg/logger"
	"os/exec"
	"strings"
)

type NginxManager struct{}

func NewNginxManager() *NginxManager {
	return &NginxManager{}
}

func (m *NginxManager) CheckInstalled() error {
	cmd := exec.Command("nginx", "-v")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("nginx未安装或不可用")
	}
	return nil
}

func (m *NginxManager) GetVersion() (string, error) {
	cmd := exec.Command("nginx", "-v")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", err
	}
	
	version := strings.TrimSpace(string(output))
	if strings.Contains(version, "nginx/") {
		parts := strings.Split(version, "nginx/")
		if len(parts) > 1 {
			return strings.Fields(parts[1])[0], nil
		}
	}
	
	return version, nil
}

func (m *NginxManager) TestConfig() error {
	cmd := exec.Command("nginx", "-t")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("配置测试失败: %s", string(output))
	}
	
	logger.Info("Nginx配置测试通过")
	return nil
}

func (m *NginxManager) Reload() error {
	cmd := exec.Command("systemctl", "reload", "nginx")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("重载失败: %s", string(output))
	}
	
	logger.Info("Nginx已重载")
	return nil
}

func (m *NginxManager) Start() error {
	cmd := exec.Command("systemctl", "start", "nginx")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("启动失败: %s", string(output))
	}
	
	logger.OK("Nginx服务已启动")
	return nil
}

func (m *NginxManager) Stop() error {
	cmd := exec.Command("systemctl", "stop", "nginx")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("停止失败: %s", string(output))
	}
	
	logger.OK("Nginx服务已停止")
	return nil
}

func (m *NginxManager) Restart() error {
	cmd := exec.Command("systemctl", "restart", "nginx")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("重启失败: %s", string(output))
	}
	
	logger.OK("Nginx服务已重启")
	return nil
}

func (m *NginxManager) IsRunning() bool {
	cmd := exec.Command("systemctl", "is-active", "nginx")
	output, err := cmd.Output()
	if err != nil {
		return false
	}
	
	return strings.TrimSpace(string(output)) == "active"
}

func (m *NginxManager) GetStatus() (map[string]interface{}, error) {
	status := make(map[string]interface{})
	
	status["running"] = m.IsRunning()
	
	version, err := m.GetVersion()
	if err == nil {
		status["version"] = version
	}
	
	cmd := exec.Command("systemctl", "show", "nginx", "--property=ActiveEnterTimestamp", "--value")
	output, err := cmd.Output()
	if err == nil {
		status["started_at"] = strings.TrimSpace(string(output))
	}
	
	return status, nil
}

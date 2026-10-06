package opengfw

import (
	"fmt"
	"lxdapi/pkg/logger"
	"os/exec"
	"strings"
)

type NFTablesManager struct {
	queueNum int
}

func NewNFTablesManager(queueNum int) *NFTablesManager {
	return &NFTablesManager{queueNum: queueNum}
}

func (m *NFTablesManager) Setup() error {
	// 确保 lxdfilter 表和 forward 链存在
	exec.Command("nft", "add", "table", "inet", "lxdfilter").Run()
	exec.Command("nft", "add", "chain", "inet", "lxdfilter", "forward", "{", "type", "filter", "hook", "forward", "priority", "0", ";", "}").Run()

	if m.ruleExists() {
		logger.Info("nftables NFQueue 规则已存在，跳过添加")
		return nil
	}
	
	logger.Info("添加 nftables NFQueue 规则: 队列号 %d", m.queueNum)
	
	// 入站流量拦截
	cmdIn := fmt.Sprintf("nft add rule inet lxdfilter forward iifname \"lxdbr0\" queue num %d bypass", m.queueNum)
	// 出站流量拦截
	cmdOut := fmt.Sprintf("nft add rule inet lxdfilter forward oifname \"lxdbr0\" queue num %d bypass", m.queueNum)
	
	if err := m.execCommand(cmdIn); err != nil {
		return fmt.Errorf("添加 nftables 规则失败: %v", err)
	}
	if err := m.execCommand(cmdOut); err != nil {
		// 尝试回滚
		handle := m.findHandle("iifname \"lxdbr0\"")
		if handle != "" {
			exec.Command("nft", "delete", "rule", "inet", "lxdfilter", "forward", "handle", handle).Run()
		}
		return fmt.Errorf("添加 nftables 规则失败: %v", err)
	}
	
	logger.OK("nftables NFQueue 规则已添加")
	// 持久化
	exec.Command("sh", "-c", "nft list ruleset > /etc/nftables.conf").Run()
	return nil
}

func (m *NFTablesManager) Remove() error {
	logger.Info("删除 nftables NFQueue 规则: 队列号 %d", m.queueNum)
	
	for {
		handle := m.findHandle(fmt.Sprintf("queue num %d", m.queueNum))
		if handle == "" {
			break
		}
		exec.Command("nft", "delete", "rule", "inet", "lxdfilter", "forward", "handle", handle).Run()
	}
	
	logger.OK("nftables NFQueue 规则已删除")
	exec.Command("sh", "-c", "nft list ruleset > /etc/nftables.conf").Run()
	return nil
}

func (m *NFTablesManager) ruleExists() bool {
	handle := m.findHandle(fmt.Sprintf("queue num %d", m.queueNum))
	return handle != ""
}

func (m *NFTablesManager) findHandle(match string) string {
	cmd := exec.Command("nft", "-a", "list", "chain", "inet", "lxdfilter", "forward")
	output, err := cmd.Output()
	if err != nil {
		return ""
	}
	
	lines := strings.Split(string(output), "\n")
	for _, line := range lines {
		if strings.Contains(line, match) {
			parts := strings.Fields(line)
			for i, p := range parts {
				if p == "handle" && i+1 < len(parts) {
					return parts[i+1]
				}
			}
		}
	}
	return ""
}

func (m *NFTablesManager) CheckNFQueueSupport() error {
	cmd := exec.Command("nft", "-v")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("nftables 不可用: %v", err)
	}
	
	cmd = exec.Command("lsmod")
	output, err := cmd.Output()
	if err != nil {
		logger.Warn("无法检查内核模块: %v", err)
		return nil
	}
	
	if !strings.Contains(string(output), "nfnetlink_queue") {
		logger.Warn("nfnetlink_queue 内核模块未加载，尝试加载...")
		cmd = exec.Command("modprobe", "nfnetlink_queue")
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("加载 nfnetlink_queue 模块失败: %v", err)
		}
		logger.OK("nfnetlink_queue 模块已加载")
	}
	
	return nil
}

func (m *NFTablesManager) execCommand(cmd string) error {
	out, err := exec.Command("sh", "-c", cmd).CombinedOutput()
	if err != nil {
		logger.Error("执行命令失败: %s, 错误: %s", cmd, string(out))
		return fmt.Errorf("%s", string(out))
	}
	return nil
}

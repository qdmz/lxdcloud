package ipv4

import (
	"fmt"
	"lxdapi/pkg/logger"
	"os/exec"
	"strings"
)

func (m *Manager) checkPortDNATExists(publicIP string, publicPort int, containerIP string, containerPort int, protocol, iface string) bool {
	groups := m.buildPortDNATMatchGroups(publicIP, publicPort, publicPort, containerIP, containerPort, containerPort, protocol, iface, true)
	handles := m.findNftHandlesByGroups("lxdnat", "prerouting", groups)
	return len(handles) > 0
}

func (m *Manager) checkDNATExists(publicIP, containerIP, iface string) bool {
	handle := m.findNftHandle("lxdip", "prerouting", publicIP, containerIP)
	return handle != ""
}

func (m *Manager) addPortDNAT(publicIP string, publicPort, publicPortEnd int, containerIP string, containerPort, containerPortEnd int, protocol, iface string) error {
	var portRange, containerPortRange string
	if publicPortEnd > 0 && publicPortEnd != publicPort {
		portRange = fmt.Sprintf("%d-%d", publicPort, publicPortEnd)
		containerPortRange = fmt.Sprintf("%s:%d-%d", containerIP, containerPort, containerPortEnd)
	} else {
		portRange = fmt.Sprintf("%d", publicPort)
		containerPortRange = fmt.Sprintf("%s:%d", containerIP, containerPort)
	}

	protos := []string{}
	if protocol == "both" {
		protos = []string{"tcp", "udp"}
	} else {
		protos = []string{protocol}
	}

	for _, proto := range protos {
		args := []string{"add", "rule", "inet", "lxdnat", "prerouting"}
		if publicIP != "" {
			args = append(args, "ip", "daddr", publicIP)
		}
		if iface != "" {
			args = append(args, "iifname", iface)
		}
		args = append(args, proto, "dport", portRange, "dnat", "to", containerPortRange)

		cmd := exec.Command("nft", args...)
		if output, err := cmd.CombinedOutput(); err != nil {
			return fmt.Errorf("nft添加DNAT规则失败: %v, output: %s", err, string(output))
		}
		logger.Info("nft添加端口转发: %s:%s(%s) -> %s", publicIP, portRange, proto, containerPortRange)
	}

	m.saveNftRules()
	return nil
}

func (m *Manager) removePortDNAT(publicIP string, publicPort, publicPortEnd int, containerIP string, containerPort, containerPortEnd int, protocol, iface string) error {
	protos := []string{}
	if protocol == "both" {
		protos = []string{"tcp", "udp"}
	} else {
		protos = []string{protocol}
	}

	var deleted int
	for _, proto := range protos {
		protoDeleted := 0
		groups := m.buildPortDNATMatchGroups(publicIP, publicPort, publicPortEnd, "", 0, 0, proto, iface, false)
		for {
			handles := m.findNftHandlesByGroups("lxdnat", "prerouting", groups)
			if len(handles) == 0 {
				break
			}
			for _, handle := range handles {
				cmd := exec.Command("nft", "delete", "rule", "inet", "lxdnat", "prerouting", "handle", handle)
				if err := cmd.Run(); err != nil {
					logger.Error("nft删除端口转发失败: %v", err)
					return err
				}
				logger.Info("nft删除端口转发: %s %s handle %s", publicIP, proto, handle)
				deleted++
				protoDeleted++
			}
		}
		if protoDeleted == 0 && containerIP != "" {
			groups := m.buildPortDNATMatchGroups(publicIP, publicPort, publicPortEnd, containerIP, containerPort, containerPortEnd, proto, iface, true)
			handles := m.findNftHandlesByGroups("lxdnat", "prerouting", groups)
			for _, handle := range handles {
				cmd := exec.Command("nft", "delete", "rule", "inet", "lxdnat", "prerouting", "handle", handle)
				if err := cmd.Run(); err != nil {
					logger.Error("nft删除端口转发失败: %v", err)
					return err
				}
				logger.Info("nft删除端口转发: %s %s handle %s", publicIP, proto, handle)
				deleted++
				protoDeleted++
			}
		}
	}

	m.saveNftRules()
	if deleted == 0 && len(protos) > 0 {
		return fmt.Errorf("nft删除端口转发失败: 未找到匹配规则")
	}
	return nil
}

func (m *Manager) buildPortDNATMatchGroups(publicIP string, publicPort, publicPortEnd int, containerIP string, containerPort, containerPortEnd int, protocol, iface string, includeTarget bool) [][]string {
	portRange := fmt.Sprintf("dport %d", publicPort)
	if publicPortEnd > 0 && publicPortEnd != publicPort {
		portRange = fmt.Sprintf("dport %d-%d", publicPort, publicPortEnd)
	}

	base := []string{}
	if publicIP != "" {
		base = append(base, publicIP)
	}
	if protocol != "" {
		base = append(base, protocol)
	}
	if publicPort > 0 {
		base = append(base, portRange)
	}

	if includeTarget && containerIP != "" {
		base = append(base, containerIP)
		if containerPort > 0 {
			if containerPortEnd > 0 && containerPortEnd != containerPort {
				base = append(base, fmt.Sprintf(":%d-%d", containerPort, containerPortEnd))
			} else {
				base = append(base, fmt.Sprintf(":%d", containerPort))
			}
		}
	}

	if iface == "" {
		return [][]string{base}
	}

	return [][]string{
		append(append([]string{}, base...), fmt.Sprintf("iifname %s", iface)),
		append(append([]string{}, base...), fmt.Sprintf("iifname \"%s\"", iface)),
	}
}

func (m *Manager) findNftHandlesByGroups(table, chain string, groups [][]string) []string {
	cmd := exec.Command("nft", "-a", "list", "chain", "inet", table, chain)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil
	}

	lines := strings.Split(string(output), "\n")
	seen := map[string]bool{}
	var handles []string
	for _, line := range lines {
		for _, matches := range groups {
			allMatch := true
			for _, match := range matches {
				if match != "" && !strings.Contains(line, match) {
					allMatch = false
					break
				}
			}
			if !allMatch {
				continue
			}
			parts := strings.Fields(line)
			for i, part := range parts {
				if part == "handle" && i+1 < len(parts) && !seen[parts[i+1]] {
					seen[parts[i+1]] = true
					handles = append(handles, parts[i+1])
				}
			}
		}
	}
	return handles
}

func (m *Manager) deleteNftHandles(table, chain string, groups [][]string) int {
	deleted := 0
	for {
		handles := m.findNftHandlesByGroups(table, chain, groups)
		if len(handles) == 0 {
			break
		}
		for _, handle := range handles {
			cmd := exec.Command("nft", "delete", "rule", "inet", table, chain, "handle", handle)
			if err := cmd.Run(); err != nil {
				logger.Error("nft删除规则失败: %v", err)
			} else {
				deleted++
			}
		}
	}
	return deleted
}

func (m *Manager) addSNAT(publicIP, containerIP, iface string) error {
	cmd := exec.Command("nft", "add", "rule", "inet", "lxdip", "postrouting",
		"ip", "saddr", containerIP, "oifname", iface, "snat", "to", publicIP)
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("nft添加SNAT规则失败: %v, output: %s", err, string(output))
	}

	logger.Info("nft添加SNAT规则: %s -> %s", containerIP, publicIP)
	m.saveNftRules()
	return nil
}

func (m *Manager) addDNAT(publicIP, containerIP, iface string) error {
	cmd := exec.Command("nft", "add", "rule", "inet", "lxdip", "prerouting",
		"ip", "daddr", publicIP, "dnat", "to", containerIP)
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("nft添加DNAT规则失败: %v, output: %s", err, string(output))
	}

	logger.Info("nft添加DNAT规则: %s -> %s", publicIP, containerIP)
	m.saveNftRules()
	return nil
}

func (m *Manager) removeSNAT(publicIP, containerIP, iface string) error {
	handle := m.findNftHandle("lxdip", "postrouting", containerIP, publicIP)
	if handle != "" {
		exec.Command("nft", "delete", "rule", "inet", "lxdip", "postrouting", "handle", handle).Run()
		logger.Info("nft删除SNAT规则: %s -> %s handle %s", containerIP, publicIP, handle)
	}
	m.saveNftRules()
	return nil
}

func (m *Manager) removeDNAT(publicIP, containerIP, iface string) error {
	handle := m.findNftHandle("lxdip", "prerouting", publicIP, containerIP)
	if handle != "" {
		exec.Command("nft", "delete", "rule", "inet", "lxdip", "prerouting", "handle", handle).Run()
		logger.Info("nft删除DNAT规则: %s -> %s handle %s", publicIP, containerIP, handle)
	}
	m.saveNftRules()
	return nil
}

func (m *Manager) findNftHandle(table, chain string, matches ...string) string {
	cmd := exec.Command("nft", "-a", "list", "chain", "inet", table, chain)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return ""
	}

	lines := strings.Split(string(output), "\n")
	for _, line := range lines {
		allMatch := true
		for _, m := range matches {
			if !strings.Contains(line, m) {
				allMatch = false
				break
			}
		}
		if allMatch {
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

func (m *Manager) removeAllRules(publicIP string) error {
	for _, chain := range []string{"prerouting", "postrouting"} {
		for {
			handle := m.findNftHandle("lxdnat", chain, publicIP)
			if handle == "" {
				break
			}
			exec.Command("nft", "delete", "rule", "inet", "lxdnat", chain, "handle", handle).Run()
		}
	}

	for _, chain := range []string{"prerouting", "postrouting"} {
		for {
			handle := m.findNftHandle("lxdip", chain, publicIP)
			if handle == "" {
				break
			}
			exec.Command("nft", "delete", "rule", "inet", "lxdip", chain, "handle", handle).Run()
		}
	}

	logger.Info("nft已清除 %s 的所有相关规则", publicIP)
	m.saveNftRules()
	return nil
}

func (m *Manager) saveNftRules() {
	cmd := exec.Command("sh", "-c", "nft list ruleset > /etc/nftables.conf")
	if err := cmd.Run(); err != nil {
		logger.Warn("导出nftables规则失败: %v", err)
	}
}

func (m *Manager) addIPToInterface(ip, iface string) error {
	checkCmd := exec.Command("ip", "addr", "show", "dev", iface)
	output, err := checkCmd.CombinedOutput()
	if err == nil && strings.Contains(string(output), ip) {
		logger.Info("IP %s 已存在于网卡 %s", ip, iface)
		return nil
	}
	
	cmd := exec.Command("ip", "addr", "add", fmt.Sprintf("%s/32", ip), "dev", iface, "noprefixroute", "preferred_lft", "0")
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("添加IP到网卡失败: %v, output: %s", err, string(output))
	}
	
	logger.OK("IP %s 已添加到网卡 %s (noprefixroute, preferred_lft 0)", ip, iface)
	return nil
}

func (m *Manager) removeIPFromInterface(ip, iface string) error {
	cmd := exec.Command("ip", "addr", "del", fmt.Sprintf("%s/32", ip), "dev", iface)
	if output, err := cmd.CombinedOutput(); err != nil {
		logger.Warn("从网卡删除IP失败: %v, output: %s", err, string(output))
		return err
	}
	
	logger.Info("IP %s 已从网卡 %s 删除", ip, iface)
	return nil
}

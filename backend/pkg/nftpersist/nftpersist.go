// Package nftpersist 只持久化 lxdcloud 自有的 nftables 表（v1.1.17）。
//
// 旧实现执行 `nft list ruleset > /etc/nftables.conf`，会把 LXD、Docker、fail2ban 等其他程序的表
// 一并覆盖写进系统配置，开机由 nftables.service 加载时可能与这些程序自己创建的规则冲突。
// 现在只导出本程序创建的表（lxdnat、lxdip、lxdfilter）到单独文件，不再改动 /etc/nftables.conf。
// 这些规则在 lxdapi 启动时本来就会从数据库恢复，该文件仅作备份/手动恢复用：
//
//	nft -f /etc/lxdcloud/nftables-lxdcloud.nft
package nftpersist

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"lxdapi/pkg/logger"
)

// OwnedTables 本程序创建并管理的 inet 表
var OwnedTables = []string{"lxdnat", "lxdip", "lxdfilter"}

// DefaultPath 导出文件路径（可用环境变量 LXDCLOUD_NFT_SAVE_PATH 覆盖）
const DefaultPath = "/etc/lxdcloud/nftables-lxdcloud.nft"

var mu sync.Mutex

// Path 实际导出路径
func Path() string {
	if p := strings.TrimSpace(os.Getenv("LXDCLOUD_NFT_SAVE_PATH")); p != "" {
		return p
	}
	return DefaultPath
}

// Render 生成自有表的可恢复脚本（每张表先 add+flush 再整体定义，可重复 nft -f）
func Render() (string, int) {
	var buf bytes.Buffer
	fmt.Fprintf(&buf, "#!/usr/sbin/nft -f\n# lxdcloud 自有 nftables 表备份，生成于 %s\n# 恢复: nft -f %s\n\n", time.Now().Format("2006-01-02 15:04:05"), Path())
	n := 0
	for _, t := range OwnedTables {
		out, err := exec.Command("nft", "list", "table", "inet", t).Output()
		if err != nil || len(bytes.TrimSpace(out)) == 0 {
			continue
		}
		fmt.Fprintf(&buf, "add table inet %s\nflush table inet %s\n", t, t)
		buf.Write(out)
		if !bytes.HasSuffix(out, []byte("\n")) {
			buf.WriteByte('\n')
		}
		buf.WriteByte('\n')
		n++
	}
	return buf.String(), n
}

// Save 导出自有表到单独文件（原子写入）；没有任何自有表时不写
func Save() {
	mu.Lock()
	defer mu.Unlock()
	content, n := Render()
	if n == 0 {
		return
	}
	p := Path()
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		logger.Warn("导出nftables规则失败: %v", err)
		return
	}
	tmp := p + ".tmp"
	if err := os.WriteFile(tmp, []byte(content), 0o600); err != nil {
		logger.Warn("导出nftables规则失败: %v", err)
		return
	}
	if err := os.Rename(tmp, p); err != nil {
		logger.Warn("导出nftables规则失败: %v", err)
	}
}

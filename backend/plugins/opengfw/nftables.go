package opengfw

import (
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"

	"lxdapi/pkg/logger"
	"lxdapi/pkg/nftpersist"
)

type NFTablesManager struct {
	queueNum int
}

func NewNFTablesManager(queueNum int) *NFTablesManager {
	return &NFTablesManager{queueNum: queueNum}
}

// nftRun 执行 nft 命令（测试时替换）
var nftRun = func(args ...string) ([]byte, error) {
	return exec.Command("nft", args...).CombinedOutput()
}

// persistSave 导出自有表备份（测试时替换）
var persistSave = nftpersist.Save

// RuleComment 返回本插件 NFQUEUE 规则的 comment 标记（dir 为 in/out）
func RuleComment(queueNum int, dir string) string {
	return fmt.Sprintf("lxdcloud-opengfw-q%d-%s", queueNum, dir)
}

// queueRule chain 中解析出的一条属于本插件的 NFQUEUE 规则
type queueRule struct {
	Handle string
	Dir    string // in（iifname lxdbr0）/ out（oifname lxdbr0）
	Tagged bool   // 带 lxdcloud comment 标记
}

// queueNumRe 兼容 nft 的多种显示格式：
//
//	queue num 100 bypass / queue to 100 / queue flags bypass to 100 / queue flags bypass,fanout to 100-101
var queueNumRe = regexp.MustCompile(`\bqueue\b(?:\s+flags\s+\S+)?\s+(?:num|to)\s+(\d+)\b`)
var commentRe = regexp.MustCompile(`comment\s+"([^"]*)"`)
var handleRe = regexp.MustCompile(`#\s*handle\s+(\d+)\s*$`)

// parseQueueRules 从 `nft -a list chain inet lxdfilter forward` 的输出中找出本插件的规则（按出现顺序）
func parseQueueRules(listing string, queueNum int) []queueRule {
	var rules []queueRule
	for _, line := range strings.Split(listing, "\n") {
		line = strings.TrimSpace(line)
		hm := handleRe.FindStringSubmatch(line)
		if hm == nil || strings.HasPrefix(line, "chain ") || strings.HasPrefix(line, "table ") {
			continue
		}
		r := queueRule{Handle: hm[1]}
		if cm := commentRe.FindStringSubmatch(line); cm != nil {
			switch cm[1] {
			case RuleComment(queueNum, "in"):
				r.Dir, r.Tagged = "in", true
			case RuleComment(queueNum, "out"):
				r.Dir, r.Tagged = "out", true
			}
		}
		if r.Dir == "" {
			qm := queueNumRe.FindStringSubmatch(line)
			if qm == nil || qm[1] != strconv.Itoa(queueNum) {
				continue
			}
			switch {
			case strings.Contains(line, `iifname "lxdbr0"`):
				r.Dir = "in"
			case strings.Contains(line, `oifname "lxdbr0"`):
				r.Dir = "out"
			default:
				continue
			}
		}
		rules = append(rules, r)
	}
	return rules
}

// planDedup 每个方向保留一条（优先带标记的），返回要保留的规则与要删除的 handle
func planDedup(rules []queueRule) (keep map[string]queueRule, del []string) {
	keep = map[string]queueRule{}
	for _, r := range rules {
		if k, ok := keep[r.Dir]; !ok || (!k.Tagged && r.Tagged) {
			if ok {
				del = append(del, k.Handle)
			}
			keep[r.Dir] = r
			continue
		}
		del = append(del, r.Handle)
	}
	return keep, del
}

func (m *NFTablesManager) listChain() (string, error) {
	out, err := nftRun("-a", "list", "chain", "inet", "lxdfilter", "forward")
	return string(out), err
}

func (m *NFTablesManager) addRule(dir string) error {
	ifk := "iifname"
	if dir == "out" {
		ifk = "oifname"
	}
	q := strconv.Itoa(m.queueNum)
	out, err := nftRun("add", "rule", "inet", "lxdfilter", "forward", ifk, `"lxdbr0"`, "queue", "num", q, "bypass", "comment", `"`+RuleComment(m.queueNum, dir)+`"`)
	if err != nil {
		// 极老的 nft 不支持规则 comment，退回无标记写法（解析时按队列号识别）
		logger.Warn("添加带 comment 的 NFQueue 规则失败（%s），改用无 comment 写法", strings.TrimSpace(string(out)))
		out, err = nftRun("add", "rule", "inet", "lxdfilter", "forward", ifk, `"lxdbr0"`, "queue", "num", q, "bypass")
	}
	if err != nil {
		return fmt.Errorf("%s", strings.TrimSpace(string(out)))
	}
	return nil
}

// Setup 确保 inet lxdfilter forward 中恰好有一对（入/出各一条）本插件的 NFQUEUE 规则，多余的重复规则会被删除
func (m *NFTablesManager) Setup() error {
	// 确保 lxdfilter 表和 forward 链存在
	nftRun("add", "table", "inet", "lxdfilter")
	nftRun("add", "chain", "inet", "lxdfilter", "forward", "{", "type", "filter", "hook", "forward", "priority", "0", ";", "}")

	listing, err := m.listChain()
	if err != nil {
		return fmt.Errorf("读取 nftables 规则失败: %v %s", err, strings.TrimSpace(listing))
	}
	keep, del := planDedup(parseQueueRules(listing, m.queueNum))
	changed := false
	for _, h := range del {
		if out, err := nftRun("delete", "rule", "inet", "lxdfilter", "forward", "handle", h); err != nil {
			logger.Warn("删除重复的 NFQueue 规则 handle %s 失败: %s", h, strings.TrimSpace(string(out)))
		} else {
			changed = true
		}
	}
	if len(del) > 0 {
		logger.Info("已清理 %d 条重复的 nftables NFQueue 规则（队列号 %d）", len(del), m.queueNum)
	}
	var added []string
	for _, dir := range []string{"in", "out"} {
		if _, ok := keep[dir]; ok {
			continue
		}
		if err := m.addRule(dir); err != nil {
			for _, d := range added { // 回滚本次新增的规则
				m.removeDir(d)
			}
			if changed {
				persistSave()
			}
			return fmt.Errorf("添加 nftables 规则失败: %v", err)
		}
		added = append(added, dir)
		changed = true
	}
	if len(added) == 0 {
		logger.Info("nftables NFQueue 规则已存在（队列号 %d），跳过添加", m.queueNum)
	} else {
		logger.OK("nftables NFQueue 规则已添加: 队列号 %d（%s）", m.queueNum, strings.Join(added, ","))
	}
	if changed {
		persistSave()
	}
	return nil
}

func (m *NFTablesManager) removeDir(dir string) {
	listing, err := m.listChain()
	if err != nil {
		return
	}
	for _, r := range parseQueueRules(listing, m.queueNum) {
		if r.Dir == dir {
			nftRun("delete", "rule", "inet", "lxdfilter", "forward", "handle", r.Handle)
		}
	}
}

// Remove 删除本插件的全部 NFQUEUE 规则（含历史上重复添加的）
func (m *NFTablesManager) Remove() error {
	logger.Info("删除 nftables NFQueue 规则: 队列号 %d", m.queueNum)
	listing, err := m.listChain()
	if err != nil {
		return nil // 表/链不存在即无规则
	}
	n := 0
	for _, r := range parseQueueRules(listing, m.queueNum) {
		if out, err := nftRun("delete", "rule", "inet", "lxdfilter", "forward", "handle", r.Handle); err != nil {
			logger.Warn("删除 NFQueue 规则 handle %s 失败: %s", r.Handle, strings.TrimSpace(string(out)))
		} else {
			n++
		}
	}
	logger.OK("nftables NFQueue 规则已删除（%d 条）", n)
	persistSave()
	return nil
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

	if !strings.Contains(string(output), "nfnetlink_queue") && !nfqueueBuiltin() {
		logger.Warn("nfnetlink_queue 内核模块未加载，尝试加载...")
		cmd = exec.Command("modprobe", "nfnetlink_queue")
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("当前内核不支持 NFQUEUE（nfnetlink_queue 模块无法加载），防火墙无法运行：%v", err)
		}
		logger.OK("nfnetlink_queue 模块已加载")
	}

	return nil
}

// nfqueueBuiltin 判断 nfnetlink_queue 是否已编译进内核（lsmod 中不会出现）
func nfqueueBuiltin() bool {
	for _, p := range []string{"/sys/module/nfnetlink_queue", "/proc/net/netfilter/nfnetlink_queue"} {
		if _, err := os.Stat(p); err == nil {
			return true
		}
	}
	return false
}

package lxc

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"lxdapi/internal/core"
	"lxdapi/pkg/logger"
	"os/exec"
	"strings"
	"sync"
	"time"
)

// ErrLXDNotInstalled 在节点未安装 lxc 客户端时返回，调用方应直接把
// 它的文案呈现给用户，而不是透出底层的 exec 错误。
var ErrLXDNotInstalled = errors.New("当前节点未安装LXD，容器相关操作不可用")

var (
	lxcPath      string
	lxcCheckOnce sync.Once
)

// BinaryAvailable 报告 lxc 可执行文件是否存在（结果缓存）。
func BinaryAvailable() bool {
	lxcCheckOnce.Do(func() {
		if p, err := exec.LookPath("lxc"); err == nil {
			lxcPath = p
		}
	})
	return lxcPath != ""
}

type Client struct {
	socket  string
	timeout time.Duration
}

func NewClient() *Client {
	cfg := core.GlobalConfig.LXC
	return &Client{
		socket:  cfg.Socket,
		timeout: time.Duration(cfg.Timeout) * time.Second,
	}
}

func (c *Client) exec(ctx context.Context, args ...string) (string, error) {
	if !BinaryAvailable() {
		return "", ErrLXDNotInstalled
	}
	cmd := exec.CommandContext(ctx, "lxc", args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	
	logger.Info("执行LXC命令: lxc %s", strings.Join(args, " "))
	
	err := cmd.Run()
	if err != nil {
		errMsg := stderr.String()
		if errMsg == "" {
			errMsg = err.Error()
		}
		logger.Error("LXC命令执行失败: %s", errMsg)
		return "", fmt.Errorf("%s", errMsg)
	}
	
	return stdout.String(), nil
}

// execQuiet 与 exec 相同，但日志只记录 logLabel（用于命令行含密码等敏感参数的场景）
func (c *Client) execQuiet(ctx context.Context, logLabel string, args ...string) (string, error) {
	if !BinaryAvailable() {
		return "", ErrLXDNotInstalled
	}
	cmd := exec.CommandContext(ctx, "lxc", args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	logger.Info("执行LXC命令: lxc %s", logLabel)
	if err := cmd.Run(); err != nil {
		errMsg := stderr.String()
		if errMsg == "" {
			errMsg = err.Error()
		}
		logger.Error("LXC命令执行失败: %s", errMsg)
		return "", fmt.Errorf("%s", errMsg)
	}
	return stdout.String(), nil
}

// execOutput 执行 lxc 命令并同时返回 stdout 与 stderr（脚本执行时便于记录失败原因）；日志只记录 logLabel
func (c *Client) execOutput(ctx context.Context, logLabel string, args ...string) (string, string, error) {
	if !BinaryAvailable() {
		return "", "", ErrLXDNotInstalled
	}
	cmd := exec.CommandContext(ctx, "lxc", args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	logger.Info("执行LXC命令: lxc %s", logLabel)
	err := cmd.Run()
	return stdout.String(), stderr.String(), err
}

func (c *Client) execJSON(ctx context.Context, result interface{}, args ...string) error {
	output, err := c.exec(ctx, append(args, "--format=json")...)
	if err != nil {
		return err
	}
	return json.Unmarshal([]byte(output), result)
}


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

func (c *Client) execJSON(ctx context.Context, result interface{}, args ...string) error {
	output, err := c.exec(ctx, append(args, "--format=json")...)
	if err != nil {
		return err
	}
	return json.Unmarshal([]byte(output), result)
}


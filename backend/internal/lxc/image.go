package lxc

import (
	"context"
	"encoding/json"
	"fmt"
	"lxdapi/pkg/logger"
	"regexp"
	"strings"
)

type ImageInfo struct {
	Fingerprint  string                 `json:"fingerprint"`
	Aliases      []ImageAlias           `json:"aliases"`
	Architecture string                 `json:"architecture"`
	Properties   map[string]interface{} `json:"properties"`
	Public       bool                   `json:"public"`
	Size         int64                  `json:"size"`
	AutoUpdate   bool                   `json:"auto_update"`
	UploadedAt   string                 `json:"uploaded_at"`
	CreatedAt    string                 `json:"created_at"`
	Type         string                 `json:"type"`
	Cached       bool                   `json:"cached"`
	LastUsedAt   string                 `json:"last_used_at"`
}

type ImageAlias struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

func (c *Client) ListImages(ctx context.Context) ([]ImageInfo, error) {
	logger.Info("获取LXD镜像列表")
	
	cmd := []string{"image", "list", "--format", "json"}
	output, err := c.exec(ctx, cmd...)
	if err != nil {
		return nil, fmt.Errorf("获取镜像列表失败: %v", err)
	}
	
	var images []ImageInfo
	if err := json.Unmarshal([]byte(output), &images); err != nil {
		return nil, fmt.Errorf("解析镜像列表失败: %v", err)
	}
	
	logger.OK("获取到 %d 个镜像", len(images))
	return images, nil
}

func (c *Client) DeleteImage(ctx context.Context, fingerprint string) error {
	logger.Info("删除镜像: %s", fingerprint)
	
	cmd := []string{"image", "delete", fingerprint}
	if _, err := c.exec(ctx, cmd...); err != nil {
		return fmt.Errorf("删除镜像失败: %v", err)
	}
	
	logger.OK("镜像删除成功: %s", fingerprint)
	return nil
}

func (c *Client) GetImageInfo(ctx context.Context, fingerprint string) (*ImageInfo, error) {
	logger.Info("获取镜像详情: %s", fingerprint)
	
	cmd := []string{"image", "show", fingerprint, "--format", "json"}
	output, err := c.exec(ctx, cmd...)
	if err != nil {
		return nil, fmt.Errorf("获取镜像详情失败: %v", err)
	}
	
	var info ImageInfo
	if err := json.Unmarshal([]byte(output), &info); err != nil {
		return nil, fmt.Errorf("解析镜像详情失败: %v", err)
	}
	
	return &info, nil
}

// ---- v1.1.22 镜像管理 ----

// CreateImageAlias 为镜像创建别名
func (c *Client) CreateImageAlias(ctx context.Context, alias, fingerprint string) error {
	if _, err := c.exec(ctx, "image", "alias", "create", alias, fingerprint); err != nil {
		return fmt.Errorf("创建别名 %s 失败: %v", alias, err)
	}
	return nil
}

// DeleteImageAlias 删除别名（不删除镜像）
func (c *Client) DeleteImageAlias(ctx context.Context, alias string) error {
	if _, err := c.exec(ctx, "image", "alias", "delete", alias); err != nil {
		return fmt.Errorf("删除别名 %s 失败: %v", alias, err)
	}
	return nil
}

var importedFPRe = regexp.MustCompile(`(?i)fingerprint:\s*([0-9a-f]{64})`)

// ImportImageFiles lxc image import <meta> [rootfs]，返回指纹（统一格式 tar 只传 meta）
func (c *Client) ImportImageFiles(ctx context.Context, meta, rootfs string) (string, error) {
	args := []string{"image", "import", meta}
	if rootfs != "" {
		args = append(args, rootfs)
	}
	out, err := c.exec(ctx, args...)
	if err != nil {
		return "", fmt.Errorf("导入镜像失败: %v", err)
	}
	if m := importedFPRe.FindStringSubmatch(out); m != nil {
		return strings.ToLower(m[1]), nil
	}
	return "", fmt.Errorf("导入完成但未识别到镜像指纹: %s", strings.TrimSpace(out))
}

// Query 调用 lxc query（LXD REST API），返回原始响应（--raw：含 type/operation/metadata）
func (c *Client) Query(ctx context.Context, method, path, body string) (*QueryResponse, error) {
	args := []string{"query", "--raw", "-X", method}
	if body != "" {
		args = append(args, "--data", body)
	}
	args = append(args, path)
	out, err := c.execQuiet(ctx, "query -X "+method+" "+path, args...)
	if err != nil {
		return nil, err
	}
	var r QueryResponse
	if err := json.Unmarshal([]byte(out), &r); err != nil {
		return nil, fmt.Errorf("解析 LXD 响应失败: %v", err)
	}
	return &r, nil
}

// QueryResponse LXD REST 响应
type QueryResponse struct {
	Type       string          `json:"type"`
	Status     string          `json:"status"`
	StatusCode int             `json:"status_code"`
	Operation  string          `json:"operation"`
	Error      string          `json:"error"`
	Metadata   json.RawMessage `json:"metadata"`
}

// Operation LXD 后台操作
type Operation struct {
	ID         string                 `json:"id"`
	Status     string                 `json:"status"`
	StatusCode int                    `json:"status_code"`
	Metadata   map[string]interface{} `json:"metadata"`
	Err        string                 `json:"err"`
}

// OperationID 从异步响应中取操作 ID
func (r *QueryResponse) OperationID() string {
	if r.Operation != "" {
		p := r.Operation
		if i := strings.Index(p, "?"); i >= 0 {
			p = p[:i]
		}
		return p[strings.LastIndex(p, "/")+1:]
	}
	var op Operation
	if json.Unmarshal(r.Metadata, &op) == nil {
		return op.ID
	}
	return ""
}

var opIDRe = regexp.MustCompile(`^[0-9a-fA-F-]{8,64}$`)

// GetOperation 查询操作状态
func (c *Client) GetOperation(ctx context.Context, id string) (*Operation, error) {
	if !opIDRe.MatchString(id) {
		return nil, fmt.Errorf("操作 ID 无效")
	}
	r, err := c.Query(ctx, "GET", "/1.0/operations/"+id, "")
	if err != nil {
		return nil, err
	}
	var op Operation
	if err := json.Unmarshal(r.Metadata, &op); err != nil {
		return nil, fmt.Errorf("解析操作状态失败: %v", err)
	}
	return &op, nil
}

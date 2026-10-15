package service

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"lxdapi/models"
	"lxdapi/pkg/logger"
)

// NodeClient 子节点客户端：通过 /api/system（api_hash 鉴权）代理执行 LXD 操作
type NodeClient struct {
	BaseURL string
	APIHash string
	client  *http.Client
}

func NewNodeClient(node *models.Node) *NodeClient {
	return NewNodeClientTimeout(node, 120*time.Second)
}

// NewNodeClientTimeout 指定超时的节点客户端（健康检查等短请求用）
func NewNodeClientTimeout(node *models.Node, timeout time.Duration) *NodeClient {
	base := strings.TrimRight(node.APIBaseURL, "/")
	// 自签证书场景：跳过 TLS 证书校验（节点间为内网 https 直连）
	tr := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}
	return &NodeClient{
		BaseURL: base,
		APIHash: node.APIHash,
		client:  &http.Client{Timeout: timeout, Transport: tr},
	}
}

// do 发起请求并解包 response.Response{code,msg,data}
func (nc *NodeClient) do(method, path string, query map[string]string, body interface{}) (map[string]interface{}, error) {
	url := nc.BaseURL + path
	if len(query) > 0 {
		url += "?" + encodeQuery(query)
	}

	var reqBody io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return nil, err
		}
		reqBody = bytes.NewReader(b)
	}

	req, err := http.NewRequest(method, url, reqBody)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-API-Hash", nc.APIHash)

	resp, err := nc.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("节点连接失败: %v", err)
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var envelope struct {
		Code int             `json:"code"`
		Msg  string          `json:"msg"`
		Data json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal(data, &envelope); err != nil {
		return nil, fmt.Errorf("节点响应解析失败: %v", err)
	}
	if envelope.Code != 200 {
		return nil, fmt.Errorf("节点返回错误: %s", envelope.Msg)
	}
	// data 可能是对象、字符串（如「操作成功」）或数组：只有对象才解包，其余视为成功
	out := map[string]interface{}{}
	if len(envelope.Data) > 0 && envelope.Data[0] == '{' {
		if err := json.Unmarshal(envelope.Data, &out); err != nil {
			return nil, fmt.Errorf("节点响应解析失败: %v", err)
		}
	}
	return out, nil
}

func encodeQuery(q map[string]string) string {
	var sb strings.Builder
	for k, v := range q {
		if sb.Len() > 0 {
			sb.WriteString("&")
		}
		sb.WriteString(k + "=" + v)
	}
	return sb.String()
}

// Ping 节点连通性测试
func (nc *NodeClient) Ping() error {
	// 任务列表接口 data 为数组，不能用 map 解包，单独解析 code
	if err := nc.pingPath("/api/system/tasks", map[string]string{"limit": "1"}); err == nil {
		return nil
	}
	// 部分节点旧版本可能无 tasks 路由，用容器列表兜底
	return nc.pingPath("/api/system/containers", map[string]string{"limit": "1"})
}

// pingPath 对指定路径发起 GET 并只校验 code=200
func (nc *NodeClient) pingPath(path string, query map[string]string) error {
	url := nc.BaseURL + path
	if len(query) > 0 {
		url += "?" + encodeQuery(query)
	}
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return err
	}
	req.Header.Set("X-API-Hash", nc.APIHash)
	resp, err := nc.client.Do(req)
	if err != nil {
		return fmt.Errorf("节点连接失败: %v", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	var env struct {
		Code int    `json:"code"`
		Msg  string `json:"msg"`
	}
	if err := json.Unmarshal(body, &env); err != nil {
		return fmt.Errorf("节点响应解析失败: %v", err)
	}
	if env.Code != 200 {
		return fmt.Errorf("节点返回错误: %s", env.Msg)
	}
	return nil
}

// CreateContainer 创建容器
func (nc *NodeClient) CreateContainer(name, image, password string, cpu, memMB, diskMB int) error {
	_, err := nc.do("POST", "/api/system/containers", nil, map[string]interface{}{
		"name":     name,
		"image":    image,
		"password": password,
		"cpu":      cpu,
		"memory":   memMB,
		"disk":     diskMB,
	})
	return err
}

// CreateContainerReq 以完整规格在节点创建容器，返回节点任务 ID（旧版节点可能为 0）
func (nc *NodeClient) CreateContainerReq(req *models.CreateContainerRequest) (uint, error) {
	data, err := nc.do("POST", "/api/system/containers", nil, req)
	if err != nil {
		return 0, err
	}
	if v, ok := data["task_id"].(float64); ok && v > 0 {
		return uint(v), nil
	}
	return 0, nil
}

// GetTask 查询节点任务
func (nc *NodeClient) GetTask(id uint) (map[string]interface{}, error) {
	return nc.do("GET", "/api/system/tasks/detail", map[string]string{"id": fmt.Sprintf("%d", id)}, nil)
}

// WaitTask 轮询节点任务直到 success/failed 或超时（超时视为失败，避免误报开通成功）
func (nc *NodeClient) WaitTask(id uint, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	interval := 2 * time.Second
	failures := 0
	for {
		t, err := nc.GetTask(id)
		if err != nil {
			failures++
			if failures >= 5 {
				return fmt.Errorf("查询节点任务 #%d 失败: %v", id, err)
			}
		} else {
			failures = 0
			st, _ := t["status"].(string)
			switch st {
			case models.TaskSuccess:
				return nil
			case models.TaskFailed:
				msg, _ := t["error_msg"].(string)
				if msg == "" {
					msg = "节点任务失败"
				}
				return fmt.Errorf("节点创建失败: %s", msg)
			}
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("等待节点任务 #%d 超时", id)
		}
		time.Sleep(interval)
		if interval < 10*time.Second {
			interval += 2 * time.Second
		}
	}
}

// DeleteContainer 删除容器
func (nc *NodeClient) DeleteContainer(name string) error {
	_, err := nc.do("DELETE", "/api/system/containers/"+name, nil, nil)
	return err
}

// ContainerAction 容器动作 start/stop/restart
func (nc *NodeClient) ContainerAction(name, action string) error {
	// 节点接口从查询参数读取 action（v1.1.16 及之前只放在 body 里，节点返回「缺少操作类型」，
	// 导致到期停机/续费恢复在节点上从未生效）；同时保留 body 兼容
	_, err := nc.do("POST", "/api/system/containers/"+name+"/action", map[string]string{"action": action}, map[string]interface{}{"action": action})
	return err
}

// GetContainerIP 获取容器 IP
func (nc *NodeClient) GetContainerIP(name string) (string, string, error) {
	data, err := nc.do("GET", "/api/system/containers/"+name+"/ip", nil, nil)
	if err != nil {
		return "", "", err
	}
	ip, _ := data["ip"].(string)
	ipv6, _ := data["ipv6"].(string)
	return ip, ipv6, nil
}

// GetContainer 获取容器信息
func (nc *NodeClient) GetContainer(name string) (map[string]interface{}, error) {
	return nc.do("GET", "/api/system/containers/"+name, nil, nil)
}

// UpdateContainerConfig 更新容器配置
func (nc *NodeClient) UpdateContainerConfig(name string, cfg map[string]interface{}) error {
	_, err := nc.do("PUT", "/api/system/containers/"+name+"/config", nil, cfg)
	return err
}

// CreateConsoleToken 创建 Web 控制台令牌
func (nc *NodeClient) CreateConsoleToken(name string) (string, error) {
	data, err := nc.do("POST", "/api/system/console/create-token", nil, map[string]interface{}{"name": name})
	if err != nil {
		return "", err
	}
	token, _ := data["token"].(string)
	if token == "" {
		logger.Warn("节点未返回 console token，data=%v", data)
		return "", nil
	}
	return token, nil
}

// ListContainers 列出容器
func (nc *NodeClient) ListContainers() ([]map[string]interface{}, error) {
	data, err := nc.do("GET", "/api/system/containers", map[string]string{"limit": "1000"}, nil)
	if err != nil {
		return nil, err
	}
	list, _ := data["containers"].([]interface{})
	out := make([]map[string]interface{}, 0, len(list))
	for _, item := range list {
		if m, ok := item.(map[string]interface{}); ok {
			out = append(out, m)
		}
	}
	return out, nil
}

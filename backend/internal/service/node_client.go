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
	base := strings.TrimRight(node.APIBaseURL, "/")
	// 自签证书场景：跳过 TLS 证书校验（节点间为内网 https 直连）
	tr := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}
	return &NodeClient{
		BaseURL: base,
		APIHash: node.APIHash,
		client:  &http.Client{Timeout: 30 * time.Second, Transport: tr},
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
		Code int                    `json:"code"`
		Msg  string                 `json:"msg"`
		Data map[string]interface{} `json:"data"`
	}
	if err := json.Unmarshal(data, &envelope); err != nil {
		return nil, fmt.Errorf("节点响应解析失败: %v", err)
	}
	if envelope.Code != 200 {
		return nil, fmt.Errorf("节点返回错误: %s", envelope.Msg)
	}
	return envelope.Data, nil
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

// DeleteContainer 删除容器
func (nc *NodeClient) DeleteContainer(name string) error {
	_, err := nc.do("DELETE", "/api/system/containers/"+name, nil, nil)
	return err
}

// ContainerAction 容器动作 start/stop/restart
func (nc *NodeClient) ContainerAction(name, action string) error {
	_, err := nc.do("POST", "/api/system/containers/"+name+"/action", nil, map[string]interface{}{"action": action})
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

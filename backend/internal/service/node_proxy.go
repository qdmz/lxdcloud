package service

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"lxdapi/internal/db"
	"lxdapi/models"
)

// 节点代理：主控把后台/用户/容器面板对节点资源的请求转发到节点 lxdapi。
// 节点侧以 X-API-Hash（系统级密钥）认证，等同管理员。

var nodeTransport = &http.Transport{
	TLSClientConfig:     &tls.Config{InsecureSkipVerify: true}, // 节点多为自签证书
	MaxIdleConnsPerHost: 8,
	IdleConnTimeout:     90 * time.Second,
}

// ProxyToNode 把当前请求转发到节点的 targetPath（保留查询参数，去掉 node_id）。
// withHash=true 时附带节点 api_hash；容器面板请求保留其 X-Container-Hash 即可。
func ProxyToNode(c *gin.Context, node *models.Node, targetPath string, withHash bool) {
	base, err := url.Parse(strings.TrimRight(node.APIBaseURL, "/"))
	if err != nil || base.Host == "" {
		c.JSON(http.StatusOK, gin.H{"code": 502, "msg": "节点地址无效"})
		c.Abort()
		return
	}
	q := c.Request.URL.Query()
	q.Del("node_id")
	rawQuery := q.Encode()

	proxy := &httputil.ReverseProxy{
		Director: func(r *http.Request) {
			r.URL.Scheme = base.Scheme
			r.URL.Host = base.Host
			r.URL.Path = base.Path + targetPath
			r.URL.RawPath = ""
			r.URL.RawQuery = rawQuery
			r.Host = base.Host
			r.Header.Del("Cookie")
			r.Header.Del("X-Forwarded-For")
			// 去掉浏览器原样的 Accept-Encoding：Go Transport 在请求无此头时会自动加 gzip
			// 并透明解压；若原样转发，节点 nginx 返回 gzip 体而 Transport 不解压，
			// ModifyResponse 里的 JSON 解析会静默失败（node 注入/token 记录双双丢失）
			r.Header.Del("Accept-Encoding")
			if withHash {
				r.Header.Set("X-API-Hash", node.APIHash)
			} else {
				r.Header.Del("X-API-Hash")
			}
		},
		Transport: nodeTransport,
		ModifyResponse: func(resp *http.Response) error {
			resp.Header.Del("Set-Cookie")
			// 网页终端：记录节点签发的令牌属于哪个节点，WebSocket 连接到主控时据此转发
			if strings.HasSuffix(targetPath, "/console/create-token") {
				recordConsoleToken(resp, node.ID)
			}
			// 容器信息：注入节点信息，方便前端显示
			if targetPath == "/api/container/info" {
				injectNodeInfo(resp, node)
			}
			return nil
		},
		ErrorHandler: func(w http.ResponseWriter, r *http.Request, e error) {
			w.Header().Set("Content-Type", "application/json; charset=utf-8")
			w.WriteHeader(http.StatusOK)
			b, _ := json.Marshal(gin.H{"code": 502, "msg": "节点连接失败: " + e.Error()})
			w.Write(b)
		},
	}
	proxy.ServeHTTP(c.Writer, c.Request)
	c.Abort()
}

// nodeGetJSON 以 api_hash 调用节点 GET 接口
func nodeGetJSON(node *models.Node, path string, header map[string]string, timeout time.Duration) (map[string]interface{}, error) {
	req, err := http.NewRequest("GET", strings.TrimRight(node.APIBaseURL, "/")+path, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("X-API-Hash", node.APIHash)
	for k, v := range header {
		req.Header.Set(k, v)
	}
	cl := &http.Client{Timeout: timeout, Transport: nodeTransport}
	resp, err := cl.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 16<<20))
	var out map[string]interface{}
	if err := json.Unmarshal(body, &out); err != nil {
		return nil, fmt.Errorf("节点返回非 JSON (HTTP %d)", resp.StatusCode)
	}
	if code, ok := out["code"].(float64); ok && code != 200 {
		msg, _ := out["msg"].(string)
		return out, fmt.Errorf("%s", msg)
	}
	return out, nil
}

// UserNodeInstance 返回用户名下位于节点上的实例（按容器名）
func UserNodeInstance(username, name string) (*models.UserProduct, *models.Node) {
	user, err := GetUserByUsername(username)
	if err != nil {
		return nil, nil
	}
	var up models.UserProduct
	if err := db.DB.Where("user_id = ? AND name = ? AND node_id > 0", user.ID, name).First(&up).Error; err != nil {
		return nil, nil
	}
	node, err := GetNode(up.NodeID)
	if err != nil {
		return nil, nil
	}
	return &up, node
}

// UserNodeContainers 拉取用户在各节点上的容器缓存（与本机缓存同结构）
func UserNodeContainers(username string) []interface{} {
	user, err := GetUserByUsername(username)
	if err != nil {
		return nil
	}
	var ups []models.UserProduct
	db.DB.Where("user_id = ? AND node_id > 0 AND status <> ?", user.ID, "failed").Find(&ups)
	if len(ups) == 0 {
		return nil
	}
	byNode := map[uint]map[string]bool{}
	for _, up := range ups {
		if byNode[up.NodeID] == nil {
			byNode[up.NodeID] = map[string]bool{}
		}
		byNode[up.NodeID][up.Name] = true
	}
	var out []interface{}
	var mu sync.Mutex
	var wg sync.WaitGroup
	for nid, names := range byNode {
		node, err := GetNode(nid)
		if err != nil {
			continue
		}
		wg.Add(1)
		go func(node *models.Node, names map[string]bool) {
			defer wg.Done()
			res, err := nodeGetJSON(node, "/api/admin/cache/containers", nil, 20*time.Second)
			if err != nil {
				return
			}
			data, _ := res["data"].(map[string]interface{})
			list, _ := data["data"].([]interface{})
			for _, it := range list {
				m, ok := it.(map[string]interface{})
				if !ok {
					continue
				}
				n, _ := m["name"].(string)
				if names[n] {
					m["node_id"] = node.ID
					m["node_name"] = node.Name
					mu.Lock()
					out = append(out, m)
					mu.Unlock()
				}
			}
		}(node, names)
	}
	wg.Wait()
	return out
}

// ---- 容器访问码 → 节点 解析（带缓存） ----

var hashNodeCache = struct {
	sync.Mutex
	m map[string]hashNodeEntry
}{m: map[string]hashNodeEntry{}}

type hashNodeEntry struct {
	nodeID uint
	name   string
	at     time.Time
}

// ResolveContainerHashNode 本机不存在的访问码，逐个询问节点；返回所在节点与容器名
func ResolveContainerHashNode(hash string) (*models.Node, string) {
	if hash == "" || len(hash) > 256 {
		return nil, ""
	}
	hashNodeCache.Lock()
	e, ok := hashNodeCache.m[hash]
	hashNodeCache.Unlock()
	ttl := 10 * time.Minute
	if ok && e.nodeID == 0 {
		ttl = time.Minute
	}
	if ok && time.Since(e.at) < ttl {
		if e.nodeID == 0 {
			return nil, ""
		}
		if node, err := GetNode(e.nodeID); err == nil {
			return node, e.name
		}
	}
	nodes, _ := ListNodes()
	var found *models.Node
	name := ""
	for i := range nodes {
		n := &nodes[i]
		res, err := nodeGetJSON(n, "/api/container/info", map[string]string{"X-Container-Hash": hash}, 15*time.Second)
		if err != nil || res == nil {
			continue
		}
		// 必须拿到有效的容器名才算找到，避免空 data 误判
		if d, ok := res["data"].(map[string]interface{}); ok {
			if v, ok := d["name"].(string); ok && v != "" {
				name = v
				found = n
				break
			}
		}
	}
	hashNodeCache.Lock()
	if found != nil {
		hashNodeCache.m[hash] = hashNodeEntry{nodeID: found.ID, name: name, at: time.Now()}
	} else {
		hashNodeCache.m[hash] = hashNodeEntry{at: time.Now()}
	}
	hashNodeCache.Unlock()
	return found, name
}

// ---- 网页终端令牌 → 节点（v1.1.17：补上节点容器 WebSocket 终端未经主控转发的缺口） ----

var consoleTokenNodes = struct {
	sync.Mutex
	m map[string]consoleTokenEntry
}{m: map[string]consoleTokenEntry{}}

type consoleTokenEntry struct {
	nodeID uint
	at     time.Time
}

// recordConsoleToken 从节点 create-token 响应中取出令牌并记录所属节点（不改变响应内容）
func recordConsoleToken(resp *http.Response, nodeID uint) {
	if resp.Body == nil {
		return
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	resp.Body.Close()
	resp.Body = io.NopCloser(bytes.NewReader(body))
	resp.ContentLength = int64(len(body))
	resp.Header.Set("Content-Length", strconv.Itoa(len(body)))
	if err != nil {
		return
	}
	var env struct {
		Code int `json:"code"`
		Data struct {
			Token string `json:"token"`
		} `json:"data"`
	}
	if json.Unmarshal(body, &env) != nil || env.Code != 200 || env.Data.Token == "" {
		return
	}
	consoleTokenNodes.Lock()
	defer consoleTokenNodes.Unlock()
	now := time.Now()
	for k, v := range consoleTokenNodes.m { // 令牌一次性且短时有效，顺带清理过期记录
		if now.Sub(v.at) > 10*time.Minute {
			delete(consoleTokenNodes.m, k)
		}
	}
	consoleTokenNodes.m[env.Data.Token] = consoleTokenEntry{nodeID: nodeID, at: now}
}

// ConsoleTokenNode 令牌由节点签发时返回节点（取出即删除，令牌只能用一次）
func ConsoleTokenNode(token string) *models.Node {
	if token == "" {
		return nil
	}
	consoleTokenNodes.Lock()
	e, ok := consoleTokenNodes.m[token]
	if ok {
		delete(consoleTokenNodes.m, token)
	}
	consoleTokenNodes.Unlock()
	if !ok || time.Since(e.at) > 10*time.Minute {
		return nil
	}
	node, err := GetNode(e.nodeID)
	if err != nil {
		return nil
	}
	return node
}

// ---- 用户端聚合：节点上属于该用户实例的端口映射 / 反向代理 ----

// userNodeNames 用户在各节点上的实例名
func userNodeNames(username string) map[uint]map[string]bool {
	user, err := GetUserByUsername(username)
	if err != nil {
		return nil
	}
	var ups []models.UserProduct
	db.DB.Where("user_id = ? AND node_id > 0 AND status <> ?", user.ID, "failed").Find(&ups)
	byNode := map[uint]map[string]bool{}
	for _, up := range ups {
		if byNode[up.NodeID] == nil {
			byNode[up.NodeID] = map[string]bool{}
		}
		byNode[up.NodeID][up.Name] = true
	}
	return byNode
}

// forEachUserNode 并发遍历用户实例所在节点
func forEachUserNode(username string, only string, fn func(node *models.Node, names map[string]bool)) {
	byNode := userNodeNames(username)
	var wg sync.WaitGroup
	for nid, names := range byNode {
		if only != "" {
			if !names[only] {
				continue
			}
			names = map[string]bool{only: true}
		}
		node, err := GetNode(nid)
		if err != nil {
			continue
		}
		wg.Add(1)
		go func(node *models.Node, names map[string]bool) {
			defer wg.Done()
			fn(node, names)
		}(node, names)
	}
	wg.Wait()
}

func filterTag(list []interface{}, names map[string]bool, node *models.Node) []interface{} {
	out := []interface{}{}
	for _, it := range list {
		m, ok := it.(map[string]interface{})
		if !ok {
			continue
		}
		n, _ := m["container_name"].(string)
		if !names[n] {
			continue
		}
		m["node_id"] = node.ID
		m["node_name"] = node.Name
		out = append(out, m)
	}
	return out
}

// UserNodePortMappings 用户节点实例上的端口映射（only 非空时只取该实例）
func UserNodePortMappings(username, only string) (v4, v6 []interface{}) {
	var mu sync.Mutex
	forEachUserNode(username, only, func(node *models.Node, names map[string]bool) {
		res, err := nodeGetJSON(node, "/api/admin/port-mapping?version=all", nil, 20*time.Second)
		if err != nil {
			return
		}
		data, _ := res["data"].(map[string]interface{})
		a, _ := data["ipv4"].([]interface{})
		b, _ := data["ipv6"].([]interface{})
		fa, fb := filterTag(a, names, node), filterTag(b, names, node)
		mu.Lock()
		v4 = append(v4, fa...)
		v6 = append(v6, fb...)
		mu.Unlock()
	})
	return
}

// UserNodeReverseProxies 用户节点实例上的反向代理
func UserNodeReverseProxies(username string) []interface{} {
	var mu sync.Mutex
	out := []interface{}{}
	forEachUserNode(username, "", func(node *models.Node, names map[string]bool) {
		res, err := nodeGetJSON(node, "/api/admin/nginx/proxies", nil, 20*time.Second)
		if err != nil {
			return
		}
		list, _ := res["data"].([]interface{})
		f := filterTag(list, names, node)
		mu.Lock()
		out = append(out, f...)
		mu.Unlock()
	})
	return out
}

// ContainerHashNode 由容器访问码查所属节点（主控库直接查，不依赖网络探测）
// 本机容器（NodeID=0）或查不到时返回 nil
func ContainerHashNode(hash string) *models.Node {
	cred, err := GetContainerByHash(hash)
	if err != nil || cred == nil {
		return nil
	}
	var container models.Container
	if err := db.DB.Where("name = ?", cred.ContainerName).First(&container).Error; err == nil {
		if container.NodeID == 0 {
			return nil
		}
		if node, err := GetNode(container.NodeID); err == nil {
			return node
		}
		return nil
	}
	// 库里没有容器记录：按容器名去各节点查（节点容器未同步到主控库的场景）
	nodes, _ := ListNodes()
	for i := range nodes {
		n := &nodes[i]
		res, err := nodeGetJSON(n, "/api/admin/containers", nil, 10*time.Second)
		if err != nil || res == nil {
			continue
		}
		if d, ok := res["data"].(map[string]interface{}); ok {
			if list, ok := d["containers"].([]interface{}); ok {
				for _, item := range list {
					if m, ok := item.(map[string]interface{}); ok {
						if name, ok := m["Name"].(string); ok && name == cred.ContainerName {
							return n
						}
						// 兼容小写 name
						if name, ok := m["name"].(string); ok && name == cred.ContainerName {
							return n
						}
					}
				}
			}
		}
	}
	return nil
}

// injectNodeInfo 向 /api/container/info 的代理响应中注入节点信息
func injectNodeInfo(resp *http.Response, node *models.Node) {
	if resp.Body == nil {
		return
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return
	}
	resp.Body.Close()
	var env struct {
		Code int                    `json:"code"`
		Msg  string                 `json:"msg"`
		Data map[string]interface{} `json:"data"`
	}
	if json.Unmarshal(body, &env) != nil || env.Code != 200 || env.Data == nil {
		resp.Body = io.NopCloser(bytes.NewReader(body))
		return
	}
	env.Data["node"] = map[string]interface{}{"id": node.ID, "name": node.Name}
	newBody, _ := json.Marshal(env)
	resp.Body = io.NopCloser(bytes.NewReader(newBody))
	resp.ContentLength = int64(len(newBody))
	resp.Header.Set("Content-Length", strconv.Itoa(len(newBody)))
}

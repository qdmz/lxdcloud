package service

import (
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httputil"
	"net/url"
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
			if withHash {
				r.Header.Set("X-API-Hash", node.APIHash)
			} else {
				r.Header.Del("X-API-Hash")
			}
		},
		Transport: nodeTransport,
		ModifyResponse: func(resp *http.Response) error {
			resp.Header.Del("Set-Cookie")
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
		if d, ok := res["data"].(map[string]interface{}); ok {
			if v, ok := d["name"].(string); ok {
				name = v
			}
		}
		found = n
		break
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

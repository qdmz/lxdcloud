package middleware

import (
	"bytes"
	"crypto/subtle"
	"encoding/json"
	"io"
	"strconv"
	"strings"

	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
	"lxdapi/internal/core"
	"lxdapi/internal/service"
	"lxdapi/pkg/auth"
	"lxdapi/pkg/response"
)

func SystemAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		apiHash := c.GetHeader("X-API-Hash")
		expected := core.GlobalConfig.System.Security.APIHash
		// 未配置（空值/占位符）时禁用系统级 API，避免空请求头即可通过认证
		if core.IsPlaceholder(expected) {
			response.Error(c, 403, "系统级 API 未启用：请在 config.yaml 中配置 system.security.api_hash")
			c.Abort()
			return
		}
		if apiHash == "" || subtle.ConstantTimeCompare([]byte(apiHash), []byte(expected)) != 1 {
			response.Error(c, 401, "系统级认证失败")
			c.Abort()
			return
		}
		c.Set("auth_level", auth.LevelSystem)
		c.Next()
	}
}

// validSystemHash 请求是否携带有效的系统级 api_hash
func validSystemHash(c *gin.Context) bool {
	apiHash := c.GetHeader("X-API-Hash")
	expected := core.GlobalConfig.System.Security.APIHash
	if apiHash == "" || core.IsPlaceholder(expected) {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(apiHash), []byte(expected)) == 1
}

func AdminAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		// 主控通过节点 api_hash 代理管理本节点（系统级密钥，等同管理员）
		if validSystemHash(c) {
			c.Set("auth_level", auth.LevelAdmin)
			c.Set("admin_username", "system")
			c.Next()
			return
		}
		session := sessions.Default(c)
		loggedIn := session.Get("admin_logged_in")
		
		if loggedIn == nil || loggedIn != true {
			response.Error(c, 401, "未登录或登录已过期，请重新登录")
			c.Abort()
			return
		}
		
		username := session.Get("admin_username")
		c.Set("auth_level", auth.LevelAdmin)
		c.Set("admin_username", username)
		c.Next()
	}
}

func AdminPageAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		// 免登录跳转令牌必须真实有效且类型匹配，否则按普通未登录处理
		if t := c.Query("token"); t != "" {
			if at, err := service.ValidateAccessToken(t); err == nil && at.Type == "admin" {
				c.Next()
				return
			}
		}
		
		session := sessions.Default(c)
		loggedIn := session.Get("admin_logged_in")
		
		if loggedIn == nil || loggedIn != true {
			c.Redirect(302, "/admin/login")
			c.Abort()
			return
		}
		
		c.Next()
	}
}

func UserAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		session := sessions.Default(c)
		loggedIn := session.Get("user_logged_in")
		
		if loggedIn == nil || loggedIn != true {
			response.Error(c, 401, "未登录或登录已过期，请重新登录")
			c.Abort()
			return
		}
		
		username := session.Get("user_username")
		if username == nil {
			response.Error(c, 401, "会话无效，请重新登录")
			c.Abort()
			return
		}
		
		user, err := service.GetUserByUsername(username.(string))
		if err != nil || user.Status == "disabled" {
			session.Clear()
			session.Save()
			response.Error(c, 403, "账户已被禁用")
			c.Abort()
			return
		}
		
		c.Set("auth_level", auth.LevelUser)
		c.Set("username", username.(string))
		c.Next()
	}
}

func UserBasicAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		username := c.GetHeader("X-User-Username")
		password := c.GetHeader("X-User-Password")
		
		if username == "" || password == "" {
			response.Error(c, 401, "缺少用户认证信息")
			c.Abort()
			return
		}
		
		user, err := service.GetUserByUsernameAndAPIKey(username, password)
		if err != nil {
			response.Error(c, 401, err.Error())
			c.Abort()
			return
		}
		
		c.Set("auth_level", auth.LevelUser)
		c.Set("username", user.Username)
		c.Next()
	}
}

func UserAuthOrBasic() gin.HandlerFunc {
	return func(c *gin.Context) {
		if c.GetHeader("X-User-Username") != "" {
			UserBasicAuth()(c)
		} else {
			UserAuth()(c)
		}
	}
}

func UserPageAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		if t := c.Query("token"); t != "" {
			if at, err := service.ValidateAccessToken(t); err == nil && at.Type == "user" {
				c.Next()
				return
			}
		}
		
		session := sessions.Default(c)
		loggedIn := session.Get("user_logged_in")
		
		if loggedIn == nil || loggedIn != true {
			c.Redirect(302, "/user/login")
			c.Abort()
			return
		}
		
		username := session.Get("user_username")
		if username != nil {
			user, err := service.GetUserByUsername(username.(string))
			if err != nil || user.Status == "disabled" {
				session.Clear()
				session.Save()
				c.Redirect(302, "/user/login?error=disabled")
				c.Abort()
				return
			}
		}
		
		c.Next()
	}
}

func ContainerAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		containerHash := c.GetHeader("X-Container-Hash")
		if containerHash == "" {
			response.Error(c, 401, "缺少容器凭证")
			c.Abort()
			return
		}
		
		cred, err := service.GetContainerByHash(containerHash)
		if err != nil {
			// 本机没有该访问码：可能是节点上的容器，转发到所在节点
			if node, _ := service.ResolveContainerHashNode(containerHash); node != nil {
				service.ProxyToNode(c, node, c.Request.URL.Path, false)
				return
			}
			response.Error(c, 401, "容器级认证失败")
			c.Abort()
			return
		}
		
		c.Set("auth_level", auth.LevelContainer)
		c.Set("container_name", cred.ContainerName)
		c.Next()
	}
}


// AdminNodeProxy 后台请求带 node_id 时转发到对应节点（节点管理接口本身除外）
func AdminNodeProxy() gin.HandlerFunc {
	return func(c *gin.Context) {
		nid := c.Query("node_id")
		if nid == "" || nid == "0" || strings.HasPrefix(c.Request.URL.Path, "/api/admin/nodes") || strings.HasPrefix(c.Request.URL.Path, "/api/admin/regions") {
			c.Next()
			return
		}
		id, err := strconv.ParseUint(nid, 10, 64)
		if err != nil {
			response.Error(c, 400, "node_id 无效")
			c.Abort()
			return
		}
		node, err := service.GetNode(uint(id))
		if err != nil {
			response.Error(c, 404, "节点不存在")
			c.Abort()
			return
		}
		service.ProxyToNode(c, node, c.Request.URL.Path, true)
	}
}

// userToAdminPath 用户容器接口路径 → 节点后台接口路径
func userToAdminPath(p string) string {
	p = strings.Replace(p, "/api/user/", "/api/admin/", 1)
	if strings.HasSuffix(p, "/credential/regenerate") {
		p = strings.TrimSuffix(p, "/regenerate")
	}
	return p
}

// UserNodeProxy 用户操作自己名下节点上的容器时，转发到节点后台接口
func UserNodeProxy() gin.HandlerFunc {
	return func(c *gin.Context) {
		name := c.Param("name")
		if name == "" || !strings.HasPrefix(c.Request.URL.Path, "/api/user/containers/") {
			c.Next()
			return
		}
		username, _ := c.Get("username")
		uname, _ := username.(string)
		if uname == "" {
			c.Next()
			return
		}
		up, node := service.UserNodeInstance(uname, name)
		if up == nil || node == nil {
			c.Next()
			return
		}
		if c.Request.Method == "DELETE" && c.Request.URL.Path == "/api/user/containers/"+name {
			response.Error(c, 400, "节点实例请在「我的实例」中删除")
			c.Abort()
			return
		}
		service.ProxyToNode(c, node, userToAdminPath(c.Request.URL.Path), true)
	}
}

// UserConsoleNodeProxy 用户为节点实例申请网页终端令牌时转发到节点（body.hostname 为实例名）
func UserConsoleNodeProxy() gin.HandlerFunc {
	return func(c *gin.Context) {
		uname := c.GetString("username")
		if uname == "" || c.Request.Body == nil {
			c.Next()
			return
		}
		body, err := io.ReadAll(io.LimitReader(c.Request.Body, 1<<16))
		c.Request.Body = io.NopCloser(bytes.NewReader(body))
		if err != nil {
			c.Next()
			return
		}
		var req struct {
			Hostname string `json:"hostname"`
		}
		_ = json.Unmarshal(body, &req)
		if req.Hostname == "" {
			c.Next()
			return
		}
		up, node := service.UserNodeInstance(uname, req.Hostname)
		if up == nil || node == nil {
			c.Next()
			return
		}
		c.Request.ContentLength = int64(len(body))
		service.ProxyToNode(c, node, "/api/admin/console/create-token", true)
	}
}

// ConsoleNodeProxy /ws/console：令牌由节点签发时以 WebSocket 反向代理到该节点
func ConsoleNodeProxy() gin.HandlerFunc {
	return func(c *gin.Context) {
		if node := service.ConsoleTokenNode(c.Query("token")); node != nil {
			service.ProxyToNode(c, node, "/ws/console", false)
			return
		}
		c.Next()
	}
}

// AdminNodeFallback 主控未注册的后台接口（如主控未启用 nginx/防火墙插件）带 node_id 时，
// 先做管理员鉴权再转发到节点；返回 true 表示已处理
func AdminNodeFallback(c *gin.Context) bool {
	p := c.Request.URL.Path
	nid := c.Query("node_id")
	if !strings.HasPrefix(p, "/api/admin/") || nid == "" || nid == "0" {
		return false
	}
	AdminAuth()(c)
	if c.IsAborted() {
		return true
	}
	AdminNodeProxy()(c)
	if !c.IsAborted() {
		response.Error(c, 404, "接口不存在")
		c.Abort()
	}
	return true
}

// ContainerNodeFallback 主控未注册的容器面板接口（如主控未启用 nginx 插件时的 /api/container/nginx/*）
// 若访问码属于节点容器则转发到节点；返回 true 表示已处理
func ContainerNodeFallback(c *gin.Context) bool {
	if !strings.HasPrefix(c.Request.URL.Path, "/api/container/") {
		return false
	}
	h := c.GetHeader("X-Container-Hash")
	if h == "" {
		return false
	}
	if _, err := service.GetContainerByHash(h); err == nil {
		return false // 本机容器，按本机逻辑处理
	}
	if node, _ := service.ResolveContainerHashNode(h); node != nil {
		service.ProxyToNode(c, node, c.Request.URL.Path, false)
		return true
	}
	return false
}

// ContainerConsoleNodeProxy 容器为节点实例申请网页终端令牌时转发到节点
// （X-Container-Hash 属于节点容器时；本机容器走本机逻辑）
func ContainerConsoleNodeProxy() gin.HandlerFunc {
	return func(c *gin.Context) {
		h := c.GetHeader("X-Container-Hash")
		if h == "" {
			c.Next()
			return
		}
		// 主控库直接查容器 NodeID（不依赖网络探测）
		if node := service.ContainerHashNode(h); node != nil {
			service.ProxyToNode(c, node, "/api/admin/console/create-token", true)
			return
		}
		// 兜底：网络探测（兼容主控库无记录的场景）
		if node, _ := service.ResolveContainerHashNode(h); node != nil {
			service.ProxyToNode(c, node, "/api/admin/console/create-token", true)
			return
		}
		c.Next()
	}
}

// ContainerNodeProxy 容器面板接口：访问码属于节点容器时转发到节点的对应接口
// （用于已注册的容器端路由，如 port-mapping；未注册路由走 ContainerNodeFallback）
func ContainerNodeProxy(targetPath string) gin.HandlerFunc {
	return func(c *gin.Context) {
		h := c.GetHeader("X-Container-Hash")
		if h == "" {
			c.Next()
			return
		}
		if node := service.ContainerHashNode(h); node != nil {
			service.ProxyToNode(c, node, targetPath, false)
			return
		}
		if node, _ := service.ResolveContainerHashNode(h); node != nil {
			service.ProxyToNode(c, node, targetPath, false)
			return
		}
		c.Next()
	}
}

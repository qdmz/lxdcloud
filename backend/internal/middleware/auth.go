package middleware

import (
	"crypto/subtle"
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
		if nid == "" || nid == "0" || strings.HasPrefix(c.Request.URL.Path, "/api/admin/nodes") {
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

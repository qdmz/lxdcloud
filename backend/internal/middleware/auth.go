package middleware

import (
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
		if apiHash != core.GlobalConfig.System.Security.APIHash {
			response.Error(c, 401, "系统级认证失败")
			c.Abort()
			return
		}
		c.Set("auth_level", auth.LevelSystem)
		c.Next()
	}
}

func AdminAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
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
		if c.Query("token") != "" {
			c.Next()
			return
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
		if c.Query("token") != "" {
			c.Next()
			return
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
			response.Error(c, 401, "容器级认证失败")
			c.Abort()
			return
		}
		
		c.Set("auth_level", auth.LevelContainer)
		c.Set("container_name", cred.ContainerName)
		c.Next()
	}
}


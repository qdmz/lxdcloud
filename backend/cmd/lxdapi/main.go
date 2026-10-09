package main

import (
	"context"
	"crypto/rand"
	"embed"
	"encoding/json"
	"fmt"
	"github.com/gin-contrib/sessions"
	"github.com/gin-contrib/sessions/cookie"
	"github.com/gin-gonic/gin"
	"html/template"
	"io/fs"
	"net/http"
	"lxdapi/handlers"
	_ "lxdapi/docs"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
	"lxdapi/internal/api/admin"
	"lxdapi/internal/api/console"
	"lxdapi/internal/api/container"
	"lxdapi/internal/api/public"
	"lxdapi/internal/api/system"
	"lxdapi/internal/api/user"
	"lxdapi/internal/core"
	"lxdapi/internal/db"
	"lxdapi/internal/executor"
	"lxdapi/internal/ipv4"
	"lxdapi/internal/ipv6"
	"lxdapi/internal/middleware"
	"lxdapi/internal/service"
	"lxdapi/internal/task"
	"lxdapi/internal/traffic"
	"lxdapi/models"
	"lxdapi/pkg/logger"
	"lxdapi/pkg/plugin"
	"lxdapi/pkg/response"
	sysinfo "lxdapi/pkg/system"
	tlsManager "lxdapi/pkg/tls"
	"lxdapi/plugins/nginx"
	"lxdapi/plugins/opengfw"
	"log"
	"os"
	"path/filepath"
	"strings"
)

//go:embed templates docs static
var embeddedFiles embed.FS

// Version 程序版本号，发布时可通过 -ldflags "-X main.Version=x.y.z" 注入
var Version = "1.1.1"

// sessionSecret 返回会话签名密钥；未配置或仍为占位符时生成随机密钥（重启后需重新登录）
func sessionSecret(cfg *core.Config) []byte {
	if !core.IsPlaceholder(cfg.Admin.SessionSecret) && len(cfg.Admin.SessionSecret) >= 16 {
		return []byte(cfg.Admin.SessionSecret)
	}
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		log.Fatalf("生成会话密钥失败: %v", err)
	}
	logger.Warn("admin.session_secret 未配置、过短或仍为占位符，已使用随机密钥（重启后所有会话失效），请在 config.yaml 中设置至少 16 位随机字符串")
	return buf
}

func main() {
	if err := core.LoadConfig("configs/config.yaml"); err != nil {
		log.Fatalf("配置加载失败: %v", err)
	}

	cfg := core.GlobalConfig

	logger.Init(cfg.System.Logger.Level, cfg.System.Logger.Colorful)
	logger.Info("配置加载成功")

	if err := db.Init(); err != nil {
		logger.Error("数据库初始化失败: %v", err)
		return
	}
	logger.OK("数据库连接成功")

	service.EnsureMailTemplates()
	go service.StartLifecycleScheduler()

	logger.Info("正在初始化网络组件...")
	
	if err := ipv4.InitManager(); err != nil {
		logger.Error("IPv4管理器初始化失败: %v", err)
	}

	if err := ipv6.InitManager(); err != nil {
		logger.Error("IPv6管理器初始化失败: %v", err)
	}
	
	var portMappingV4Count int64
	db.DB.Model(&models.PortMappingV4{}).Count(&portMappingV4Count)
	logger.OK("IPv4端口映射: %d条规则", portMappingV4Count)
	
	var portMappingV6Count int64
	db.DB.Model(&models.PortMappingV6{}).Count(&portMappingV6Count)
	logger.OK("IPv6端口映射: %d条规则", portMappingV6Count)
	


	if err := traffic.InitMonitor(); err != nil {
		logger.Error("流量监控器初始化失败: %v", err)
	} else {
		logger.OK("流量监控器初始化成功")
	}

	pluginManager := plugin.InitManager()
	logger.OK("插件管理器初始化成功")
	
	opengfwPlugin := opengfw.NewOpenGFWPlugin()
	if err := pluginManager.Register(opengfwPlugin); err != nil {
		logger.Error("注册 OpenGFW 插件失败: %v", err)
	} else {
		logger.OK("OpenGFW 插件注册成功")
	}

	if cfg.Plugins.Nginx.Enabled {
		nginxPlugin := nginx.NewNginxPlugin()
		if err := pluginManager.Register(nginxPlugin); err != nil {
			logger.Error("注册 Nginx 插件失败: %v", err)
		} else {
			logger.OK("Nginx 插件注册成功")
		}
	} else {
		logger.Info("Nginx 插件已禁用，跳过注册")
	}

		
	if err := pluginManager.StartAll(); err != nil {
		logger.Error("插件启动失败: %v", err)
	} else {
		logger.OK("所有插件启动成功")
	}

	containerService := service.NewContainerService()
	ipv4Service := service.NewIPv4Service()
	ipv6Service := service.NewIPv6Service()

	system.InitContainerService(containerService)
	system.InitIPv4Service(ipv4Service)
	system.InitIPv6Service(ipv6Service)
	user.InitContainerService(containerService)
	container.InitContainerService(containerService)
	admin.InitBrandService()
	admin.SetEmbeddedFS(embeddedFiles)

	executor.RegisterTaskExecutor("create", func(ctx context.Context, name string, paramsJSON string) error {
		var params map[string]interface{}
		if paramsJSON != "" {
			if err := json.Unmarshal([]byte(paramsJSON), &params); err != nil {
				return fmt.Errorf("解析创建参数失败: %v", err)
			}
		}
		
		getString := func(key string) string {
			if v, ok := params[key]; ok {
				if s, ok := v.(string); ok {
					return s
				}
			}
			return ""
		}
		getInt := func(key string) int {
			if v, ok := params[key]; ok {
				if f, ok := v.(float64); ok {
					return int(f)
				}
			}
			return 0
		}
		getBool := func(key string) bool {
			if v, ok := params[key]; ok {
				if b, ok := v.(bool); ok {
					return b
				}
			}
			return false
		}
		
		req := &models.CreateContainerRequest{
			Name:              getString("name"),
			Image:             getString("image"),
			Password:          getString("password"),
			CPU:               getInt("cpu"),
			Memory:            getInt("memory"),
			Disk:              getInt("disk"),
			Ingress:           getInt("ingress"),
			Egress:            getInt("egress"),
			TrafficLimit:      getInt("traffic_limit"),
			AllowNesting:      getBool("allow_nesting"),
			MemorySwap:        getBool("memory_swap"),
			Privileged:        getBool("privileged"),
			Username:          getString("username"),
			IPv4PoolLimit:     getInt("ipv4_pool_limit"),
			IPv4MappingLimit:  getInt("ipv4_mapping_limit"),
			IPv6PoolLimit:     getInt("ipv6_pool_limit"),
			IPv6MappingLimit:  getInt("ipv6_mapping_limit"),
			ReverseProxyLimit: getInt("reverse_proxy_limit"),
			CPUAllowance:      getInt("cpu_allowance"),
			IORead:            getInt("io_read"),
			IOWrite:           getInt("io_write"),
			ProcessesLimit:    getInt("processes_limit"),
		}
		
		return containerService.Create(ctx, req)
	})
	executor.RegisterTaskExecutor("start", func(ctx context.Context, name string, paramsJSON string) error {
		return containerService.Start(ctx, name)
	})
	executor.RegisterTaskExecutor("stop", func(ctx context.Context, name string, paramsJSON string) error {
		return containerService.Stop(ctx, name)
	})
	executor.RegisterTaskExecutor("restart", func(ctx context.Context, name string, paramsJSON string) error {
		return containerService.Restart(ctx, name)
	})
	executor.RegisterTaskExecutor("delete", func(ctx context.Context, name string, paramsJSON string) error {
		return containerService.Delete(ctx, name)
	})
	executor.RegisterTaskExecutor("reinstall", func(ctx context.Context, name string, paramsJSON string) error {
		var params map[string]interface{}
		image := ""
		password := ""
		if paramsJSON != "" {
			if err := json.Unmarshal([]byte(paramsJSON), &params); err == nil {
				if v, ok := params["image"].(string); ok {
					image = v
				}
				if v, ok := params["password"].(string); ok {
					password = v
				}
			}
		}
		return containerService.Reinstall(ctx, name, image, password)
	})

	executor.ClearPendingTasks()

	if err := executor.InitQueue(); err != nil {
		logger.Error("任务队列初始化失败: %v", err)
	} else {
		logger.OK("任务队列初始化成功")
	}

	ctx := context.Background()
	if traffic.GlobalMonitor != nil {
		go traffic.GlobalMonitor.Start(ctx)
		logger.OK("流量监控器已启动")
	}

	task.StartAutoCleanup()
	logger.OK("自动清理任务已启动")

	task.StartCacheAutoRefresh()
	logger.OK("缓存自动刷新调度器已启动")

	gin.SetMode(cfg.System.Server.Mode)
	r := gin.Default()

	// 只信任本机/内网反向代理的 X-Forwarded-For，防止伪造 IP 绕过登录限流
	trusted := cfg.System.Server.TrustedProxies
	if len(trusted) == 0 {
		trusted = []string{"127.0.0.1", "::1", "10.0.0.0/8", "172.16.0.0/12", "192.168.0.0/16", "fc00::/7"}
	}
	if err := r.SetTrustedProxies(trusted); err != nil {
		logger.Warn("trusted_proxies 配置无效: %v", err)
	}
	r.MaxMultipartMemory = 32 << 20

	if core.IsPlaceholder(cfg.System.Security.APIHash) {
		logger.Warn("system.security.api_hash 未配置，系统级 API（/api/system/*）已禁用")
	}
	if core.IsPlaceholder(cfg.Admin.Password) {
		logger.Warn("admin.password 未配置或仍为占位符，管理员将无法登录，请尽快修改 config.yaml")
	}

	store := cookie.NewStore(sessionSecret(cfg))
	store.Options(sessions.Options{
		Path:     "/",
		MaxAge:   86400,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})
	r.Use(middleware.SecurityHeaders())
	r.Use(sessions.Sessions("lxdapi_session", store))
	r.Use(middleware.SessionCookieOptions())
	r.Use(middleware.BrandMiddleware())

	tmpl := template.New("").Funcs(template.FuncMap{})
	
	templatesFS, err := fs.Sub(embeddedFiles, "templates")
	if err != nil {
		logger.Error("加载嵌入式模板失败: %v", err)
		patterns := []string{
			"templates/*.html",
			"templates/admin/*.html",
			"templates/user/*.html",
			"templates/container/*.html",
		}
		
		for _, pattern := range patterns {
			files, err := filepath.Glob(pattern)
			if err != nil {
				logger.Error("加载模板失败 %s: %v", pattern, err)
				continue
			}
			for _, file := range files {
				name := filepath.ToSlash(file[len("templates/"):])
				content, err := os.ReadFile(file)
				if err != nil {
					logger.Error("读取模板失败 %s: %v", file, err)
					continue
				}
				_, err = tmpl.New(name).Parse(string(content))
				if err != nil {
					logger.Error("解析模板失败 %s: %v", file, err)
				}
			}
		}
	} else {
		fs.WalkDir(templatesFS, ".", func(path string, d fs.DirEntry, err error) error {
			if err != nil || d.IsDir() || filepath.Ext(path) != ".html" {
				return nil
			}
			
			content, err := fs.ReadFile(templatesFS, path)
			if err != nil {
				logger.Error("读取嵌入式模板失败 %s: %v", path, err)
				return nil
			}
			
			name := filepath.ToSlash(path)
			_, err = tmpl.New(name).Parse(string(content))
			if err != nil {
				logger.Error("解析嵌入式模板失败 %s: %v", path, err)
			}
			return nil
		})
	}
	
	r.SetHTMLTemplate(tmpl)
	logger.OK("模板加载完成")
	
	r.NoRoute(func(c *gin.Context) {
		// API 路径返回 JSON 404，避免前端把首页 JSON 当成接口数据
		if strings.HasPrefix(c.Request.URL.Path, "/api/") {
			response.Error(c, 404, "接口不存在")
			return
		}
		c.Redirect(302, "/")
	})

	r.GET("/api/public/version", func(c *gin.Context) {
		response.Success(c, gin.H{"version": Version})
	})
	r.GET("/healthz", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "ok", "version": Version})
	})

	pluginManager.RegisterRoutes(r)

	r.GET("/", func(c *gin.Context) {
		sysInfo := sysinfo.GetSystemInfo()
		response.Success(c, gin.H{
			"author":       "xkatld",
			"project":      "https://github.com/xkatld/lxdapi-web-server",
			"description":  "主流财务系统对接支持，提供完整的Web管理界面与RESTful API",
			"version":      "v" + Version,
			"name":         sysInfo.Name,
			"docs":         sysInfo.Docs,
			"os":           sysInfo.OS,
			"arch":         sysInfo.Arch,
			"lxd_version":  sysInfo.LXDVersion,
			"distribution": sysInfo.Distribution,
			"kernel":       sysInfo.Kernel,
		})
	})
	
	r.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))

	staticFS, err := fs.Sub(embeddedFiles, "static")
	if err == nil {
		r.StaticFS("/static", http.FS(staticFS))
	}

	r.GET("/console", handlers.ConsolePage)
	r.GET("/ws/console", console.HandleWebSocket)
	r.GET("/container/dashboard/lite", handlers.ContainerDashboardLite)
	r.GET("/container/dashboard/base", handlers.ContainerDashboardBase)
	
	r.GET("/admin/login", handlers.AdminLogin)
	r.GET("/api/admin/captcha", admin.GetCaptcha)
	r.POST("/api/admin/login", admin.Login)
	
	adminPages := r.Group("/admin")
	adminPages.Use(middleware.AdminPageAuth())
	{
		adminPages.GET("/dashboard", handlers.AdminDashboard)
		adminPages.GET("/containers", handlers.AdminContainers)
		adminPages.GET("/containers/:name", handlers.AdminContainerDetail)
		adminPages.GET("/users", handlers.AdminUsers)
		adminPages.GET("/users/:id", handlers.AdminUserDetail)
		adminPages.GET("/tasks", handlers.AdminTasks)
		adminPages.GET("/ip-pool/v4", handlers.AdminIPPoolV4)
		adminPages.GET("/ip-pool/v6", handlers.AdminIPPoolV6)
		adminPages.GET("/port-mapping/v4", handlers.AdminPortMappingV4)
		adminPages.GET("/port-mapping/v6", handlers.AdminPortMappingV6)
		adminPages.GET("/templates", handlers.AdminTemplates)
		adminPages.GET("/brand-settings", handlers.AdminBrandSettings)
		adminPages.GET("/firewall", handlers.AdminFirewall)
		adminPages.GET("/nginx", handlers.AdminNginx)
		adminPages.GET("/storage-pools", handlers.AdminStoragePools)
	}
	
	r.POST("/api/admin/logout", admin.Logout)
	r.GET("/admin/logout", handlers.AdminLogout)
	
	r.GET("/api/user/captcha", user.GetCaptcha)
	r.POST("/api/user/login", user.Login)
	r.POST("/api/user/logout", user.Logout)

	r.GET("/user/login", handlers.UserLogin)
	
	userPages := r.Group("/user")
	userPages.Use(middleware.UserPageAuth())
	{
		userPages.GET("/dashboard", handlers.UserDashboard)
		userPages.GET("/containers", handlers.UserContainers)
		userPages.GET("/container", handlers.UserContainerDetail)
	}

	r.GET("/container/login", handlers.ContainerLogin)
	r.GET("/container/dashboard", handlers.ContainerDashboard)

	systemAPI := r.Group("/api/system")
	systemAPI.Use(middleware.SystemAuth())
	{
		systemAPI.POST("/containers", system.CreateContainer)
		systemAPI.GET("/containers", system.ListContainers)
		systemAPI.GET("/containers/:name", system.GetContainer)
		systemAPI.DELETE("/containers/:name", system.DeleteContainer)
		systemAPI.POST("/containers/:name/action", system.ContainerAction)
		systemAPI.PUT("/containers/:name/config", system.UpdateContainerConfig)
		systemAPI.POST("/port-mapping/allocate", system.AllocatePortMapping)
		systemAPI.POST("/port-mapping/release", system.ReleasePortMapping)
		systemAPI.GET("/port-mapping", system.ListPortMappings)
		systemAPI.GET("/ip", system.GetContainerIP)
		systemAPI.POST("/ip/allocate", system.AllocateIP)
		systemAPI.POST("/ip/release", system.ReleaseIP)
		systemAPI.GET("/traffic", system.GetTraffic)
		systemAPI.POST("/traffic/reset", system.ResetTraffic)
		systemAPI.GET("/tasks", system.ListTasks)
		systemAPI.GET("/tasks/detail", system.GetTask)
		systemAPI.POST("/users", system.CreateUser)
		systemAPI.GET("/users", system.GetUsers)
		systemAPI.PUT("/users/:id", system.UpdateUser)
		systemAPI.DELETE("/users/:id", system.DeleteUser)
		systemAPI.POST("/users/:id/regenerate-key", system.RegenerateUserAPIKey)
		systemAPI.GET("/containers/:name/credential", system.GetContainerCredential)
		systemAPI.POST("/containers/:name/credential/regenerate", system.RegenerateContainerCredential)
		systemAPI.POST("/console/create-token", console.CreateToken)
		systemAPI.GET("/admin/access-token", system.GetAdminAccessToken)
		systemAPI.GET("/users/:username/access-token", system.GetUserAccessToken)
	}

	r.GET("/api/port-range/config", admin.GetPortRangeConfig)

	adminAPI := r.Group("/api/admin")
	adminAPI.Use(middleware.AdminAuth())
	{
		adminAPI.GET("/dashboard", admin.Dashboard)
		adminAPI.GET("/host/stats", admin.GetHostStats)
		adminAPI.GET("/containers", admin.ListContainers)
		adminAPI.GET("/containers/:name", admin.GetContainer)
		adminAPI.DELETE("/containers/:name", admin.DeleteContainer)
		adminAPI.POST("/containers/:name/action", admin.ContainerAction)
		adminAPI.GET("/containers/:name/config", admin.GetContainerConfig)
		adminAPI.PUT("/containers/:name/config", admin.UpdateContainerConfig)
		adminAPI.GET("/containers/:name/dns", admin.GetContainerDNS)
		adminAPI.PUT("/containers/:name/dns", admin.SetContainerDNS)
		adminAPI.PUT("/containers/:name/remark", admin.UpdateContainerRemark)
		adminAPI.POST("/containers/create", system.CreateContainer)
		adminAPI.GET("/containers/:name/credential", admin.GetContainerCredential)
		adminAPI.POST("/containers/:name/credential", admin.RegenerateContainerCredential)
		adminAPI.GET("/containers/:name/ip", admin.GetContainerIP)
		adminAPI.POST("/containers/:name/ip/allocate", admin.AllocateContainerIP)
		adminAPI.GET("/cache/containers", admin.GetContainersCache)
		adminAPI.POST("/cache/refresh", admin.RefreshCache)
		adminAPI.GET("/cache/auto-refresh/settings", admin.GetAutoRefreshSettings)
		adminAPI.PUT("/cache/auto-refresh/settings", admin.UpdateAutoRefreshSettings)
		adminAPI.GET("/tasks", admin.GetTaskList)
		adminAPI.GET("/tasks/:id", admin.GetTask)
		adminAPI.DELETE("/tasks/:id", admin.DeleteTask)
		adminAPI.POST("/tasks/batch-delete", admin.BatchDeleteTasks)
		adminAPI.GET("/port-mapping", admin.ListPortMappings)
		adminAPI.POST("/port-mapping/allocate", admin.AllocatePortMapping)
		adminAPI.POST("/port-mapping/release", admin.ReleasePortMapping)
		adminAPI.GET("/nat-config", admin.GetNATConfig)
		adminAPI.POST("/nat-config", admin.SaveNATConfig)
		adminAPI.GET("/ip", admin.GetIPList)
		adminAPI.POST("/ip/allocate", admin.AllocateIP)
		adminAPI.POST("/ip/release", admin.ReleaseIP)
		adminAPI.GET("/ip/pool", admin.GetIPPool)
		adminAPI.POST("/ip/pool", admin.AddIPToPool)
		adminAPI.PUT("/ip/pool", admin.UpdateIPPool)
		adminAPI.DELETE("/ip/pool", admin.DeleteIPFromPool)
		adminAPI.POST("/ip/pool/batch", admin.BatchAddIPToPool)
		adminAPI.POST("/ip/pool/batch-delete", admin.BatchDeleteIPFromPool)
		adminAPI.GET("/users", admin.GetUsers)
		adminAPI.POST("/users", admin.CreateUser)
		adminAPI.PUT("/users/:id", admin.UpdateUser)
		adminAPI.DELETE("/users/:id", admin.DeleteUser)
		adminAPI.POST("/users/batch-delete", admin.BatchDeleteUsers)
		adminAPI.POST("/users/:id/regenerate-key", admin.RegenerateAPIKey)
		adminAPI.GET("/templates", admin.GetTemplateList)
		adminAPI.POST("/templates/sync", admin.SyncTemplates)
		adminAPI.DELETE("/templates/:fingerprint", admin.DeleteTemplate)
		adminAPI.POST("/templates/batch-delete", admin.BatchDeleteTemplates)
		adminAPI.GET("/templates/:fingerprint/permission", admin.GetTemplatePermission)
		adminAPI.PUT("/templates/:fingerprint/permission", admin.SetTemplatePermission)
		adminAPI.GET("/brand-settings", admin.GetBrandSettings)
		adminAPI.POST("/brand-settings", admin.UpdateBrandSettings)
		adminAPI.GET("/container-templates", admin.GetContainerTemplates)
		adminAPI.GET("/port-range/config", admin.GetPortRangeConfig)
		adminAPI.POST("/port-range/config", admin.SavePortRangeConfig)
		adminAPI.GET("/ip-pool/settings", admin.GetIPPoolSettings)
		adminAPI.PUT("/ip-pool/settings", admin.UpdateIPPoolSettings)
		adminAPI.POST("/console/create-token", console.CreateToken)
		adminAPI.GET("/storage-pools", admin.GetStoragePools)
		adminAPI.POST("/storage-pools/sync", admin.SyncStoragePools)
		adminAPI.PUT("/storage-pools/:name/priority", admin.SetStoragePoolPriority)
		adminAPI.GET("/network/nat", admin.GetNetworkNATStatus)
		adminAPI.POST("/network/nat", admin.SetNetworkNATStatus)
		// ---- 商业化模块 ----
		adminAPI.GET("/nodes", admin.ListNodes)
		adminAPI.POST("/nodes", admin.CreateNode)
		adminAPI.PUT("/nodes/:id", admin.UpdateNode)
		adminAPI.DELETE("/nodes/:id", admin.DeleteNode)
		adminAPI.POST("/nodes/:id/test", admin.TestNode)
		adminAPI.GET("/smtp", admin.GetSMTP)
		adminAPI.POST("/smtp", admin.SaveSMTP)
		adminAPI.POST("/smtp/test", admin.TestMail)
		adminAPI.GET("/mail-templates", admin.ListMailTemplates)
		adminAPI.POST("/mail-templates/:code", admin.SaveMailTemplate)
		adminAPI.GET("/pay", admin.GetPay)
		adminAPI.POST("/pay", admin.SavePay)
		adminAPI.GET("/products", admin.ListProducts)
		adminAPI.POST("/products", admin.CreateProduct)
		adminAPI.PUT("/products/:id", admin.UpdateProduct)
		adminAPI.DELETE("/products/:id", admin.DeleteProduct)
		adminAPI.GET("/orders", admin.ListOrders)
		adminAPI.GET("/instances", admin.ListInstances)
		adminAPI.DELETE("/instances/:id", admin.DeleteInstance)
		adminAPI.GET("/tickets", admin.ListTickets)
		adminAPI.GET("/tickets/:id", admin.GetTicket)
		adminAPI.POST("/tickets/:id/reply", admin.ReplyTicket)
		adminAPI.POST("/tickets/:id/close", admin.CloseTicket)
		// ---- 容器文件管理 ----
		adminAPI.GET("/containers/:name/files", admin.ListFiles)
		adminAPI.GET("/containers/:name/files/download", admin.DownloadFile)
		adminAPI.POST("/containers/:name/files/upload", admin.UploadFile)
		adminAPI.POST("/containers/:name/files/mkdir", admin.MakeDir)
		adminAPI.POST("/containers/:name/files/delete", admin.DeleteFile)
		adminAPI.POST("/containers/:name/files/rename", admin.RenameFile)
	}

	userAPI := r.Group("/api/user")
	userAPI.Use(middleware.UserAuthOrBasic())
	{
		userAPI.GET("/info", user.GetUserInfo)
		userAPI.POST("/containers", user.CreateContainer)
		userAPI.GET("/containers", user.ListContainers)
		userAPI.GET("/containers/:name", user.GetContainer)
		userAPI.DELETE("/containers/:name", user.DeleteContainer)
		userAPI.POST("/containers/:name/action", user.ContainerAction)
		userAPI.GET("/containers/:name/config", user.GetContainerConfig)
		userAPI.PUT("/containers/:name/config", user.UpdateContainerConfig)
		userAPI.GET("/containers/:name/dns", user.GetContainerDNS)
		userAPI.PUT("/containers/:name/dns", user.SetContainerDNS)
		userAPI.PUT("/containers/:name/remark", user.UpdateContainerRemark)
		userAPI.POST("/cache/refresh", user.RefreshCache)
		userAPI.GET("/containers/:name/credential", user.GetContainerCredential)
		userAPI.POST("/containers/:name/credential/regenerate", user.RegenerateContainerCredential)
		userAPI.POST("/port-mapping/allocate", user.AllocatePortMapping)
		userAPI.POST("/port-mapping/release", user.ReleasePortMapping)
		userAPI.GET("/port-mapping", user.ListPortMappings)
		userAPI.GET("/containers/:name/ip", user.GetContainerIP)
		userAPI.POST("/containers/:name/ip/allocate", user.AllocateContainerIP)
		userAPI.POST("/containers/:name/ip/release", user.ReleaseContainerIP)
		userAPI.GET("/templates", user.GetTemplateList)
		userAPI.GET("/tasks/:id", user.GetTask)
		userAPI.POST("/console/create-token", console.CreateToken)
		// ---- 容器文件管理 ----
		userAPI.GET("/containers/:name/files", user.ListFiles)
		userAPI.GET("/containers/:name/files/download", user.DownloadFile)
		userAPI.POST("/containers/:name/files/upload", user.UploadFile)
		userAPI.POST("/containers/:name/files/mkdir", user.MakeDir)
		userAPI.POST("/containers/:name/files/delete", user.DeleteFile)
		userAPI.POST("/containers/:name/files/rename", user.RenameFile)
		// ---- 商业化模块：商品/订单/实例/工单/消息/资料 ----
		userAPI.GET("/products", user.ListProducts)
		userAPI.GET("/products/:id", user.GetProduct)
		userAPI.GET("/orders", user.ListOrders)
		userAPI.POST("/orders", user.CreateOrder)
		userAPI.POST("/orders/:id/pay", user.PayOrder)
		userAPI.POST("/orders/:id/cancel", user.CancelOrder)
		userAPI.GET("/instances", user.ListInstances)
		userAPI.GET("/instances/:id", user.GetInstance)
		userAPI.POST("/instances/:id/renew", user.RenewInstance)
		userAPI.DELETE("/instances/:id", user.DeleteInstance)
		userAPI.GET("/tickets", user.ListTickets)
		userAPI.POST("/tickets", user.CreateTicket)
		userAPI.GET("/tickets/:id", user.GetTicket)
		userAPI.POST("/tickets/:id/reply", user.ReplyTicket)
		userAPI.POST("/tickets/:id/close", user.CloseTicket)
		userAPI.GET("/notifications", user.ListNotifications)
		userAPI.POST("/notifications/:id/read", user.MarkNotificationRead)
		userAPI.POST("/notifications/read-all", user.MarkAllNotificationsRead)
		userAPI.DELETE("/notifications/:id", user.DeleteNotification)
		userAPI.POST("/change-password", user.ChangePassword)
		userAPI.POST("/profile", user.UpdateProfile)
	}

	r.GET("/api/public/brand-settings", public.GetBrandSettings)
	r.GET("/api/public/brand", public.GetBrandSettings) // 静态前端使用的别名
	r.GET("/api/public/ip-pool-settings", admin.GetIPPoolSettingsPublic)
	// ---- 公开商业化接口 ----
	r.POST("/api/public/register", public.Register)
	r.GET("/api/public/activate", public.Activate)
	r.POST("/api/public/forgot", public.ForgotPassword)
	r.POST("/api/public/reset", public.ResetPassword)
	r.GET("/api/public/site-info", public.SiteInfo)
	r.POST("/api/public/pay/notify", public.PayNotify)
	r.GET("/api/public/pay/return", public.PayReturn)

	r.GET("/api/container/captcha", container.GetCaptcha)
	r.POST("/api/container/verify", container.VerifyAccess)
	
	containerAPI := r.Group("/api/container")
	containerAPI.Use(middleware.ContainerAuth())
	{
		containerAPI.GET("/info", container.GetInfo)
		containerAPI.GET("/templates", container.GetTemplateList)
		containerAPI.POST("/action", container.Action)
		containerAPI.GET("/dns", container.GetDNS)
		containerAPI.PUT("/dns", container.SetDNS)
		containerAPI.POST("/port-mapping/allocate", container.AllocatePortMapping)
		containerAPI.POST("/port-mapping/release", container.ReleasePortMapping)
		containerAPI.GET("/port-mapping", container.ListPortMappings)
		containerAPI.GET("/ip", container.GetIP)
		containerAPI.POST("/ip/allocate", container.AllocateIP)
		containerAPI.POST("/ip/release", container.ReleaseIP)
		containerAPI.POST("/console/create-token", console.CreateToken)
		// ---- 容器文件管理 ----
		containerAPI.GET("/files", container.ListFiles)
		containerAPI.GET("/files/download", container.DownloadFile)
		containerAPI.POST("/files/upload", container.UploadFile)
		containerAPI.POST("/files/mkdir", container.MakeDir)
		containerAPI.POST("/files/delete", container.DeleteFile)
		containerAPI.POST("/files/rename", container.RenameFile)
	}

	addr := fmt.Sprintf("%s:%d", cfg.System.Server.Host, cfg.System.Server.Port)
	
	if cfg.System.Server.TLS.Enabled {
		startWithTLS(r, addr, cfg)
	} else {
		logger.Info("服务启动 (HTTP)，监听: %s", addr)
		if err := r.Run(addr); err != nil {
			logger.Error("服务启动失败: %v", err)
		}
	}
}

func startWithTLS(r *gin.Engine, addr string, cfg *core.Config) {
	tlsCfg := cfg.System.Server.TLS
	certMgr := tlsManager.NewCertificateManager(tlsCfg.CertFile, tlsCfg.KeyFile)
	
	certFile := tlsCfg.CertFile
	keyFile := tlsCfg.KeyFile
	certSource := ""

	brandService := service.NewBrandService()
	certContent, keyContent, err := brandService.LoadTLSCertificates()
	
	if err == nil && certContent != "" && keyContent != "" {
		logger.Info("检测到Web后台配置的证书，优先使用...")
		if err := brandService.SaveTLSCertificates(certContent, keyContent); err != nil {
			logger.Error("写入Web配置的证书失败: %v", err)
		} else {
			certSource = "自定义"
			logger.OK("已加载自定义证书")
		}
	}
	
	if certSource == "" && tlsCfg.AutoGenerate {
		generateSelfSignedCert(certMgr)
		certSource = "自签名"
	}
	
	if certSource == "" {
		log.Fatalf("无可用证书，无法启动HTTPS服务")
	}
	
	logger.Info("证书来源: %s", certSource)
	logger.Info("服务启动 (HTTPS)，监听: %s", addr)
	logger.Info("TLS证书: %s", certFile)
	logger.Info("TLS私钥: %s", keyFile)
	
	if err := r.RunTLS(addr, certFile, keyFile); err != nil {
		logger.Error("HTTPS服务器启动失败: %v", err)
	}
}

func generateSelfSignedCert(certMgr *tlsManager.CertificateManager) {
	if !certMgr.CertificateExists() {
		logger.Info("证书文件不存在，开始生成自签名证书...")
		opts := tlsManager.GenerateOptions{
			Organization:  "lxdapi",
			Country:       "US",
			Province:      "State",
			Locality:      "City",
			ServerIPs:     []string{"127.0.0.1"},
			ServerDomains: []string{"localhost"},
			ValidityDays:  3650,
		}
		
		if err := certMgr.GenerateSelfSignedCert(opts); err != nil {
			logger.Error("生成自签名证书失败: %v", err)
			log.Fatalf("生成自签名证书失败: %v", err)
		}
		
		logger.OK("自签名证书生成成功 (有效期10年)")
	} else if !certMgr.ValidateCertificate() {
		logger.Warn("现有证书无效或已过期，重新生成...")
		opts := tlsManager.GenerateOptions{
			Organization:  "lxdapi",
			Country:       "US",
			Province:      "State",
			Locality:      "City",
			ServerIPs:     []string{"127.0.0.1"},
			ServerDomains: []string{"localhost"},
			ValidityDays:  3650,
		}
		
		if err := certMgr.GenerateSelfSignedCert(opts); err != nil {
			logger.Error("重新生成证书失败: %v", err)
			log.Fatalf("重新生成证书失败: %v", err)
		}
		
		logger.OK("证书重新生成成功 (有效期10年)")
	} else {
		logger.OK("使用现有证书")
	}
}


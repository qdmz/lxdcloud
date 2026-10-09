package admin

import (
	"fmt"
	"strings"

	"github.com/gin-gonic/gin"
	"lxdapi/internal/service"
	"lxdapi/models"
	"lxdapi/pkg/logger"
	"lxdapi/pkg/response"
)

type CreateUserRequest struct {
	Username           string `json:"username" binding:"required"`
	Email              string `json:"email"`
	Password           string `json:"password"`
	Status             string `json:"status"`
	Remark             string `json:"remark"`
	CPU                int    `json:"cpu"`    // 前端别名 → cpu_quota
	Memory             int    `json:"memory"` // 前端别名 → memory_quota
	Disk               int    `json:"disk"`   // 前端别名 → disk_quota
	CPUQuota           int    `json:"cpu_quota"`
	MemoryQuota        int    `json:"memory_quota"`
	DiskQuota          int    `json:"disk_quota"`
	MaxCPUPerContainer int    `json:"max_cpu_per_container"`
	TrafficLimit       int    `json:"traffic_limit"`
	IPv4PoolLimit      int    `json:"ipv4_pool_limit"`
	IPv4MappingLimit   int    `json:"ipv4_mapping_limit"`
	IPv6PoolLimit      int    `json:"ipv6_pool_limit"`
	IPv6MappingLimit   int    `json:"ipv6_mapping_limit"`
	ReverseProxyLimit  int    `json:"reverse_proxy_limit"`
	Ingress            int    `json:"ingress"`
	Egress             int    `json:"egress"`
	CPUAllowance       int    `json:"cpu_allowance"`
	IORead             int    `json:"io_read"`
	IOWrite            int    `json:"io_write"`
	ProcessesLimit     int    `json:"processes_limit"`
	AllowNesting       bool   `json:"allow_nesting"`
	MemorySwap         bool   `json:"memory_swap"`
}

// CreateUser 创建用户
// @Summary 创建用户
// @Description 创建新用户并生成密码
// @Tags Admin API - 用户管理
// @Accept json
// @Produce json
// @Param request body CreateUserRequest true "用户信息"
// @Success 200 {object} response.Response "创建成功"
// @Failure 400 {object} response.Response "参数错误"
// @Failure 500 {object} response.Response "创建失败"
// @Security SessionAuth
// @Router /api/admin/users [post]
func CreateUser(c *gin.Context) {
	var req CreateUserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, 400, "参数错误: "+err.Error())
		return
	}

	if req.CPUQuota == 0 && req.CPU > 0 {
		req.CPUQuota = req.CPU
	}
	if req.MemoryQuota == 0 && req.Memory > 0 {
		req.MemoryQuota = req.Memory
	}
	if req.DiskQuota == 0 && req.Disk > 0 {
		req.DiskQuota = req.Disk
	}

	user, err := service.CreateUser(req.Username, req.Email, "", req.CPUQuota, req.MemoryQuota, req.DiskQuota,
		req.MaxCPUPerContainer, req.TrafficLimit, req.IPv4PoolLimit, req.IPv4MappingLimit,
		req.IPv6PoolLimit, req.IPv6MappingLimit, req.ReverseProxyLimit,
		req.Ingress, req.Egress, req.CPUAllowance, req.IORead, req.IOWrite, req.ProcessesLimit,
		req.AllowNesting, req.MemorySwap)
	if err != nil {
		logger.Error("创建用户失败: %v", err)
		response.Error(c, 500, err.Error())
		return
	}

	// 管理员指定了初始密码：写入 bcrypt 密码哈希，用户可直接用该密码登录用户中心
	extra := map[string]interface{}{}
	if req.Password != "" {
		hash, herr := service.HashPassword(req.Password)
		if herr != nil {
			response.Error(c, 500, "密码处理失败")
			return
		}
		extra["password_hash"] = hash
		extra["email_verified"] = true
	}
	if st := normalizeUserStatus(req.Status); st == "disabled" {
		extra["status"] = st
	}
	if len(extra) > 0 {
		if err := service.UpdateUser(fmt.Sprint(user.ID), extra); err != nil {
			logger.Error("设置新用户密码/状态失败: %v", err)
		}
	}

	password := user.APIKey
	if req.Password != "" {
		password = req.Password
	}
	logger.OK("创建用户成功: %s", user.Username)
	response.Success(c, gin.H{
		"user":     user,
		"password": password,
	})
}

// GetUsers 获取用户列表或详情（统一接口）
// @Summary 获取用户列表或详情
// @Description 获取用户列表，如果提供id参数则返回用户详情
// @Tags Admin API - 用户管理
// @Accept json
// @Produce json
// @Param id query string false "用户ID（可选，提供则返回详情）"
// @Success 200 {object} response.Response "获取成功"
// @Failure 404 {object} response.Response "用户不存在"
// @Failure 500 {object} response.Response "获取失败"
// @Security SessionAuth
// @Router /api/admin/users [get]
func GetUsers(c *gin.Context) {
	userID := c.Query("id")

	if userID != "" {
		user, containers, err := service.GetUserWithContainers(userID)
		if err != nil {
			logger.Error("获取用户失败: %v", err)
			response.Error(c, 404, err.Error())
			return
		}
		
		stats := service.GetUserFullStats(user.Username)

		response.Success(c, gin.H{
			"user":       user,
			"containers": containers,
			"total":      len(containers),
			"stats":      stats,
		})
		return
	}

	users, err := service.ListUsers()
	if err != nil {
		logger.Error("获取用户列表失败: %v", err)
		response.Error(c, 500, err.Error())
		return
	}

	type UserWithCount struct {
		*models.User
		ContainerCount int64 `json:"container_count"`
		UsedCPU        int   `json:"used_cpu"`
		UsedMemory     int   `json:"used_memory"`
		UsedDisk       int   `json:"used_disk"`
	}
	
	usersWithCount := make([]UserWithCount, len(users))
	for i, user := range users {
		count, usedCPU, usedMemory, usedDisk := service.GetUserContainerStats(user.Username)
		usersWithCount[i] = UserWithCount{
			User:           &user,
			ContainerCount: count,
			UsedCPU:        usedCPU,
			UsedMemory:     usedMemory,
			UsedDisk:       usedDisk,
		}
	}

	response.Success(c, gin.H{
		"users": usersWithCount,
		"total": len(users),
	})
}

type UpdateUserRequest struct {
	Status             *string `json:"status"`
	Password           *string `json:"password"`
	Remark             *string `json:"remark"`
	CPU                *int    `json:"cpu"`    // 前端别名 → cpu_quota
	Memory             *int    `json:"memory"` // 前端别名 → memory_quota
	Disk               *int    `json:"disk"`   // 前端别名 → disk_quota
	CPUQuota           *int    `json:"cpu_quota"`
	MaxCPUPerContainer *int    `json:"max_cpu_per_container"`
	MemoryQuota        *int    `json:"memory_quota"`
	DiskQuota          *int    `json:"disk_quota"`
	TrafficLimit       *int    `json:"traffic_limit"`
	IPv4PoolLimit      *int    `json:"ipv4_pool_limit"`
	IPv4MappingLimit   *int    `json:"ipv4_mapping_limit"`
	IPv6PoolLimit      *int    `json:"ipv6_pool_limit"`
	IPv6MappingLimit   *int    `json:"ipv6_mapping_limit"`
	ReverseProxyLimit  *int    `json:"reverse_proxy_limit"`
	Ingress            *int    `json:"ingress"`
	Egress             *int    `json:"egress"`
	CPUAllowance       *int    `json:"cpu_allowance"`
	IORead             *int    `json:"io_read"`
	IOWrite            *int    `json:"io_write"`
	ProcessesLimit     *int    `json:"processes_limit"`
	AllowNesting       *bool   `json:"allow_nesting"`
	MemorySwap         *bool   `json:"memory_swap"`
}

// UpdateUser 更新用户
// @Summary 更新用户
// @Description 更新用户配额和状态
// @Tags Admin API - 用户管理
// @Accept json
// @Produce json
// @Param id path string true "用户ID"
// @Param request body UpdateUserRequest true "更新参数"
// @Success 200 {object} response.Response "更新成功"
// @Failure 400 {object} response.Response "参数错误"
// @Failure 500 {object} response.Response "更新失败"
// @Security SessionAuth
// @Router /api/admin/users/:id [put]
func UpdateUser(c *gin.Context) {
	userID := c.Param("id")
	if userID == "" {
		response.Error(c, 400, "缺少用户ID")
		return
	}

	var req UpdateUserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, 400, "参数错误: "+err.Error())
		return
	}

	if req.CPUQuota == nil && req.CPU != nil {
		req.CPUQuota = req.CPU
	}
	if req.MemoryQuota == nil && req.Memory != nil {
		req.MemoryQuota = req.Memory
	}
	if req.DiskQuota == nil && req.Disk != nil {
		req.DiskQuota = req.Disk
	}

	updates := make(map[string]interface{})
	if req.Status != nil {
		if st := normalizeUserStatus(*req.Status); st != "" {
			updates["status"] = st
		}
	}
	if req.Password != nil && *req.Password != "" {
		hash, err := service.HashPassword(*req.Password)
		if err != nil {
			response.Error(c, 500, "密码处理失败")
			return
		}
		updates["password_hash"] = hash
	}
	if req.CPUQuota != nil {
		updates["cpu_quota"] = *req.CPUQuota
	}
	if req.MaxCPUPerContainer != nil {
		updates["max_cpu_per_container"] = *req.MaxCPUPerContainer
	}
	if req.MemoryQuota != nil {
		updates["memory_quota"] = *req.MemoryQuota
	}
	if req.DiskQuota != nil {
		updates["disk_quota"] = *req.DiskQuota
	}
	if req.TrafficLimit != nil {
		updates["traffic_limit"] = *req.TrafficLimit
	}
	if req.IPv4PoolLimit != nil {
		updates["ipv4_pool_limit"] = *req.IPv4PoolLimit
	}
	if req.IPv4MappingLimit != nil {
		updates["ipv4_mapping_limit"] = *req.IPv4MappingLimit
	}
	if req.IPv6PoolLimit != nil {
		updates["ipv6_pool_limit"] = *req.IPv6PoolLimit
	}
	if req.IPv6MappingLimit != nil {
		updates["ipv6_mapping_limit"] = *req.IPv6MappingLimit
	}
	if req.ReverseProxyLimit != nil {
		updates["reverse_proxy_limit"] = *req.ReverseProxyLimit
	}
	if req.Ingress != nil {
		updates["ingress"] = *req.Ingress
	}
	if req.Egress != nil {
		updates["egress"] = *req.Egress
	}
	if req.CPUAllowance != nil {
		updates["cpu_allowance"] = *req.CPUAllowance
	}
	if req.IORead != nil {
		updates["io_read"] = *req.IORead
	}
	if req.IOWrite != nil {
		updates["io_write"] = *req.IOWrite
	}
	if req.ProcessesLimit != nil {
		updates["processes_limit"] = *req.ProcessesLimit
	}
	if req.AllowNesting != nil {
		updates["allow_nesting"] = *req.AllowNesting
	}
	if req.MemorySwap != nil {
		updates["memory_swap"] = *req.MemorySwap
	}

	if len(updates) == 0 {
		response.Error(c, 400, "没有要更新的字段")
		return
	}

	if err := service.UpdateUser(userID, updates); err != nil {
		logger.Error("更新用户失败: %v", err)
		response.Error(c, 500, err.Error())
		return
	}

	logger.OK("更新用户成功: %s", userID)
	response.Success(c, nil)
}

// GetUser 获取单个用户详情（路径参数版，供前端 GET /api/admin/users/:id 使用）
// @Router /api/admin/users/:id [get]
func GetUser(c *gin.Context) {
	userID := c.Param("id")
	user, containers, err := service.GetUserWithContainers(userID)
	if err != nil {
		response.Error(c, 404, err.Error())
		return
	}
	response.Success(c, gin.H{
		"user":       user,
		"containers": containers,
		"total":      len(containers),
		"stats":      service.GetUserFullStats(user.Username),
	})
}

// normalizeUserStatus 统一前端状态值：enabled/active/normal → active，disabled → disabled，pending → pending
func normalizeUserStatus(s string) string {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "enabled", "active", "normal", "1":
		return "active"
	case "disabled", "0":
		return "disabled"
	case "pending":
		return "pending"
	}
	return ""
}

// DeleteUser 删除用户
// @Summary 删除用户
// @Description 删除指定用户
// @Tags Admin API - 用户管理
// @Accept json
// @Produce json
// @Param id path string true "用户ID"
// @Success 200 {object} response.Response "删除成功"
// @Failure 400 {object} response.Response "缺少参数"
// @Failure 500 {object} response.Response "删除失败"
// @Security SessionAuth
// @Router /api/admin/users/:id [delete]
func DeleteUser(c *gin.Context) {
	userID := c.Param("id")
	if userID == "" {
		userID = c.Query("user_id")
	}
	if userID == "" {
		response.Error(c, 400, "缺少用户ID")
		return
	}

	if err := service.DeleteUser(userID); err != nil {
		logger.Error("删除用户失败: %v", err)
		response.Error(c, 500, err.Error())
		return
	}

	logger.OK("删除用户成功: %s", userID)
	response.Success(c, nil)
}

func BatchDeleteUsers(c *gin.Context) {
	var req struct {
		UserIDs []string `json:"user_ids" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, 400, "参数错误")
		return
	}

	if len(req.UserIDs) == 0 {
		response.Error(c, 400, "请选择要删除的用户")
		return
	}

	var deleted, skipped int
	for _, userID := range req.UserIDs {
		if err := service.DeleteUser(userID); err == nil {
			deleted++
		} else {
			skipped++
		}
	}

	logger.OK("批量删除用户: 成功%d, 跳过%d", deleted, skipped)
	response.Success(c, gin.H{"deleted": deleted, "skipped": skipped})
}

// RegenerateAPIKey 重新生成密码
// @Summary 重新生成密码
// @Description 重新生成用户的密码
// @Tags Admin API - 用户管理
// @Accept json
// @Produce json
// @Param id path string true "用户ID"
// @Success 200 {object} response.Response "生成成功"
// @Failure 400 {object} response.Response "缺少参数"
// @Failure 500 {object} response.Response "生成失败"
// @Security SessionAuth
// @Router /api/admin/users/:id/regenerate-key [post]
func RegenerateAPIKey(c *gin.Context) {
	userID := c.Param("id")
	if userID == "" {
		userID = c.Query("user_id")
	}
	if userID == "" {
		response.Error(c, 400, "缺少用户ID")
		return
	}

	user, err := service.RegenerateAPIKey(userID, "")
	if err != nil {
		logger.Error("重新生成密码失败: %v", err)
		response.Error(c, 500, err.Error())
		return
	}

	logger.OK("重新生成密码成功: %s", userID)
	response.Success(c, gin.H{
		"user":     user,
		"password": user.APIKey,
	})
}

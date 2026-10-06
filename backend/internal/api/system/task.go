package system

import (
	"github.com/gin-gonic/gin"
	"lxdapi/internal/db"
	"lxdapi/models"
	"lxdapi/pkg/response"
	"strconv"
)

// GetTask 获取任务详情
// @Summary 获取任务详情
// @Description 获取指定任务的详细信息
// @Tags System API - 任务管理
// @Accept json
// @Produce json
// @Param id query string true "任务ID"
// @Success 200 {object} response.Response "获取成功"
// @Failure 400 {object} response.Response "参数错误"
// @Failure 404 {object} response.Response "任务不存在"
// @Security ApiKeyAuth
// @Router /api/system/tasks/detail [get]
func GetTask(c *gin.Context) {
	idStr := c.Query("id")
	if idStr == "" {
		response.Error(c, 400, "缺少任务ID")
		return
	}
	
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		response.Error(c, 400, "任务ID格式错误")
		return
	}
	
	var task models.Task
	if err := db.DB.First(&task, id).Error; err != nil {
		response.Error(c, 404, "任务不存在")
		return
	}
	
	response.Success(c, task)
}

// ListTasks 获取任务列表
// @Summary 获取任务列表
// @Description 获取任务列表，可按容器过滤
// @Tags System API - 任务管理
// @Accept json
// @Produce json
// @Param name query string false "容器名称（可选）"
// @Success 200 {object} response.Response "获取成功"
// @Failure 500 {object} response.Response "获取失败"
// @Security ApiKeyAuth
// @Router /api/system/tasks [get]
func ListTasks(c *gin.Context) {
	containerName := c.Query("name")
	
	query := db.DB.Model(&models.Task{})
	if containerName != "" {
		query = query.Where("container_name = ?", containerName)
	}
	
	var tasks []models.Task
	if err := query.Order("id desc").Find(&tasks).Error; err != nil {
		response.Error(c, 500, err.Error())
		return
	}
	
	response.Success(c, tasks)
}


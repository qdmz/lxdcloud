package user

import (
	"github.com/gin-gonic/gin"
	"lxdapi/internal/db"
	"lxdapi/models"
	"lxdapi/pkg/response"
)

// GetTask 获取任务状态
// @Summary 获取任务状态
// @Description 获取指定任务的状态（只能查询自己的任务）
// @Tags User API - 任务管理
// @Accept json
// @Produce json
// @Param id path string true "任务ID"
// @Success 200 {object} response.Response "获取成功"
// @Failure 403 {object} response.Response "无权访问"
// @Failure 404 {object} response.Response "任务不存在"
// @Security UserSession
// @Router /api/user/tasks/:id [get]
func GetTask(c *gin.Context) {
	taskID := c.Param("id")
	if taskID == "" {
		response.Error(c, 400, "缺少任务ID")
		return
	}

	username, _ := c.Get("username")

	var task models.Task
	if err := db.DB.Where("id = ?", taskID).First(&task).Error; err != nil {
		response.Error(c, 404, "任务不存在")
		return
	}

	if task.UserID != username.(string) {
		response.Error(c, 403, "无权访问该任务")
		return
	}

	response.Success(c, gin.H{
		"id":        task.ID,
		"status":    task.Status,
		"action":    task.Action,
		"error_msg": task.ErrorMsg,
	})
}

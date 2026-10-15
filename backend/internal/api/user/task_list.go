package user

import (
	"github.com/gin-gonic/gin"
	"lxdapi/internal/db"
	"lxdapi/models"
	"lxdapi/pkg/response"
)

// ListTasks 获取当前用户的任务日志（按所属容器过滤）
func ListTasks(c *gin.Context) {
	username := c.GetString("username")
	if username == "" {
		response.Error(c, 401, "未授权")
		return
	}

	var containers []models.Container
	db.DB.Where("user_id = ?", username).Find(&containers)
	names := make([]string, 0, len(containers))
	for _, cc := range containers {
		names = append(names, cc.Name)
	}

	var tasks []models.Task
	q := db.DB.Order("id DESC")
	if len(names) > 0 {
		q = q.Where("container_name IN ? OR user_id = ?", names, username)
	} else {
		q = q.Where("user_id = ?", username)
	}
	if err := q.Find(&tasks).Error; err != nil {
		response.Error(c, 500, "获取任务列表失败")
		return
	}

	out := make([]gin.H, 0, len(tasks))
	for _, t := range tasks {
		out = append(out, gin.H{
			"id":             t.ID,
			"container_name": t.ContainerName,
			"action":         t.Action,
			"type":           t.Type,
			"status":         t.Status,
			"error_msg":      t.ErrorMsg,
			"duration":       t.Duration,
			"created_at":     t.CreatedAt,
			"updated_at":     t.UpdatedAt,
			"completed_at":   t.CompletedAt,
		})
	}
	response.Success(c, out)
}

package executor

import (
	"lxdapi/internal/db"
	"lxdapi/models"
	"lxdapi/pkg/logger"
)

func ClearPendingTasks() {
	result := db.DB.Model(&models.Task{}).
		Where("status IN ?", []string{models.TaskQueued, models.TaskRunning}).
		Updates(map[string]interface{}{
			"status":    models.TaskFailed,
			"error_msg": "服务重启，任务已清理",
		})
	
	if result.RowsAffected > 0 {
		logger.Warn("启动时清理了 %d 个未完成任务", result.RowsAffected)
	} else {
		logger.Info("无需清理的任务")
	}
}

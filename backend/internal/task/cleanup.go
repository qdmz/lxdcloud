package task

import (
	"time"

	"lxdapi/internal/core"
	"lxdapi/internal/db"
	"lxdapi/pkg/logger"
	"lxdapi/models"
)

func StartAutoCleanup() {
	cfg := core.GlobalConfig
	if cfg == nil {
		return
	}
	
	if !cfg.Task.Enabled || cfg.Task.AutoCleanupDays <= 0 {
		return
	}

	logger.OK("任务自动清理已启动，保留最近 %d 天", cfg.Task.AutoCleanupDays)

	go func() {
		for {
			now := time.Now()
			next := time.Date(now.Year(), now.Month(), now.Day()+1, 2, 0, 0, 0, now.Location())
			duration := next.Sub(now)

			time.Sleep(duration)
			CleanupOldTasks(cfg.Task.AutoCleanupDays)
		}
	}()
}

func CleanupOldTasks(days int) {
	if days <= 0 {
		return
	}

	cutoffTime := time.Now().AddDate(0, 0, -days)

	result := db.DB.Where("created_at < ? AND (status = ? OR status = ? OR status = ?)", 
		cutoffTime, "completed", "success", "failed").
		Delete(&models.Task{})

	if result.Error != nil {
		logger.Error("任务清理失败: %v", result.Error)
		return
	}

	if result.RowsAffected > 0 {
		logger.Info("任务清理完成，删除了 %d 条记录", result.RowsAffected)
	}
}

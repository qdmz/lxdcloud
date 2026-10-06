package task

import (
	"context"
	"encoding/json"
	"time"

	"lxdapi/internal/cache"
	"lxdapi/internal/db"
	"lxdapi/models"
	"lxdapi/pkg/logger"

	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
)

var autoRefreshTicker *time.Ticker
var autoRefreshStop chan bool

func StartCacheAutoRefresh() {
	logger.Info("缓存自动刷新调度器已启动")
	
	autoRefreshStop = make(chan bool)
	
	go func() {
		checkTicker := time.NewTicker(1 * time.Minute)
		defer checkTicker.Stop()
		
		for {
			select {
			case <-checkTicker.C:
				checkAndRefresh()
			case <-autoRefreshStop:
				logger.Info("缓存自动刷新调度器已停止")
				return
			}
		}
	}()
}

func StopCacheAutoRefresh() {
	if autoRefreshStop != nil {
		autoRefreshStop <- true
	}
}

func checkAndRefresh() {
	var settings models.CacheAutoRefreshSettings
	result := db.DB.Session(&gorm.Session{Logger: gormlogger.Discard}).Where("id = ?", 1).First(&settings)
	if result.Error != nil {
		return
	}
	
	if !settings.Enabled {
		return
	}
	
	now := time.Now()
	nextRun := settings.LastRunTime.Add(time.Duration(settings.Interval) * time.Minute)
	
	if now.Before(nextRun) {
		return
	}
	
	logger.Info("触发自动刷新容器缓存任务")
	
	if err := db.DB.Model(&settings).Update("last_run_time", now).Error; err != nil {
		logger.Error("更新最后运行时间失败: %v", err)
		return
	}
	
	params := map[string]interface{}{
		"action":         "auto_refresh_cache",
		"batch_size":     settings.BatchSize,
		"batch_interval": settings.BatchInterval,
	}
	
	paramsJSON, _ := json.Marshal(params)
	
	task := &models.Task{
		ContainerName: "system",
		UserID:        "auto",
		Action:        "refresh_cache",
		Type:          "system",
		Status:        models.TaskQueued,
		Params:        string(paramsJSON),
	}
	
	if err := db.DB.Create(task).Error; err != nil {
		logger.Error("创建自动刷新任务失败: %v", err)
		return
	}
	
	logger.Info("自动刷新任务已创建: TaskID=%d", task.ID)
	
	go func() {
		taskCtx := context.Background()
		
		task.Status = models.TaskRunning
		startTime := time.Now()
		task.StartedAt = &startTime
		db.DB.Save(task)
		
		total, success, err := cache.RefreshAllContainersCache(taskCtx)
		
		completedTime := time.Now()
		task.CompletedAt = &completedTime
		task.Duration = completedTime.Sub(startTime).Milliseconds()
		
		if err != nil {
			task.Status = models.TaskFailed
			task.ErrorMsg = err.Error()
			logger.Error("自动刷新任务失败: %v", err)
		} else {
			task.Status = models.TaskSuccess
			failed := total - success
			result := map[string]interface{}{
				"total":   total,
				"success": success,
				"failed":  failed,
			}
			resultJSON, _ := json.Marshal(result)
			task.Result = string(resultJSON)
			logger.OK("自动刷新任务完成: 总数=%d, 成功=%d, 失败=%d", total, success, failed)
		}
		
		db.DB.Save(task)
	}()
}

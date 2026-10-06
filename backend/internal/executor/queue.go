package executor

import (
	"context"
	"encoding/json"
	"fmt"
	"lxdapi/internal/core"
	"lxdapi/internal/db"
	"lxdapi/models"
	"lxdapi/pkg/logger"
	"strings"
	"sync"
	"time"
)

type TaskFunc func(ctx context.Context) error

type taskIDKey struct{}

type Task struct {
	ID            uint
	ContainerName string
	Action        string
	UserID        string
	Params        map[string]interface{}
	Func          TaskFunc
}

type Queue struct {
	tasks   chan *Task
	workers int
	wg      sync.WaitGroup
	ctx     context.Context
	cancel  context.CancelFunc
}

var GlobalQueue *Queue

func InitQueue() error {
	cfg := core.GlobalConfig.Task
	if !cfg.Enabled {
		logger.Info("异步任务队列未启用")
		return nil
	}
	
	if cfg.Backend == "redis" {
		return InitRedisQueue()
	}
	
	ctx, cancel := context.WithCancel(context.Background())
	
	GlobalQueue = &Queue{
		tasks:   make(chan *Task, cfg.QueueSize),
		workers: cfg.Workers,
		ctx:     ctx,
		cancel:  cancel,
	}
	
	GlobalQueue.Start()
	logger.OK("异步任务队列初始化成功，Worker数: %d", cfg.Workers)
	return nil
}

func (q *Queue) Start() {
	for i := 0; i < q.workers; i++ {
		q.wg.Add(1)
		go q.worker(i)
	}
}

func (q *Queue) Stop() {
	if q == nil {
		return
	}
	q.cancel()
	close(q.tasks)
	q.wg.Wait()
	logger.Info("异步任务队列已停止")
}

func (q *Queue) Submit(task *Task) error {
	if q == nil {
		if GlobalRedisQueue != nil {
			return GlobalRedisQueue.Submit(task)
		}
		return task.Func(context.Background())
	}
	
	select {
	case q.tasks <- task:
		logger.Info("任务已提交: %s - %s", task.ContainerName, task.Action)
		return nil
	default:
		logger.Error("任务队列已满")
		return fmt.Errorf("任务队列已满")
	}
}

func (q *Queue) worker(id int) {
	defer q.wg.Done()
	
	logger.Info("Worker %d 已启动", id)
	
	for {
		select {
		case <-q.ctx.Done():
			return
		case task, ok := <-q.tasks:
			if !ok {
				return
			}
			q.executeTask(task)
		}
	}
}

func (q *Queue) executeTask(task *Task) {
	now := time.Now()
	
	var logs []string
	logs = append(logs, fmt.Sprintf("[%s] 任务开始执行", now.Format("2006-01-02 15:04:05")))
	logs = append(logs, fmt.Sprintf("容器: %s, 操作: %s", task.ContainerName, task.Action))
	
	db.DB.Model(&models.Task{}).Where("id = ?", task.ID).Updates(map[string]interface{}{
		"status":     models.TaskRunning,
		"started_at": &now,
		"logs":       strings.Join(logs, "\n"),
	})
	
	timeout := time.Duration(core.GlobalConfig.Task.Timeout) * time.Second
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	ctx = context.WithValue(ctx, taskIDKey{}, task.ID)
	defer cancel()
	
	err := task.Func(ctx)
	
	completed := time.Now()
	duration := completed.Sub(now).Milliseconds()
	
	logs = append(logs, fmt.Sprintf("[%s] 任务执行完成", completed.Format("2006-01-02 15:04:05")))
	logs = append(logs, fmt.Sprintf("耗时: %dms", duration))
	
	if err != nil {
		logs = append(logs, fmt.Sprintf("执行失败: %v", err))
		db.DB.Model(&models.Task{}).Where("id = ?", task.ID).Updates(map[string]interface{}{
			"status":       models.TaskFailed,
			"error_msg":    err.Error(),
			"completed_at": &completed,
			"duration":     duration,
			"logs":         strings.Join(logs, "\n"),
		})
		logger.Error("任务执行失败: %s - %s, 错误: %v", task.ContainerName, task.Action, err)
	} else {
		logs = append(logs, "执行成功")
		db.DB.Model(&models.Task{}).Where("id = ?", task.ID).Updates(map[string]interface{}{
			"status":       models.TaskSuccess,
			"result":       "操作执行成功",
			"completed_at": &completed,
			"duration":     duration,
			"logs":         strings.Join(logs, "\n"),
		})
		logger.OK("任务执行成功: %s - %s", task.ContainerName, task.Action)
	}
}

func CreateTask(containerName, action, userID string, params map[string]interface{}, fn TaskFunc) (*models.Task, error) {
	if action != "delete" {
		var count int64
		db.DB.Model(&models.Task{}).
			Where("container_name = ? AND status IN ?", containerName, []string{models.TaskQueued, models.TaskRunning}).
			Count(&count)
		
		if count > 0 {
			return nil, fmt.Errorf("容器正在执行任务，请稍后再试")
		}
	}
	
	var paramsJSON string
	if params != nil {
		data, _ := json.Marshal(params)
		paramsJSON = string(data)
	}
	
	task := &models.Task{
		ContainerName: containerName,
		Action:        action,
		UserID:        userID,
		Type:          "async",
		Status:        models.TaskQueued,
		Params:        paramsJSON,
	}
	
	if err := db.DB.Create(task).Error; err != nil {
		return nil, err
	}
	
	queueTask := &Task{
		ID:            task.ID,
		ContainerName: containerName,
		Action:        action,
		UserID:        userID,
		Params:        params,
		Func:          fn,
	}
	
	if err := GlobalQueue.Submit(queueTask); err != nil {
		return nil, err
	}
	
	return task, nil
}

func CreateSyncTask(containerName, action, userID string, fn TaskFunc) error {
	var count int64
	db.DB.Model(&models.Task{}).
		Where("container_name = ? AND status IN ?", containerName, []string{models.TaskQueued, models.TaskRunning}).
		Count(&count)
	
	if count > 0 {
		return fmt.Errorf("容器正在执行任务，请稍后再试")
	}
	
	now := time.Now()
	
	task := &models.Task{
		ContainerName: containerName,
		Action:        action,
		UserID:        userID,
		Type:          "sync",
		Status:        models.TaskRunning,
		StartedAt:     &now,
	}
	
	if err := db.DB.Create(task).Error; err != nil {
		return err
	}
	
	var logs []string
	logs = append(logs, fmt.Sprintf("[%s] 任务开始执行", now.Format("2006-01-02 15:04:05")))
	logs = append(logs, fmt.Sprintf("容器: %s, 操作: %s", containerName, action))
	
	ctx := context.Background()
	err := fn(ctx)
	
	completed := time.Now()
	duration := completed.Sub(now).Milliseconds()
	
	logs = append(logs, fmt.Sprintf("[%s] 任务执行完成", completed.Format("2006-01-02 15:04:05")))
	logs = append(logs, fmt.Sprintf("耗时: %dms", duration))
	
	if err != nil {
		logs = append(logs, fmt.Sprintf("执行失败: %v", err))
		db.DB.Model(&models.Task{}).Where("id = ?", task.ID).Updates(map[string]interface{}{
			"status":       models.TaskFailed,
			"error_msg":    err.Error(),
			"completed_at": &completed,
			"duration":     duration,
			"logs":         strings.Join(logs, "\n"),
		})
		return err
	}
	
	db.DB.Model(&models.Task{}).Where("id = ?", task.ID).Updates(map[string]interface{}{
		"status":       models.TaskSuccess,
		"completed_at": &completed,
		"duration":     duration,
		"logs":         strings.Join(logs, "\n"),
	})
	
	return nil
}

func UpdateTaskResult(ctx context.Context, result string) {
	taskID, ok := ctx.Value(taskIDKey{}).(uint)
	if !ok || taskID == 0 {
		return
	}
	db.DB.Model(&models.Task{}).Where("id = ?", taskID).Update("result", result)
}


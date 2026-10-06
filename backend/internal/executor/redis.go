package executor

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/redis/go-redis/v9"
	"lxdapi/internal/core"
	"lxdapi/internal/db"
	"lxdapi/models"
	"lxdapi/pkg/logger"
	"sync"
	"time"
)

type RedisQueue struct {
	client  *redis.Client
	workers int
	wg      sync.WaitGroup
	ctx     context.Context
	cancel  context.CancelFunc
}

type RedisTask struct {
	ID            uint   `json:"id"`
	ContainerName string `json:"container_name"`
	Action        string `json:"action"`
	UserID        string `json:"user_id"`
}

func InitRedisQueue() error {
	cfg := core.GlobalConfig.Task
	if !cfg.Enabled {
		logger.Info("异步任务队列未启用")
		return nil
	}

	if cfg.Backend != "redis" {
		return nil
	}

	rdb := redis.NewClient(&redis.Options{
		Addr:     fmt.Sprintf("%s:%d", cfg.Redis.Host, cfg.Redis.Port),
		Password: cfg.Redis.Password,
		DB:       cfg.Redis.DB,
	})

	ctx := context.Background()
	if err := rdb.Ping(ctx).Err(); err != nil {
		return fmt.Errorf("Redis连接失败: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())

	GlobalRedisQueue = &RedisQueue{
		client:  rdb,
		workers: cfg.Workers,
		ctx:     ctx,
		cancel:  cancel,
	}

	GlobalRedisQueue.Start()
	logger.OK("Redis任务队列初始化成功，Worker数: %d", cfg.Workers)
	return nil
}

var GlobalRedisQueue *RedisQueue

func (q *RedisQueue) Start() {
	for i := 0; i < q.workers; i++ {
		q.wg.Add(1)
		go q.worker(i)
	}
}

func (q *RedisQueue) Stop() {
	if q == nil {
		return
	}
	q.cancel()
	q.wg.Wait()
	q.client.Close()
	logger.Info("Redis任务队列已停止")
}

func (q *RedisQueue) Submit(task *Task) error {
	if q == nil {
		return task.Func(context.Background())
	}

	redisTask := RedisTask{
		ID:            task.ID,
		ContainerName: task.ContainerName,
		Action:        task.Action,
		UserID:        task.UserID,
	}

	data, err := json.Marshal(redisTask)
	if err != nil {
		return fmt.Errorf("序列化任务失败: %v", err)
	}

	ctx := context.Background()
	if err := q.client.LPush(ctx, "lxdapi:tasks", data).Err(); err != nil {
		return fmt.Errorf("提交任务到Redis失败: %v", err)
	}

	logger.Info("任务已提交到Redis: %s - %s", task.ContainerName, task.Action)
	return nil
}

func (q *RedisQueue) worker(id int) {
	defer q.wg.Done()

	logger.Info("Redis Worker %d 已启动", id)

	for {
		select {
		case <-q.ctx.Done():
			return
		default:
			ctx := context.Background()
			result, err := q.client.BRPop(ctx, 5*time.Second, "lxdapi:tasks").Result()
			if err != nil {
				if err != redis.Nil {
					logger.Error("从Redis获取任务失败: %v", err)
				}
				continue
			}

			if len(result) < 2 {
				continue
			}

			var redisTask RedisTask
			if err := json.Unmarshal([]byte(result[1]), &redisTask); err != nil {
				logger.Error("解析任务失败: %v", err)
				continue
			}

			q.executeTask(&redisTask)
		}
	}
}

func (q *RedisQueue) executeTask(redisTask *RedisTask) {
	now := time.Now()
	
	var dbTask models.Task
	if err := db.DB.First(&dbTask, redisTask.ID).Error; err != nil {
		logger.Error("读取任务详情失败 (ID: %d): %v", redisTask.ID, err)
		db.DB.Model(&models.Task{}).Where("id = ?", redisTask.ID).Updates(map[string]interface{}{
			"status":    models.TaskFailed,
			"error_msg": fmt.Sprintf("读取任务详情失败: %v", err),
		})
		return
	}
	
	var logs []string
	logs = append(logs, fmt.Sprintf("[%s] 任务开始执行", now.Format("2006-01-02 15:04:05")))
	logs = append(logs, fmt.Sprintf("容器: %s, 操作: %s", redisTask.ContainerName, redisTask.Action))

	db.DB.Model(&models.Task{}).Where("id = ?", redisTask.ID).Updates(map[string]interface{}{
		"status":     models.TaskRunning,
		"started_at": &now,
		"logs":       fmt.Sprintf("%s\n%s", logs[0], logs[1]),
	})

	timeout := time.Duration(core.GlobalConfig.Task.Timeout) * time.Second
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	executor, ok := taskExecutors[redisTask.Action]
	var err error
	if !ok {
		err = fmt.Errorf("未知的任务类型: %s", redisTask.Action)
	} else {
		err = executor(ctx, redisTask.ContainerName, dbTask.Params)
	}

	completed := time.Now()
	duration := completed.Sub(now).Milliseconds()
	
	logs = append(logs, fmt.Sprintf("[%s] 任务执行完成", completed.Format("2006-01-02 15:04:05")))
	logs = append(logs, fmt.Sprintf("耗时: %dms", duration))
	
	if err != nil {
		logs = append(logs, fmt.Sprintf("执行失败: %v", err))
		db.DB.Model(&models.Task{}).Where("id = ?", redisTask.ID).Updates(map[string]interface{}{
			"status":       models.TaskFailed,
			"error_msg":    err.Error(),
			"completed_at": &completed,
			"duration":     duration,
			"logs":         fmt.Sprintf("%s\n%s\n%s\n%s\n%s", logs[0], logs[1], logs[2], logs[3], logs[4]),
		})
		logger.Error("任务执行失败: %s - %s, 错误: %v", redisTask.ContainerName, redisTask.Action, err)
	} else {
		logs = append(logs, "执行成功")
		db.DB.Model(&models.Task{}).Where("id = ?", redisTask.ID).Updates(map[string]interface{}{
			"status":       models.TaskSuccess,
			"result":       "操作执行成功",
			"completed_at": &completed,
			"duration":     duration,
			"logs":         fmt.Sprintf("%s\n%s\n%s\n%s\n%s", logs[0], logs[1], logs[2], logs[3], logs[4]),
		})
		logger.OK("任务执行成功: %s - %s", redisTask.ContainerName, redisTask.Action)
	}
}

var taskExecutors = make(map[string]func(context.Context, string, string) error)

func RegisterTaskExecutor(action string, executor func(context.Context, string, string) error) {
	taskExecutors[action] = executor
}


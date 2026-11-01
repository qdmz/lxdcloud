package service

import (
	"context"
	"encoding/json"
	"fmt"
	"lxdapi/internal/db"
	"lxdapi/internal/executor"
	"lxdapi/models"
	"lxdapi/pkg/logger"
	"math/rand"
	"sort"
	"sync"
	"time"
)

// BgJob 节点本地的后台作业（镜像下载/导入、包缓存同步），同时登记到任务队列（任务管理页可见）
type BgJob struct {
	ID          string     `json:"id"`
	TaskID      uint       `json:"task_id"`
	Kind        string     `json:"kind"`  // image_download / image_import / pkgcache_sync
	Label       string     `json:"label"` // 展示名
	Status      string     `json:"status"`
	Progress    int        `json:"progress"` // 0-100，-1 表示未知
	Message     string     `json:"message"`
	Error       string     `json:"error"`
	Fingerprint string     `json:"fingerprint,omitempty"`
	Alias       string     `json:"alias,omitempty"`
	CreatedAt   time.Time  `json:"created_at"`
	FinishedAt  *time.Time `json:"finished_at,omitempty"`
}

var bgJobs = struct {
	sync.Mutex
	m map[string]*BgJob
}{m: map[string]*BgJob{}}

func newJobID() string {
	return fmt.Sprintf("%x%04x", time.Now().UnixNano()/1e6, rand.Intn(0xffff))
}

func getJob(id string) *BgJob {
	bgJobs.Lock()
	defer bgJobs.Unlock()
	return bgJobs.m[id]
}

func putJob(j *BgJob) {
	bgJobs.Lock()
	defer bgJobs.Unlock()
	bgJobs.m[j.ID] = j
	// 只保留最近 50 个
	if len(bgJobs.m) > 50 {
		var list []*BgJob
		for _, x := range bgJobs.m {
			list = append(list, x)
		}
		sort.Slice(list, func(a, b int) bool { return list[a].CreatedAt.After(list[b].CreatedAt) })
		for _, x := range list[50:] {
			if x.Status != models.TaskRunning && x.Status != models.TaskQueued {
				delete(bgJobs.m, x.ID)
			}
		}
	}
}

func updateJob(id string, fn func(j *BgJob)) {
	bgJobs.Lock()
	defer bgJobs.Unlock()
	if j := bgJobs.m[id]; j != nil {
		fn(j)
	}
}

// ListJobs 列出某类作业（kind 为空则全部），最新在前
func ListJobs(kinds ...string) []BgJob {
	bgJobs.Lock()
	defer bgJobs.Unlock()
	want := map[string]bool{}
	for _, k := range kinds {
		want[k] = true
	}
	var out []BgJob
	for _, j := range bgJobs.m {
		if len(want) == 0 || want[j.Kind] {
			out = append(out, *j)
		}
	}
	sort.Slice(out, func(a, b int) bool { return out[a].CreatedAt.After(out[b].CreatedAt) })
	return out
}

// jobRunning 是否有同类作业在进行
func jobRunning(kind string) *BgJob {
	bgJobs.Lock()
	defer bgJobs.Unlock()
	for _, j := range bgJobs.m {
		if j.Kind == kind && (j.Status == models.TaskRunning || j.Status == models.TaskQueued) {
			c := *j
			return &c
		}
	}
	return nil
}

// bgJobRunners 作业执行函数（按 kind），参数为提交时的 params JSON（含 job_id）
var bgJobRunners = map[string]func(ctx context.Context, jobID string, paramsJSON string) error{}

// RegisterBgJobExecutors 向任务队列注册执行器（redis 队列按 action 名查找执行器）
func RegisterBgJobExecutors() {
	for kind := range bgJobRunners {
		k := kind
		executor.RegisterTaskExecutor(k, func(ctx context.Context, name string, paramsJSON string) error {
			return runBgJob(k, paramsJSON)
		})
	}
}

func runBgJob(kind, paramsJSON string) error {
	var p struct {
		JobID string `json:"job_id"`
		Label string `json:"label"`
	}
	json.Unmarshal([]byte(paramsJSON), &p)
	if p.JobID == "" {
		p.JobID = newJobID()
	}
	if getJob(p.JobID) == nil {
		putJob(&BgJob{ID: p.JobID, Kind: kind, Label: p.Label, Status: models.TaskQueued, Progress: -1, CreatedAt: time.Now()})
	}
	updateJob(p.JobID, func(j *BgJob) { j.Status = models.TaskRunning })
	run := bgJobRunners[kind]
	if run == nil {
		return fmt.Errorf("未知作业类型: %s", kind)
	}
	// 镜像下载可能远超任务队列默认超时，使用独立的长超时上下文
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Hour)
	defer cancel()
	err := run(ctx, p.JobID, paramsJSON)
	now := time.Now()
	updateJob(p.JobID, func(j *BgJob) {
		j.FinishedAt = &now
		if err != nil {
			j.Status = models.TaskFailed
			j.Error = err.Error()
		} else {
			j.Status = models.TaskSuccess
			j.Progress = 100
		}
	})
	if err != nil {
		logger.Warn("后台作业 %s(%s) 失败: %v", kind, p.Label, err)
	}
	return err
}

// submitBgJob 创建作业并提交到任务队列；队列未启用时在后台 goroutine 中执行并自行维护任务记录
func submitBgJob(kind, label string, params map[string]interface{}) (*BgJob, error) {
	job := &BgJob{ID: newJobID(), Kind: kind, Label: label, Status: models.TaskQueued, Progress: -1, CreatedAt: time.Now()}
	params["job_id"] = job.ID
	params["label"] = label
	pj, _ := json.Marshal(params)
	paramsJSON := string(pj)
	putJob(job)
	taskName := kind + ":" + label
	if len(taskName) > 250 {
		taskName = taskName[:250]
	}
	if executor.GlobalQueue != nil || executor.GlobalRedisQueue != nil {
		t, err := executor.CreateTask(taskName, kind, "admin", params, func(ctx context.Context) error {
			return runBgJob(kind, paramsJSON)
		})
		if err != nil {
			bgJobs.Lock()
			delete(bgJobs.m, job.ID)
			bgJobs.Unlock()
			return nil, err
		}
		updateJob(job.ID, func(j *BgJob) { j.TaskID = t.ID })
		job.TaskID = t.ID
		return job, nil
	}
	// 无任务队列：自行登记任务并后台执行
	now := time.Now()
	t := &models.Task{ContainerName: taskName, Action: kind, UserID: "admin", Type: "async", Status: models.TaskRunning, Params: paramsJSON, StartedAt: &now}
	if db.DB != nil {
		db.DB.Create(t)
	}
	job.TaskID = t.ID
	updateJob(job.ID, func(j *BgJob) { j.TaskID = t.ID })
	go func() {
		err := runBgJob(kind, paramsJSON)
		done := time.Now()
		upd := map[string]interface{}{"completed_at": &done, "duration": done.Sub(now).Milliseconds()}
		if err != nil {
			upd["status"], upd["error_msg"] = models.TaskFailed, err.Error()
		} else {
			upd["status"], upd["result"] = models.TaskSuccess, "操作执行成功"
		}
		if db.DB != nil && t.ID != 0 {
			db.DB.Model(&models.Task{}).Where("id = ?", t.ID).Updates(upd)
		}
	}()
	return job, nil
}

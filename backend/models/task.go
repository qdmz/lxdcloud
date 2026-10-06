package models

import (
	"time"
	"gorm.io/gorm"
)

const (
	TaskQueued  = "queued"
	TaskRunning = "running"
	TaskSuccess = "success"
	TaskFailed  = "failed"
)

type Task struct {
	gorm.Model
	ContainerName string     `gorm:"index;size:255" json:"container_name"`
	UserID        string     `gorm:"index;size:255" json:"user_id"`
	Action        string     `gorm:"size:100" json:"action"`
	Type          string     `gorm:"size:50" json:"type"`
	Status        string     `gorm:"index;size:50" json:"status"`
	Params        string     `gorm:"type:text" json:"params"`
	Result        string     `gorm:"type:text" json:"result"`
	Logs          string     `gorm:"type:text" json:"logs"`
	ErrorMsg      string     `gorm:"type:text" json:"error_msg"`
	StartedAt     *time.Time `json:"started_at"`
	CompletedAt   *time.Time `json:"completed_at"`
	Duration      int64      `json:"duration"`
}


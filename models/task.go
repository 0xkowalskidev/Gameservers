package models

import (
	"time"
)

type TaskType string

const (
	TaskTypeRestart TaskType = "restart"
	TaskTypeBackup  TaskType = "backup"
)

type TaskStatus string

const (
	TaskStatusActive   TaskStatus = "active"
	TaskStatusDisabled TaskStatus = "disabled"
)

type ScheduledTask struct {
	ID           string     `json:"id"`
	GameserverID string     `json:"gameserver_id"`
	Name         string     `json:"name"`
	Type         TaskType   `json:"type"`
	Status       TaskStatus `json:"status"`
	CronSchedule string     `json:"cron_schedule"`
	CreatedAt    time.Time  `json:"created_at"`
	UpdatedAt    time.Time  `json:"updated_at"`
	LastRun      *time.Time `json:"last_run,omitempty"`
	NextRun      *time.Time `json:"next_run,omitempty"`
}
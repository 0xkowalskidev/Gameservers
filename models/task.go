package models

import (
	"time"

	"gorm.io/gorm"
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
	ID           string     `json:"id" gorm:"primaryKey;type:varchar(50)"`
	GameserverID string     `json:"gameserver_id" gorm:"type:varchar(50);not null;index"`
	Name         string     `json:"name" gorm:"type:varchar(200);not null"`
	Type         TaskType   `json:"type" gorm:"type:varchar(20);not null"`
	Status       TaskStatus `json:"status" gorm:"type:varchar(20);not null;default:'active'"`
	CronSchedule string     `json:"cron_schedule" gorm:"type:varchar(100);not null"`
	CreatedAt    time.Time  `json:"created_at"`
	UpdatedAt    time.Time  `json:"updated_at"`
	DeletedAt    gorm.DeletedAt `json:"deleted_at,omitempty" gorm:"index"`
	LastRun      *time.Time `json:"last_run,omitempty"`
	NextRun      *time.Time `json:"next_run,omitempty"`

	// Relations (removed foreign key constraint to avoid migration issues) 
	Gameserver *Gameserver `json:"gameserver,omitempty" gorm:"-"`
}

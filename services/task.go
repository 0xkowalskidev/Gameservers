package services

import (
	"time"

	"0xkowalskidev/gameservers/models"
)

// TaskService handles scheduled task operations
type TaskService struct {
	db models.DatabaseInterface
}

// NewTaskService creates a new task service
func NewTaskService(db models.DatabaseInterface) models.TaskServiceInterface {
	return &TaskService{
		db: db,
	}
}

// CreateScheduledTask creates a new scheduled task
func (s *TaskService) CreateScheduledTask(task *models.ScheduledTask) error {
	now := time.Now()
	task.CreatedAt, task.UpdatedAt = now, now
	task.ID = models.GenerateID()

	// Calculate initial next run time
	nextRun := CalculateNextRun(task.CronSchedule, now)
	if !nextRun.IsZero() {
		task.NextRun = &nextRun
	}

	return s.db.CreateScheduledTask(task)
}

// GetScheduledTask returns a specific scheduled task
func (s *TaskService) GetScheduledTask(id string) (*models.ScheduledTask, error) {
	return s.db.GetScheduledTask(id)
}

// UpdateScheduledTask updates an existing scheduled task
func (s *TaskService) UpdateScheduledTask(task *models.ScheduledTask) error {
	task.UpdatedAt = time.Now()
	// Clear next run time so scheduler will recalculate it
	task.NextRun = nil
	return s.db.UpdateScheduledTask(task)
}

// DeleteScheduledTask deletes a scheduled task
func (s *TaskService) DeleteScheduledTask(id string) error {
	return s.db.DeleteScheduledTask(id)
}

// ListScheduledTasksForGameserver returns all scheduled tasks for a gameserver
func (s *TaskService) ListScheduledTasksForGameserver(gameserverID string) ([]*models.ScheduledTask, error) {
	return s.db.ListScheduledTasksForGameserver(gameserverID)
}
package database

import (
	"fmt"

	"gorm.io/gorm"

	"0xkowalskidev/gameservers/models"
)

// CreateScheduledTask inserts a new scheduled task into the database
func (dm *DatabaseManager) CreateScheduledTask(task *models.ScheduledTask) error {
	if err := dm.db.Create(task).Error; err != nil {
		return &models.DatabaseError{Op: "create_task", Msg: "failed to create scheduled task", Err: err}
	}
	return nil
}

// GetScheduledTask retrieves a scheduled task by ID
func (dm *DatabaseManager) GetScheduledTask(id string) (*models.ScheduledTask, error) {
	var task models.ScheduledTask
	if err := dm.db.First(&task, "id = ?", id).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, &models.DatabaseError{Op: "get_task", Msg: fmt.Sprintf("scheduled task %s not found", id), Err: nil}
		}
		return nil, &models.DatabaseError{Op: "get_task", Msg: fmt.Sprintf("failed to query scheduled task %s", id), Err: err}
	}
	return &task, nil
}

// UpdateScheduledTask updates an existing scheduled task
func (dm *DatabaseManager) UpdateScheduledTask(task *models.ScheduledTask) error {
	result := dm.db.Save(task)
	if result.Error != nil {
		return &models.DatabaseError{Op: "update_task", Msg: "failed to update scheduled task", Err: result.Error}
	}
	if result.RowsAffected == 0 {
		return &models.DatabaseError{Op: "update_task", Msg: fmt.Sprintf("scheduled task %s not found", task.ID), Err: nil}
	}
	return nil
}

// DeleteScheduledTask deletes a scheduled task by ID
func (dm *DatabaseManager) DeleteScheduledTask(id string) error {
	result := dm.db.Unscoped().Delete(&models.ScheduledTask{}, "id = ?", id)
	if result.Error != nil {
		return &models.DatabaseError{Op: "delete_task", Msg: "failed to delete scheduled task", Err: result.Error}
	}
	if result.RowsAffected == 0 {
		return &models.DatabaseError{Op: "delete_task", Msg: fmt.Sprintf("scheduled task %s not found", id), Err: nil}
	}
	return nil
}

// ListScheduledTasksForGameserver retrieves all scheduled tasks for a gameserver
func (dm *DatabaseManager) ListScheduledTasksForGameserver(gameserverID string) ([]*models.ScheduledTask, error) {
	var tasks []*models.ScheduledTask
	if err := dm.db.Where("gameserver_id = ?", gameserverID).Order("created_at DESC").Find(&tasks).Error; err != nil {
		return nil, &models.DatabaseError{Op: "list_tasks", Msg: "failed to query scheduled tasks", Err: err}
	}
	return tasks, nil
}

// ListActiveScheduledTasks retrieves all active scheduled tasks
func (dm *DatabaseManager) ListActiveScheduledTasks() ([]*models.ScheduledTask, error) {
	var tasks []*models.ScheduledTask
	if err := dm.db.Where("status = ?", models.TaskStatusActive).Order("next_run ASC").Find(&tasks).Error; err != nil {
		return nil, &models.DatabaseError{Op: "list_active_tasks", Msg: "failed to query active scheduled tasks", Err: err}
	}
	return tasks, nil
}

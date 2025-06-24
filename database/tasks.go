package database

import (
	"database/sql"
	"fmt"

	"0xkowalskidev/gameservers/models"
)

// CreateScheduledTask inserts a new scheduled task into the database
func (dm *DatabaseManager) CreateScheduledTask(task *models.ScheduledTask) error {
	query := `INSERT INTO scheduled_tasks (id, gameserver_id, name, type, status, cron_schedule, created_at, updated_at, last_run, next_run) 
			  VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`

	_, err := dm.db.Exec(query, task.ID, task.GameserverID, task.Name, string(task.Type), string(task.Status),
		task.CronSchedule, task.CreatedAt, task.UpdatedAt, task.LastRun, task.NextRun)

	if err != nil {
		return &models.DatabaseError{Op: "create_task", Msg: "failed to create scheduled task", Err: err}
	}
	return nil
}

// GetScheduledTask retrieves a scheduled task by ID
func (dm *DatabaseManager) GetScheduledTask(id string) (*models.ScheduledTask, error) {
	query := `SELECT id, gameserver_id, name, type, status, cron_schedule, created_at, updated_at, last_run, next_run 
			  FROM scheduled_tasks WHERE id = ?`

	row := dm.db.QueryRow(query, id)
	task, err := dm.scanScheduledTask(row)
	if err == sql.ErrNoRows {
		return nil, &models.DatabaseError{Op: "get_task", Msg: fmt.Sprintf("scheduled task %s not found", id), Err: nil}
	}
	if err != nil {
		return nil, &models.DatabaseError{Op: "get_task", Msg: fmt.Sprintf("failed to query scheduled task %s", id), Err: err}
	}
	return task, nil
}

// UpdateScheduledTask updates an existing scheduled task
func (dm *DatabaseManager) UpdateScheduledTask(task *models.ScheduledTask) error {
	query := `UPDATE scheduled_tasks SET name = ?, type = ?, status = ?, cron_schedule = ?, updated_at = ?, last_run = ?, next_run = ? 
			  WHERE id = ?`

	result, err := dm.db.Exec(query, task.Name, string(task.Type), string(task.Status), task.CronSchedule,
		task.UpdatedAt, task.LastRun, task.NextRun, task.ID)

	if err != nil {
		return &models.DatabaseError{Op: "update_task", Msg: "failed to update scheduled task", Err: err}
	}
	if rowsAffected, _ := result.RowsAffected(); rowsAffected == 0 {
		return &models.DatabaseError{Op: "update_task", Msg: fmt.Sprintf("scheduled task %s not found", task.ID), Err: nil}
	}
	return nil
}

// DeleteScheduledTask deletes a scheduled task by ID
func (dm *DatabaseManager) DeleteScheduledTask(id string) error {
	query := `DELETE FROM scheduled_tasks WHERE id = ?`

	result, err := dm.db.Exec(query, id)
	if err != nil {
		return &models.DatabaseError{Op: "delete_task", Msg: "failed to delete scheduled task", Err: err}
	}
	if rowsAffected, _ := result.RowsAffected(); rowsAffected == 0 {
		return &models.DatabaseError{Op: "delete_task", Msg: fmt.Sprintf("scheduled task %s not found", id), Err: nil}
	}
	return nil
}

// ListScheduledTasksForGameserver retrieves all scheduled tasks for a gameserver
func (dm *DatabaseManager) ListScheduledTasksForGameserver(gameserverID string) ([]*models.ScheduledTask, error) {
	query := `SELECT id, gameserver_id, name, type, status, cron_schedule, created_at, updated_at, last_run, next_run 
			  FROM scheduled_tasks WHERE gameserver_id = ? ORDER BY created_at DESC`

	rows, err := dm.db.Query(query, gameserverID)
	if err != nil {
		return nil, &models.DatabaseError{Op: "list_tasks", Msg: "failed to query scheduled tasks", Err: err}
	}
	defer rows.Close()

	var tasks []*models.ScheduledTask
	for rows.Next() {
		task, err := dm.scanScheduledTask(rows)
		if err != nil {
			return nil, &models.DatabaseError{Op: "list_tasks", Msg: "failed to scan scheduled task row", Err: err}
		}
		tasks = append(tasks, task)
	}
	return tasks, rows.Err()
}

// ListActiveScheduledTasks retrieves all active scheduled tasks
func (dm *DatabaseManager) ListActiveScheduledTasks() ([]*models.ScheduledTask, error) {
	query := `SELECT id, gameserver_id, name, type, status, cron_schedule, created_at, updated_at, last_run, next_run 
			  FROM scheduled_tasks WHERE status = ? ORDER BY next_run ASC`

	rows, err := dm.db.Query(query, string(models.TaskStatusActive))
	if err != nil {
		return nil, &models.DatabaseError{Op: "list_active_tasks", Msg: "failed to query active scheduled tasks", Err: err}
	}
	defer rows.Close()

	var tasks []*models.ScheduledTask
	for rows.Next() {
		task, err := dm.scanScheduledTask(rows)
		if err != nil {
			return nil, &models.DatabaseError{Op: "list_active_tasks", Msg: "failed to scan scheduled task row", Err: err}
		}
		tasks = append(tasks, task)
	}
	return tasks, rows.Err()
}

// ScheduledTaskScanner interface for scanning scheduled task rows
type ScheduledTaskScanner interface {
	Scan(dest ...interface{}) error
}

// scanScheduledTask scans a database row into a ScheduledTask struct
func (dm *DatabaseManager) scanScheduledTask(row ScheduledTaskScanner) (*models.ScheduledTask, error) {
	var task models.ScheduledTask
	var taskType, status string
	var lastRun, nextRun sql.NullTime

	err := row.Scan(&task.ID, &task.GameserverID, &task.Name, &taskType, &status,
		&task.CronSchedule, &task.CreatedAt, &task.UpdatedAt, &lastRun, &nextRun)

	if err != nil {
		return nil, err
	}

	task.Type = models.TaskType(taskType)
	task.Status = models.TaskStatus(status)

	if lastRun.Valid {
		task.LastRun = &lastRun.Time
	}
	if nextRun.Valid {
		task.NextRun = &nextRun.Time
	}

	return &task, nil
}

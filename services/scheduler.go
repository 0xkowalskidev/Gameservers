package services

import (
	"time"

	"github.com/robfig/cron/v3"
	"github.com/rs/zerolog/log"

	"0xkowalskidev/gameservers/database"
	"0xkowalskidev/gameservers/models"
)

// TaskScheduler handles scheduled task execution
type TaskScheduler struct {
	db            DatabaseInterface
	gameserverSvc *database.GameserverRepository
	ticker        *time.Ticker
	done          chan struct{}
	checkInterval time.Duration
}

// DatabaseInterface defines the required database operations for the scheduler
type DatabaseInterface interface {
	ListActiveScheduledTasks() ([]*models.ScheduledTask, error)
	UpdateScheduledTask(task *models.ScheduledTask) error
}

// NewTaskScheduler creates a new task scheduler instance
func NewTaskScheduler(db DatabaseInterface, gameserverSvc *database.GameserverRepository) *TaskScheduler {
	return &TaskScheduler{
		db:            db,
		gameserverSvc: gameserverSvc,
		done:          make(chan struct{}),
		checkInterval: time.Minute,
	}
}

// Start begins the task scheduler
func (ts *TaskScheduler) Start() {
	log.Info().Dur("interval", ts.checkInterval).Msg("Starting task scheduler")
	ts.ticker = time.NewTicker(ts.checkInterval)

	go func() {
		ts.updateNextRunTimes() // Initial calculation
		for {
			select {
			case <-ts.done:
				return
			case <-ts.ticker.C:
				ts.processTasks()
			}
		}
	}()
}

// Stop halts the task scheduler
func (ts *TaskScheduler) Stop() {
	log.Info().Msg("Stopping task scheduler")
	if ts.ticker != nil {
		ts.ticker.Stop()
	}
	close(ts.done)
}

func (ts *TaskScheduler) updateNextRunTimes() {
	tasks, err := ts.db.ListActiveScheduledTasks()
	if err != nil {
		log.Error().Err(err).Msg("Failed to list active scheduled tasks")
		return
	}

	now := time.Now()
	for _, task := range tasks {
		if task.NextRun == nil {
			ts.updateTaskNextRun(task, now)
		}
	}
}

func (ts *TaskScheduler) processTasks() {
	now := time.Now()
	tasks, err := ts.db.ListActiveScheduledTasks()
	if err != nil {
		log.Error().Err(err).Msg("Failed to list active scheduled tasks")
		return
	}

	for _, task := range tasks {
		if task.NextRun == nil {
			ts.updateTaskNextRun(task, now)
		} else if task.NextRun.Before(now) {
			ts.executeTask(task)
			task.LastRun = &now
			ts.updateTaskNextRun(task, now)
		}
	}
}

func (ts *TaskScheduler) updateTaskNextRun(task *models.ScheduledTask, from time.Time) {
	schedule, err := cron.ParseStandard(task.CronSchedule)
	if err != nil {
		log.Error().Err(err).Str("task_id", task.ID).Str("cron", task.CronSchedule).Msg("Invalid cron schedule")
		task.NextRun = nil
	} else {
		nextRun := schedule.Next(from)
		task.NextRun = &nextRun
	}
	task.UpdatedAt = from

	if err := ts.db.UpdateScheduledTask(task); err != nil {
		log.Error().Err(err).Str("task_id", task.ID).Msg("Failed to update task")
	}
}

func (ts *TaskScheduler) executeTask(task *models.ScheduledTask) {
	log.Info().Str("task_id", task.ID).Str("task_name", task.Name).Str("type", string(task.Type)).Msg("Executing scheduled task")
	if err := ts.gameserverSvc.ExecuteScheduledTask(task); err != nil {
		log.Error().Err(err).Str("task_id", task.ID).Str("task_name", task.Name).Msg("Failed to execute scheduled task")
	}
}

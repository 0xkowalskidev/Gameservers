package services

import (
	"context"
	"time"

	"github.com/rs/zerolog/log"

	"0xkowalskidev/gameservers/models"
)

// TaskScheduler handles scheduled task execution
type TaskScheduler struct {
	db            DatabaseInterface
	gameserverSvc GameserverServiceInterface
	ctx           context.Context
	cancel        context.CancelFunc
	ticker        *time.Ticker
	checkInterval time.Duration
}

// DatabaseInterface defines the required database operations for the scheduler
type DatabaseInterface interface {
	ListActiveScheduledTasks() ([]*models.ScheduledTask, error)
	UpdateScheduledTask(task *models.ScheduledTask) error
}

// NewTaskScheduler creates a new task scheduler instance
func NewTaskScheduler(db DatabaseInterface, gameserverSvc GameserverServiceInterface) *TaskScheduler {
	ctx, cancel := context.WithCancel(context.Background())
	return &TaskScheduler{
		db:            db,
		gameserverSvc: gameserverSvc,
		ctx:           ctx,
		cancel:        cancel,
		checkInterval: time.Minute, // Check every minute
	}
}

// Start begins the task scheduler
func (ts *TaskScheduler) Start() {
	log.Info().Dur("interval", ts.checkInterval).Msg("Starting task scheduler")
	
	ts.ticker = time.NewTicker(ts.checkInterval)
	go func() {
		// Initial calculation of next run times
		ts.calculateNextRunTimes()
		
		for {
			select {
			case <-ts.ctx.Done():
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
	ts.cancel()
}

func (ts *TaskScheduler) calculateNextRunTimes() {
	tasks, err := ts.db.ListActiveScheduledTasks()
	if err != nil {
		log.Error().Err(err).Msg("Failed to list active scheduled tasks")
		return
	}

	for _, task := range tasks {
		nextRun := CalculateNextRun(task.CronSchedule, time.Now())
		if !nextRun.IsZero() {
			task.NextRun = &nextRun
			task.UpdatedAt = time.Now()
			if err := ts.db.UpdateScheduledTask(task); err != nil {
				log.Error().Err(err).Str("task_id", task.ID).Msg("Failed to update task next run time")
			}
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
		// Recalculate next run time if it's nil (e.g., after task update)
		if task.NextRun == nil {
			log.Info().Str("task_id", task.ID).Str("task_name", task.Name).Msg("Recalculating next run time for updated task")
			nextRun := CalculateNextRun(task.CronSchedule, now)
			if !nextRun.IsZero() {
				task.NextRun = &nextRun
			} else {
				task.NextRun = nil
			}
			task.UpdatedAt = now
			if err := ts.db.UpdateScheduledTask(task); err != nil {
				log.Error().Err(err).Str("task_id", task.ID).Msg("Failed to update task next run time")
			}
			continue
		}

		// Check if task is due
		if task.NextRun != nil && task.NextRun.Before(now) {
			ts.executeTask(task)
			
			// Update last run and calculate next run
			task.LastRun = &now
			nextRun := CalculateNextRun(task.CronSchedule, now)
			if !nextRun.IsZero() {
				task.NextRun = &nextRun
			} else {
				task.NextRun = nil
			}
			task.UpdatedAt = now
			
			if err := ts.db.UpdateScheduledTask(task); err != nil {
				log.Error().Err(err).Str("task_id", task.ID).Msg("Failed to update task after execution")
			}
		}
	}
}

func (ts *TaskScheduler) executeTask(task *models.ScheduledTask) {
	log.Info().
		Str("task_id", task.ID).
		Str("task_name", task.Name).
		Str("type", string(task.Type)).
		Msg("Executing scheduled task")

	if err := ts.gameserverSvc.ExecuteScheduledTask(context.Background(), task); err != nil {
		log.Error().
			Err(err).
			Str("task_id", task.ID).
			Str("task_name", task.Name).
			Msg("Failed to execute scheduled task")
	}
}
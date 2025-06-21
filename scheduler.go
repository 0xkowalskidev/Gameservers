package main

import (
	"context"
	"fmt"
	"time"

	"github.com/rs/zerolog/log"
)

type TaskScheduler struct {
	db             *DatabaseManager
	gameserverSvc  *GameserverService
	ctx            context.Context
	cancel         context.CancelFunc
	ticker         *time.Ticker
	checkInterval  time.Duration
}

func NewTaskScheduler(db *DatabaseManager, gameserverSvc *GameserverService) *TaskScheduler {
	ctx, cancel := context.WithCancel(context.Background())
	return &TaskScheduler{
		db:             db,
		gameserverSvc:  gameserverSvc,
		ctx:            ctx,
		cancel:         cancel,
		checkInterval:  time.Minute, // Check every minute
	}
}

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
		
		if now.After(*task.NextRun) {
			log.Info().Str("task_id", task.ID).Str("task_name", task.Name).Str("type", string(task.Type)).Msg("Executing scheduled task")
			
			if err := ts.executeTask(task); err != nil {
				log.Error().Err(err).Str("task_id", task.ID).Msg("Failed to execute scheduled task")
				continue
			}

			// Update last run and calculate next run
			now := time.Now()
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

func (ts *TaskScheduler) executeTask(task *ScheduledTask) error {
	// First check if the gameserver exists and get its status
	gameserver, err := ts.gameserverSvc.GetGameserver(task.GameserverID)
	if err != nil {
		log.Error().Err(err).Str("gameserver_id", task.GameserverID).Msg("Gameserver not found, skipping task")
		return fmt.Errorf("gameserver not found: %w", err)
	}
	
	switch task.Type {
	case TaskTypeRestart:
		// Only restart if the gameserver is currently running
		if gameserver.Status != StatusRunning {
			log.Info().Str("gameserver_id", task.GameserverID).Str("status", string(gameserver.Status)).Msg("Skipping restart - gameserver not running")
			return nil // Don't treat this as an error, just skip
		}
		log.Info().Str("gameserver_id", task.GameserverID).Msg("Executing scheduled restart")
		return ts.gameserverSvc.RestartGameserver(task.GameserverID)
	
	case TaskTypeBackup:
		// Backups can run regardless of gameserver status (stopped servers can still be backed up)
		log.Info().Str("gameserver_id", task.GameserverID).Str("status", string(gameserver.Status)).Msg("Executing scheduled backup")
		return ts.createBackup(task.GameserverID)
	
	default:
		return fmt.Errorf("unknown task type: %s", task.Type)
	}
}

func (ts *TaskScheduler) createBackup(gameserverID string) error {
	// Get gameserver info
	gameserver, err := ts.gameserverSvc.GetGameserver(gameserverID)
	if err != nil {
		return fmt.Errorf("failed to get gameserver: %w", err)
	}
	
	// Create backup directly in the server volume
	err = ts.gameserverSvc.docker.CreateBackup(gameserver.ContainerID, gameserver.Name)
	if err != nil {
		return err
	}
	
	// Clean up old backups if max_backups is set
	err = ts.gameserverSvc.docker.CleanupOldBackups(gameserver.ContainerID, gameserver.MaxBackups)
	if err != nil {
		log.Error().Err(err).Str("gameserver_id", gameserverID).Msg("Failed to cleanup old backups")
		// Don't return error for cleanup failure, backup creation was successful
	}
	
	return nil
}


package main

import (
	"context"
	"fmt"
	"strconv"
	"strings"
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
		nextRun := ts.calculateNextRun(task.CronSchedule, time.Now())
		if nextRun != nil {
			task.NextRun = nextRun
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
		if task.NextRun != nil && now.After(*task.NextRun) {
			log.Info().Str("task_id", task.ID).Str("task_name", task.Name).Str("type", string(task.Type)).Msg("Executing scheduled task")
			
			if err := ts.executeTask(task); err != nil {
				log.Error().Err(err).Str("task_id", task.ID).Msg("Failed to execute scheduled task")
				continue
			}

			// Update last run and calculate next run
			now := time.Now()
			task.LastRun = &now
			task.NextRun = ts.calculateNextRun(task.CronSchedule, now)
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

// Simple cron parser for basic patterns: "minute hour day month weekday"
// Supports: numbers, asterisks (*), and step values (*/5)
func (ts *TaskScheduler) calculateNextRun(cronSchedule string, from time.Time) *time.Time {
	parts := strings.Fields(cronSchedule)
	if len(parts) != 5 {
		log.Error().Str("cron", cronSchedule).Msg("Invalid cron schedule format")
		return nil
	}

	minute := parts[0]
	hour := parts[1]
	day := parts[2]
	month := parts[3]
	weekday := parts[4]

	// Start from the next minute
	next := from.Truncate(time.Minute).Add(time.Minute)
	
	// Simple implementation - find next matching time within next 7 days
	for attempts := 0; attempts < 7*24*60; attempts++ {
		if ts.cronMatches(next, minute, hour, day, month, weekday) {
			return &next
		}
		next = next.Add(time.Minute)
	}

	log.Error().Str("cron", cronSchedule).Msg("Could not calculate next run time")
	return nil
}

func (ts *TaskScheduler) cronMatches(t time.Time, minute, hour, day, month, weekday string) bool {
	return ts.fieldMatches(t.Minute(), minute) &&
		ts.fieldMatches(t.Hour(), hour) &&
		ts.fieldMatches(t.Day(), day) &&
		ts.fieldMatches(int(t.Month()), month) &&
		ts.fieldMatches(int(t.Weekday()), weekday)
}

func (ts *TaskScheduler) fieldMatches(value int, pattern string) bool {
	if pattern == "*" {
		return true
	}
	
	// Handle step values like */5
	if strings.HasPrefix(pattern, "*/") {
		stepStr := pattern[2:]
		if step, err := strconv.Atoi(stepStr); err == nil {
			return value%step == 0
		}
		return false
	}
	
	// Handle exact matches
	if patternValue, err := strconv.Atoi(pattern); err == nil {
		return value == patternValue
	}
	
	return false
}


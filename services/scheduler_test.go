package services

import (
	"context"
	"fmt"
	"testing"
	"time"

	"0xkowalskidev/gameservers/models"
)

// =============================================================================
// Mock Database for Scheduler Testing
// =============================================================================

type MockSchedulerDatabase struct {
	tasks      map[string]*models.ScheduledTask
	shouldFail map[string]bool
}

func NewMockSchedulerDatabase() *MockSchedulerDatabase {
	return &MockSchedulerDatabase{
		tasks:      make(map[string]*models.ScheduledTask),
		shouldFail: make(map[string]bool),
	}
}

func (m *MockSchedulerDatabase) ListActiveScheduledTasks() ([]*models.ScheduledTask, error) {
	if m.shouldFail["list_active"] {
		return nil, fmt.Errorf("mock list active error")
	}
	
	var activeTasks []*models.ScheduledTask
	for _, task := range m.tasks {
		if task.Status == models.TaskStatusActive {
			activeTasks = append(activeTasks, task)
		}
	}
	return activeTasks, nil
}

func (m *MockSchedulerDatabase) UpdateScheduledTask(task *models.ScheduledTask) error {
	if m.shouldFail["update"] {
		return fmt.Errorf("mock update error")
	}
	
	m.tasks[task.ID] = task
	return nil
}

// =============================================================================
// Mock Gameserver Service for Scheduler Testing
// =============================================================================

type MockSchedulerGameserverService struct {
	gameservers    map[string]*models.Gameserver
	executedTasks  []string
	shouldFail     map[string]bool
	restartCalled  []string
	backupsCalled  []string
}

func NewMockSchedulerGameserverService() *MockSchedulerGameserverService {
	return &MockSchedulerGameserverService{
		gameservers:   make(map[string]*models.Gameserver),
		executedTasks: make([]string, 0),
		shouldFail:    make(map[string]bool),
		restartCalled: make([]string, 0),
		backupsCalled: make([]string, 0),
	}
}

func (m *MockSchedulerGameserverService) CreateGameserver(ctx context.Context, req CreateGameserverRequest) (*models.Gameserver, error) {
	return nil, fmt.Errorf("not implemented")
}

func (m *MockSchedulerGameserverService) GetGameserver(ctx context.Context, id string) (*models.Gameserver, error) {
	if gs, exists := m.gameservers[id]; exists {
		return gs, nil
	}
	return nil, fmt.Errorf("gameserver not found")
}

func (m *MockSchedulerGameserverService) UpdateGameserver(ctx context.Context, id string, req UpdateGameserverRequest) error {
	return fmt.Errorf("not implemented")
}

func (m *MockSchedulerGameserverService) DeleteGameserver(ctx context.Context, id string) error {
	return fmt.Errorf("not implemented")
}

func (m *MockSchedulerGameserverService) StartGameserver(ctx context.Context, id string) error {
	if m.shouldFail["start"] {
		return fmt.Errorf("mock start error")
	}
	if gs, exists := m.gameservers[id]; exists {
		gs.Status = models.StatusRunning
		return nil
	}
	return fmt.Errorf("gameserver not found")
}

func (m *MockSchedulerGameserverService) StopGameserver(ctx context.Context, id string) error {
	if m.shouldFail["stop"] {
		return fmt.Errorf("mock stop error")
	}
	if gs, exists := m.gameservers[id]; exists {
		gs.Status = models.StatusStopped
		return nil
	}
	return fmt.Errorf("gameserver not found")
}

func (m *MockSchedulerGameserverService) RestartGameserver(ctx context.Context, id string) error {
	if m.shouldFail["restart"] {
		return fmt.Errorf("mock restart error")
	}
	m.restartCalled = append(m.restartCalled, id)
	return nil
}

func (m *MockSchedulerGameserverService) ExecuteScheduledTask(ctx context.Context, task *models.ScheduledTask) error {
	if m.shouldFail["execute"] {
		return fmt.Errorf("mock execute error")
	}
	
	m.executedTasks = append(m.executedTasks, task.ID)
	
	switch task.Type {
	case models.TaskTypeRestart:
		// Check if gameserver exists and is running
		if gs, exists := m.gameservers[task.GameserverID]; exists {
			if gs.Status == models.StatusRunning {
				return m.RestartGameserver(ctx, task.GameserverID)
			}
		}
	case models.TaskTypeBackup:
		m.backupsCalled = append(m.backupsCalled, task.GameserverID)
		return m.CreateBackup(ctx, task.GameserverID, "")
	}
	
	return nil
}

func (m *MockSchedulerGameserverService) CreateBackup(ctx context.Context, gameserverID string, name string) error {
	if m.shouldFail["backup"] {
		return fmt.Errorf("mock backup error")
	}
	return nil
}

func (m *MockSchedulerGameserverService) FileOperation(ctx context.Context, gameserverID string, path string, op func(string, string) error) error {
	return fmt.Errorf("not implemented")
}

// =============================================================================
// Scheduler Tests
// =============================================================================

func TestTaskScheduler_NextRunCalculation(t *testing.T) {
	db := NewMockSchedulerDatabase()
	svc := NewMockSchedulerGameserverService()
	scheduler := NewTaskScheduler(db, svc)
	
	// Create a task with a cron schedule
	task := &models.ScheduledTask{
		ID:           "test-task",
		GameserverID: "test-gs",
		Name:         "Test Task",
		Type:         models.TaskTypeRestart,
		Status:       models.TaskStatusActive,
		CronSchedule: "0 2 * * *", // Daily at 2 AM
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}
	
	db.tasks[task.ID] = task
	
	// Calculate next run times
	scheduler.updateNextRunTimes()
	
	// Verify next run was set
	updatedTask := db.tasks[task.ID]
	if updatedTask.NextRun == nil {
		t.Errorf("Expected next run to be calculated, but it was nil")
	}
}

func TestTaskScheduler_TaskExecution(t *testing.T) {
	tests := []struct {
		name           string
		taskType       models.TaskType
		serverStatus   models.GameserverStatus
		expectExecuted bool
		expectRestart  bool
		expectBackup   bool
	}{
		{
			name:           "restart_running_server",
			taskType:       models.TaskTypeRestart,
			serverStatus:   models.StatusRunning,
			expectExecuted: true,
			expectRestart:  true,
			expectBackup:   false,
		},
		{
			name:           "skip_restart_stopped_server",
			taskType:       models.TaskTypeRestart,
			serverStatus:   models.StatusStopped,
			expectExecuted: true,
			expectRestart:  false,
			expectBackup:   false,
		},
		{
			name:           "backup_running_server",
			taskType:       models.TaskTypeBackup,
			serverStatus:   models.StatusRunning,
			expectExecuted: true,
			expectRestart:  false,
			expectBackup:   true,
		},
		{
			name:           "backup_stopped_server",
			taskType:       models.TaskTypeBackup,
			serverStatus:   models.StatusStopped,
			expectExecuted: true,
			expectRestart:  false,
			expectBackup:   true,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db := NewMockSchedulerDatabase()
			svc := NewMockSchedulerGameserverService()
			scheduler := NewTaskScheduler(db, svc)
			
			// Setup gameserver
			gameserver := &models.Gameserver{
				ID:     "test-gs",
				Name:   "Test Server",
				Status: tt.serverStatus,
			}
			svc.gameservers[gameserver.ID] = gameserver
			
			// Create task that's due now
			now := time.Now()
			task := &models.ScheduledTask{
				ID:           "test-task",
				GameserverID: gameserver.ID,
				Name:         "Test Task",
				Type:         tt.taskType,
				Status:       models.TaskStatusActive,
				CronSchedule: "* * * * *", // Every minute
				NextRun:      &now,
				CreatedAt:    now,
				UpdatedAt:    now,
			}
			db.tasks[task.ID] = task
			
			// Execute the scheduler check
			scheduler.processTasks()
			
			// Verify execution
			if tt.expectExecuted {
				if len(svc.executedTasks) == 0 {
					t.Errorf("Expected task to be executed, but it wasn't")
				}
			}
			
			if tt.expectRestart {
				if len(svc.restartCalled) == 0 {
					t.Errorf("Expected restart to be called, but it wasn't")
				}
			} else {
				if len(svc.restartCalled) > 0 {
					t.Errorf("Expected restart NOT to be called, but it was")
				}
			}
			
			if tt.expectBackup {
				if len(svc.backupsCalled) == 0 {
					t.Errorf("Expected backup to be called, but it wasn't")
				}
			} else {
				if len(svc.backupsCalled) > 0 {
					t.Errorf("Expected backup NOT to be called, but it was")
				}
			}
			
			// Verify task was updated with new next run
			updatedTask := db.tasks[task.ID]
			if updatedTask.LastRun == nil {
				t.Errorf("Expected LastRun to be set after execution")
			}
			if updatedTask.NextRun == nil || updatedTask.NextRun.Equal(now) {
				t.Errorf("Expected NextRun to be updated after execution")
			}
		})
	}
}

func TestTaskScheduler_ErrorHandling(t *testing.T) {
	db := NewMockSchedulerDatabase()
	svc := NewMockSchedulerGameserverService()
	scheduler := NewTaskScheduler(db, svc)
	
	// Test with non-existent gameserver
	now := time.Now()
	task := &models.ScheduledTask{
		ID:           "test-task",
		GameserverID: "non-existent",
		Name:         "Test Task",
		Type:         models.TaskTypeRestart,
		Status:       models.TaskStatusActive,
		CronSchedule: "* * * * *",
		NextRun:      &now,
	}
	db.tasks[task.ID] = task
	
	// This should not panic
	scheduler.processTasks()
	
	// Task should still be updated even if execution failed
	updatedTask := db.tasks[task.ID]
	if updatedTask.LastRun == nil {
		t.Errorf("Expected LastRun to be set even after error")
	}
}

func TestTaskScheduler_DatabaseErrors(t *testing.T) {
	t.Run("list_active_error", func(t *testing.T) {
		db := NewMockSchedulerDatabase()
		svc := NewMockSchedulerGameserverService()
		scheduler := NewTaskScheduler(db, svc)
		
		db.shouldFail["list_active"] = true
		
		// Should not panic
		scheduler.processTasks()
	})
	
	t.Run("update_error", func(t *testing.T) {
		db := NewMockSchedulerDatabase()
		svc := NewMockSchedulerGameserverService()
		scheduler := NewTaskScheduler(db, svc)
		
		now := time.Now()
		task := &models.ScheduledTask{
			ID:           "test-task",
			GameserverID: "test-gs",
			Type:         models.TaskTypeBackup,
			Status:       models.TaskStatusActive,
			CronSchedule: "* * * * *",
			NextRun:      &now,
		}
		db.tasks[task.ID] = task
		
		// Setup gameserver
		svc.gameservers["test-gs"] = &models.Gameserver{
			ID:     "test-gs",
			Status: models.StatusRunning,
		}
		
		db.shouldFail["update"] = true
		
		// Should not panic even if update fails
		scheduler.processTasks()
	})
}

func TestCronFieldMatches(t *testing.T) {
	tests := []struct {
		name     string
		field    string
		value    int
		expected bool
	}{
		{"asterisk matches any", "*", 5, true},
		{"exact match", "5", 5, true},
		{"exact mismatch", "5", 6, false},
		{"step value match", "*/5", 10, true},
		{"step value mismatch", "*/5", 11, false},
		{"step value every 2", "*/2", 4, true},
		{"step value every 2 odd", "*/2", 5, false},
		{"invalid step", "*/0", 5, false},
		{"invalid pattern", "abc", 5, false},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := cronFieldMatches(tt.field, tt.value)
			if result != tt.expected {
				t.Errorf("cronFieldMatches(%q, %d) = %v, expected %v",
					tt.field, tt.value, result, tt.expected)
			}
		})
	}
}
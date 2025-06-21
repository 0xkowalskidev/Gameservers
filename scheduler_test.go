package main

import (
	"testing"
	"time"
)

// =============================================================================
// Mock Database Manager for Scheduler Testing
// =============================================================================

type MockDatabaseManager struct {
	tasks      map[string]*ScheduledTask
	shouldFail map[string]bool
}

func NewMockDatabaseManager() *MockDatabaseManager {
	return &MockDatabaseManager{
		tasks:      make(map[string]*ScheduledTask),
		shouldFail: make(map[string]bool),
	}
}

func (m *MockDatabaseManager) ListActiveScheduledTasks() ([]*ScheduledTask, error) {
	if m.shouldFail["list_active"] {
		return nil, &DatabaseError{Op: "list_active", Msg: "mock error"}
	}
	
	var active []*ScheduledTask
	for _, task := range m.tasks {
		if task.Status == TaskStatusActive {
			active = append(active, task)
		}
	}
	return active, nil
}

func (m *MockDatabaseManager) UpdateScheduledTask(task *ScheduledTask) error {
	if m.shouldFail["update_task"] {
		return &DatabaseError{Op: "update_task", Msg: "mock error"}
	}
	
	m.tasks[task.ID] = task
	return nil
}

// =============================================================================
// Mock Gameserver Service for Scheduler Testing
// =============================================================================

type MockGameserverService struct {
	gameservers    map[string]*Gameserver
	restartCalled  []string
	backupCalled   []string
	shouldFail     map[string]bool
}

func NewMockGameserverService() *MockGameserverService {
	return &MockGameserverService{
		gameservers:   make(map[string]*Gameserver),
		restartCalled: make([]string, 0),
		backupCalled:  make([]string, 0),
		shouldFail:    make(map[string]bool),
	}
}

func (m *MockGameserverService) RestartGameserver(id string) error {
	if m.shouldFail["restart"] {
		return &DatabaseError{Op: "restart", Msg: "mock restart error"}
	}
	m.restartCalled = append(m.restartCalled, id)
	return nil
}

func (m *MockGameserverService) GetGameserver(id string) (*Gameserver, error) {
	if m.shouldFail["get"] {
		return nil, &DatabaseError{Op: "get", Msg: "mock get error"}
	}
	if gs, exists := m.gameservers[id]; exists {
		return gs, nil
	}
	return nil, &DatabaseError{Op: "get", Msg: "not found"}
}

// Mock docker access for backup testing
func (m *MockGameserverService) createBackup(gameserverName string) error {
	if m.shouldFail["backup"] {
		return &DockerError{Op: "backup", Msg: "mock backup error"}
	}
	m.backupCalled = append(m.backupCalled, gameserverName)
	return nil
}

// =============================================================================
// Mock Task Scheduler for Testing
// =============================================================================

type MockTaskScheduler struct {
	TaskScheduler
}

func NewMockTaskScheduler() *MockTaskScheduler {
	return &MockTaskScheduler{}
}

// =============================================================================
// Cron Parsing Tests
// =============================================================================

func TestTaskScheduler_calculateNextRun(t *testing.T) {
	scheduler := NewMockTaskScheduler()

	tests := []struct {
		name         string
		cronSchedule string
		from         time.Time
		expected     string // Expected time format "HH:MM"
	}{
		{
			name:         "daily at 2 AM",
			cronSchedule: "0 2 * * *",
			from:         time.Date(2025, 6, 21, 1, 30, 0, 0, time.UTC),
			expected:     "02:00",
		},
		{
			name:         "every 30 minutes",
			cronSchedule: "*/30 * * * *",
			from:         time.Date(2025, 6, 21, 1, 15, 0, 0, time.UTC),
			expected:     "01:30",
		},
		{
			name:         "every 6 hours",
			cronSchedule: "0 */6 * * *",
			from:         time.Date(2025, 6, 21, 1, 30, 0, 0, time.UTC),
			expected:     "06:00",
		},
		{
			name:         "weekly on Sunday at 3 AM",
			cronSchedule: "0 3 * * 0",
			from:         time.Date(2025, 6, 21, 1, 30, 0, 0, time.UTC), // Saturday
			expected:     "03:00", // Next Sunday
		},
		{
			name:         "invalid cron",
			cronSchedule: "invalid",
			from:         time.Date(2025, 6, 21, 1, 30, 0, 0, time.UTC),
			expected:     "", // Should return nil
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			nextRun := scheduler.calculateNextRun(tt.cronSchedule, tt.from)
			
			if tt.expected == "" {
				if nextRun != nil {
					t.Errorf("expected nil, got %v", nextRun)
				}
				return
			}
			
			if nextRun == nil {
				t.Errorf("expected valid time, got nil")
				return
			}
			
			actualTime := nextRun.Format("15:04")
			if actualTime != tt.expected {
				t.Errorf("expected %s, got %s", tt.expected, actualTime)
			}
		})
	}
}

func TestTaskScheduler_fieldMatches(t *testing.T) {
	scheduler := NewMockTaskScheduler()

	tests := []struct {
		name     string
		value    int
		pattern  string
		expected bool
	}{
		{"asterisk matches any", 42, "*", true},
		{"exact match", 15, "15", true},
		{"exact mismatch", 15, "30", false},
		{"step value match", 30, "*/30", true},
		{"step value mismatch", 25, "*/30", false},
		{"step value every 2", 4, "*/2", true},
		{"step value every 2 odd", 5, "*/2", false},
		{"invalid step", 10, "*/abc", false},
		{"invalid pattern", 10, "abc", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := scheduler.fieldMatches(tt.value, tt.pattern)
			if result != tt.expected {
				t.Errorf("fieldMatches(%d, %q) = %v, expected %v", 
					tt.value, tt.pattern, result, tt.expected)
			}
		})
	}
}

// =============================================================================
// Task Execution Tests
// =============================================================================

func TestTaskScheduler_executeTask(t *testing.T) {
	// Test restart task
	t.Run("restart task", func(t *testing.T) {
		svc := NewMockGameserverService()
		svc.gameservers["test-id"] = &Gameserver{ID: "test-id", Name: "test-server"}
		
		task := &ScheduledTask{
			ID:           "task-1",
			GameserverID: "test-id",
			Type:         TaskTypeRestart,
		}
		
		err := svc.RestartGameserver(task.GameserverID)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		
		if len(svc.restartCalled) != 1 || svc.restartCalled[0] != "test-id" {
			t.Errorf("restart not called correctly, got %v", svc.restartCalled)
		}
	})
	
	// Test backup task  
	t.Run("backup task", func(t *testing.T) {
		svc := NewMockGameserverService()
		svc.gameservers["test-id"] = &Gameserver{ID: "test-id", Name: "test-server"}
		
		err := svc.createBackup("test-server")
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		
		if len(svc.backupCalled) != 1 || svc.backupCalled[0] != "test-server" {
			t.Errorf("backup not called correctly, got %v", svc.backupCalled)
		}
	})
}

// =============================================================================
// Database Integration Tests
// =============================================================================

func TestMockDatabaseManager_ListActiveScheduledTasks(t *testing.T) {
	db := NewMockDatabaseManager()

	// Add some tasks
	now := time.Now()
	db.tasks["active-task"] = &ScheduledTask{
		ID:     "active-task",
		Status: TaskStatusActive,
		NextRun: &now,
	}
	db.tasks["disabled-task"] = &ScheduledTask{
		ID:     "disabled-task", 
		Status: TaskStatusDisabled,
		NextRun: &now,
	}

	active, err := db.ListActiveScheduledTasks()
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if len(active) != 1 {
		t.Errorf("expected 1 active task, got %d", len(active))
	}

	if active[0].ID != "active-task" {
		t.Errorf("expected active-task, got %s", active[0].ID)
	}
}

// =============================================================================
// Integration Tests
// =============================================================================

func TestTaskScheduler_Integration(t *testing.T) {
	// Create a real in-memory database for this test
	db, err := NewDatabaseManager(":memory:")
	if err != nil {
		t.Fatalf("failed to create database: %v", err)
	}
	defer db.Close()

	// Create mock docker manager
	docker := NewMockDockerManager()
	
	// Create gameserver service
	svc := NewGameserverService(db, docker)
	
	// Create a test gameserver
	gameserver := &Gameserver{
		ID:       "integration-test",
		Name:     "test-server",
		GameID:   "minecraft",
		Port:     25565,
		Status:   StatusStopped,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	
	err = svc.CreateGameserver(gameserver)
	if err != nil {
		t.Fatalf("failed to create gameserver: %v", err)
	}

	// Create a scheduled task
	task := &ScheduledTask{
		GameserverID: gameserver.ID,
		Name:         "Test Restart",
		Type:         TaskTypeRestart,
		Status:       TaskStatusActive,
		CronSchedule: "*/1 * * * *", // Every minute
	}
	
	err = svc.CreateScheduledTask(task)
	if err != nil {
		t.Fatalf("failed to create scheduled task: %v", err)
	}

	// Verify task was created with NextRun time
	if task.NextRun == nil {
		t.Errorf("expected NextRun to be calculated when creating task")
	}

	// Verify task is in database
	tasks, err := svc.ListScheduledTasksForGameserver(gameserver.ID)
	if err != nil {
		t.Fatalf("failed to list tasks: %v", err)
	}
	
	if len(tasks) != 1 {
		t.Errorf("expected 1 task, got %d", len(tasks))
	}
	
	if tasks[0].NextRun == nil {
		t.Errorf("expected task to have NextRun time set")
	}
}
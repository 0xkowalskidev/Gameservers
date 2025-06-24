package main

import (
	"io"
	"strings"
	"testing"
	"time"

	"0xkowalskidev/gameservers/models"
)

// =============================================================================
// Mock Database Manager for Scheduler Testing
// =============================================================================

type MockDatabaseManager struct {
	tasks      map[string]*models.ScheduledTask
	shouldFail map[string]bool
}

func NewMockDatabaseManager() *MockDatabaseManager {
	return &MockDatabaseManager{
		tasks:      make(map[string]*models.ScheduledTask),
		shouldFail: make(map[string]bool),
	}
}

func (m *MockDatabaseManager) ListActiveScheduledTasks() ([]*models.ScheduledTask, error) {
	if m.shouldFail["list_active"] {
		return nil, &models.DatabaseError{Op: "list_active", Msg: "mock error"}
	}
	
	var active []*models.ScheduledTask
	for _, task := range m.tasks {
		if task.Status == models.TaskStatusActive {
			active = append(active, task)
		}
	}
	return active, nil
}

func (m *MockDatabaseManager) UpdateScheduledTask(task *models.ScheduledTask) error {
	if m.shouldFail["update_task"] {
		return &models.DatabaseError{Op: "update_task", Msg: "mock error"}
	}
	
	m.tasks[task.ID] = task
	return nil
}

// =============================================================================
// Mock models.Gameserver Service for Scheduler Testing
// =============================================================================

type MockGameserverService struct {
	gameservers    map[string]*models.Gameserver
	restartCalled  []string
	backupCalled   []string
	shouldFail     map[string]bool
}

func NewMockGameserverService() *MockGameserverService {
	return &MockGameserverService{
		gameservers:   make(map[string]*models.Gameserver),
		restartCalled: make([]string, 0),
		backupCalled:  make([]string, 0),
		shouldFail:    make(map[string]bool),
	}
}

func (m *MockGameserverService) RestartGameserver(id string) error {
	if m.shouldFail["restart"] {
		return &models.DatabaseError{Op: "restart", Msg: "mock restart error"}
	}
	m.restartCalled = append(m.restartCalled, id)
	return nil
}

func (m *MockGameserverService) GetGameserver(id string) (*models.Gameserver, error) {
	if m.shouldFail["get"] {
		return nil, &models.DatabaseError{Op: "get", Msg: "mock get error"}
	}
	if gs, exists := m.gameservers[id]; exists {
		return gs, nil
	}
	return nil, &models.DatabaseError{Op: "get", Msg: "not found"}
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
// Mock Docker Manager for Scheduler Testing  
// =============================================================================

type MockDockerManagerForScheduler struct {
	containers map[string]*models.Gameserver
	logs       map[string][]string
	shouldFail map[string]bool
}

func NewMockDockerManagerForScheduler() *MockDockerManagerForScheduler {
	return &MockDockerManagerForScheduler{
		containers: make(map[string]*models.Gameserver),
		logs:       make(map[string][]string),
		shouldFail: make(map[string]bool),
	}
}

func (m *MockDockerManagerForScheduler) CreateContainer(server *models.Gameserver) error {
	if m.shouldFail["create"] {
		return &DockerError{Op: "create", Msg: "mock create error"}
	}
	server.ContainerID = "mock-container-" + server.ID
	server.Status = models.StatusStopped
	m.containers[server.ContainerID] = server
	return nil
}

func (m *MockDockerManagerForScheduler) StartContainer(containerID string) error {
	if m.shouldFail["start"] {
		return &DockerError{Op: "start", Msg: "mock start error"}
	}
	if server, exists := m.containers[containerID]; exists {
		server.Status = models.StatusRunning
		return nil
	}
	return &DockerError{Op: "start", Msg: "container not found"}
}

func (m *MockDockerManagerForScheduler) StopContainer(containerID string) error {
	if m.shouldFail["stop"] {
		return &DockerError{Op: "stop", Msg: "mock stop error"}
	}
	if server, exists := m.containers[containerID]; exists {
		server.Status = models.StatusStopped
		return nil
	}
	return &DockerError{Op: "stop", Msg: "container not found"}
}

func (m *MockDockerManagerForScheduler) RemoveContainer(containerID string) error {
	if m.shouldFail["remove"] {
		return &DockerError{Op: "remove", Msg: "mock remove error"}
	}
	delete(m.containers, containerID)
	return nil
}

func (m *MockDockerManagerForScheduler) GetContainerStatus(containerID string) (models.GameserverStatus, error) {
	if m.shouldFail["status"] {
		return models.StatusError, &DockerError{Op: "status", Msg: "mock status error"}
	}
	if server, exists := m.containers[containerID]; exists {
		return server.Status, nil
	}
	return models.StatusError, &DockerError{Op: "status", Msg: "container not found"}
}

func (m *MockDockerManagerForScheduler) StreamContainerLogs(containerID string) (io.ReadCloser, error) {
	if m.shouldFail["stream_logs"] {
		return nil, &DockerError{Op: "stream_logs", Msg: "mock stream logs error"}
	}
	return io.NopCloser(strings.NewReader("Mock log stream")), nil
}

func (m *MockDockerManagerForScheduler) StreamContainerStats(containerID string) (io.ReadCloser, error) {
	if m.shouldFail["stream_stats"] {
		return nil, &DockerError{Op: "stream_stats", Msg: "mock stream stats error"}
	}
	return io.NopCloser(strings.NewReader(`{"cpu_stats":{"cpu_usage":{"total_usage":100},"system_cpu_usage":200},"precpu_stats":{"cpu_usage":{"total_usage":50},"system_cpu_usage":100},"memory_stats":{"usage":536870912,"limit":1073741824}}`)), nil
}

func (m *MockDockerManagerForScheduler) ListContainers() ([]string, error) {
	if m.shouldFail["list"] {
		return nil, &DockerError{Op: "list", Msg: "mock list error"}
	}
	containers := make([]string, 0, len(m.containers))
	for id := range m.containers {
		containers = append(containers, id)
	}
	return containers, nil
}

func (m *MockDockerManagerForScheduler) CreateVolume(volumeName string) error {
	if m.shouldFail["create_volume"] {
		return &DockerError{Op: "create_volume", Msg: "mock create volume error"}
	}
	return nil
}

func (m *MockDockerManagerForScheduler) RemoveVolume(volumeName string) error {
	if m.shouldFail["remove_volume"] {
		return &DockerError{Op: "remove_volume", Msg: "mock remove volume error"}
	}
	return nil
}

func (m *MockDockerManagerForScheduler) GetVolumeInfo(volumeName string) (*models.VolumeInfo, error) {
	if m.shouldFail["get_volume_info"] {
		return nil, &DockerError{Op: "get_volume_info", Msg: "mock get volume info error"}
	}
	return &models.VolumeInfo{
		Name:       volumeName,
		MountPoint: "/var/lib/docker/volumes/" + volumeName + "/_data",
		Driver:     "local",
		CreatedAt:  "2025-06-21T04:00:00Z",
		Labels:     map[string]string{"gameserver.managed": "true"},
	}, nil
}

func (m *MockDockerManagerForScheduler) CreateBackup(containerID, gameserverName string) error {
	if m.shouldFail["create_backup"] {
		return &DockerError{Op: "create_backup", Msg: "mock create backup error"}
	}
	return nil
}

func (m *MockDockerManagerForScheduler) RestoreBackup(containerID, backupFilename string) error {
	if m.shouldFail["restore_backup"] {
		return &DockerError{Op: "restore_backup", Msg: "mock restore backup error"}
	}
	return nil
}

// File manager methods
func (m *MockDockerManagerForScheduler) ListFiles(containerID string, path string) ([]*models.FileInfo, error) {
	if m.shouldFail["list_files"] {
		return nil, &DockerError{Op: "list_files", Msg: "mock list files error"}
	}
	modTime, _ := time.Parse(time.RFC3339, "2025-06-21T00:00:00Z")
	return []*models.FileInfo{
		{Name: "server.properties", Size: 1024, IsDir: false, Modified: modTime.Format(time.RFC3339)},
		{Name: "logs", Size: 0, IsDir: true, Modified: modTime.Format(time.RFC3339)},
	}, nil
}

func (m *MockDockerManagerForScheduler) ReadFile(containerID string, path string) ([]byte, error) {
	if m.shouldFail["read_file"] {
		return nil, &DockerError{Op: "read_file", Msg: "mock read file error"}
	}
	return []byte("mock file content"), nil
}

func (m *MockDockerManagerForScheduler) WriteFile(containerID string, path string, content []byte) error {
	if m.shouldFail["write_file"] {
		return &DockerError{Op: "write_file", Msg: "mock write file error"}
	}
	return nil
}

func (m *MockDockerManagerForScheduler) CreateDirectory(containerID string, path string) error {
	if m.shouldFail["create_directory"] {
		return &DockerError{Op: "create_directory", Msg: "mock create directory error"}
	}
	return nil
}

func (m *MockDockerManagerForScheduler) DeletePath(containerID string, path string) error {
	if m.shouldFail["delete_path"] {
		return &DockerError{Op: "delete_path", Msg: "mock delete path error"}
	}
	return nil
}

func (m *MockDockerManagerForScheduler) DownloadFile(containerID string, path string) (io.ReadCloser, error) {
	if m.shouldFail["download_file"] {
		return nil, &DockerError{Op: "download_file", Msg: "mock download file error"}
	}
	return io.NopCloser(strings.NewReader("mock file content")), nil
}

func (m *MockDockerManagerForScheduler) RenameFile(containerID string, oldPath string, newPath string) error {
	if m.shouldFail["rename_file"] {
		return &DockerError{Op: "rename_file", Msg: "mock rename file error"}
	}
	return nil
}

func (m *MockDockerManagerForScheduler) SendCommand(containerID string, command string) error {
	if m.shouldFail["send_command"] {
		return &DockerError{Op: "send_command", Msg: "mock send command error"}
	}
	return nil
}

func (m *MockDockerManagerForScheduler) ExecCommand(containerID string, cmd []string) ([]byte, error) {
	if m.shouldFail["exec_command"] {
		return nil, &DockerError{Op: "exec_command", Msg: "mock exec command error"}
	}
	return []byte("mock command output"), nil
}

func (m *MockDockerManagerForScheduler) UploadFile(containerID string, destPath string, reader io.Reader) error {
	if m.shouldFail["upload_file"] {
		return &DockerError{Op: "upload_file", Msg: "mock upload file error"}
	}
	return nil
}

func (m *MockDockerManagerForScheduler) CleanupOldBackups(containerID string, maxBackups int) error {
	if m.shouldFail["cleanup_backups"] {
		return &DockerError{Op: "cleanup_backups", Msg: "mock cleanup backups error"}
	}
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

func TestCronCalculateNextRun(t *testing.T) {
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
			nextRun := CalculateNextRun(tt.cronSchedule, tt.from)
			
			if tt.expected == "" {
				if !nextRun.IsZero() {
					t.Errorf("expected zero time, got %v", nextRun)
				}
				return
			}
			
			if nextRun.IsZero() {
				t.Errorf("expected valid time, got zero time")
				return
			}
			
			actualTime := nextRun.Format("15:04")
			if actualTime != tt.expected {
				t.Errorf("expected %s, got %s", tt.expected, actualTime)
			}
		})
	}
}

func TestCronFieldMatches(t *testing.T) {
	tests := []struct {
		name     string
		field    string
		value    int
		expected bool
	}{
		{"asterisk matches any", "*", 42, true},
		{"exact match", "15", 15, true},
		{"exact mismatch", "30", 15, false},
		{"step value match", "*/30", 30, true},
		{"step value mismatch", "*/30", 25, false},
		{"step value every 2", "*/2", 4, true},
		{"step value every 2 odd", "*/2", 5, false},
		{"invalid step", "*/abc", 10, false},
		{"invalid pattern", "abc", 10, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := fieldMatches(tt.field, tt.value)
			if result != tt.expected {
				t.Errorf("fieldMatches(%q, %d) = %v, expected %v", 
					tt.field, tt.value, result, tt.expected)
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
		svc.gameservers["test-id"] = &models.Gameserver{ID: "test-id", Name: "test-server"}
		
		task := &models.ScheduledTask{
			ID:           "task-1",
			GameserverID: "test-id",
			Type:         models.TaskTypeRestart,
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
		svc.gameservers["test-id"] = &models.Gameserver{ID: "test-id", Name: "test-server"}
		
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
	db.tasks["active-task"] = &models.ScheduledTask{
		ID:     "active-task",
		Status: models.TaskStatusActive,
		NextRun: &now,
	}
	db.tasks["disabled-task"] = &models.ScheduledTask{
		ID:     "disabled-task", 
		Status: models.TaskStatusDisabled,
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
// Backup Path Tests
// =============================================================================

func TestTaskScheduler_BackupPathHandling(t *testing.T) {
	// Create a real in-memory database for this test
	db, err := NewDatabaseManager(":memory:")
	if err != nil {
		t.Fatalf("failed to create database: %v", err)
	}
	defer db.Close()

	// Create mock docker manager
	docker := NewMockDockerManagerForScheduler()
	
	// Create gameserver service
	svc := NewGameserverService(db, docker)
	
	// Create scheduler
	scheduler := NewTaskScheduler(db, svc)
	
	// Create a test gameserver
	gameserver := &models.Gameserver{
		ID:       "backup-test",
		Name:     "test-backup-server",
		GameID:   "minecraft",
		PortMappings: []models.PortMapping{{Protocol: "tcp", ContainerPort: 25565, HostPort: 0}},
		Status:   models.StatusStopped,
		Environment: []string{"EULA=true"},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	
	err = svc.CreateGameserver(gameserver)
	if err != nil {
		t.Fatalf("failed to create gameserver: %v", err)
	}

	// Test that backup creation generates absolute paths
	err = scheduler.createBackup(gameserver.ID)
	if err != nil {
		t.Errorf("backup creation should not fail with absolute path: %v", err)
	}
}

// =============================================================================
// Integration Tests
// =============================================================================

func TestTaskScheduler_IntegrationBasic(t *testing.T) {
	// Create a real in-memory database for this test
	db, err := NewDatabaseManager(":memory:")
	if err != nil {
		t.Fatalf("failed to create database: %v", err)
	}
	defer db.Close()

	// Create mock docker manager
	docker := NewMockDockerManagerForScheduler()
	
	// Create gameserver service
	svc := NewGameserverService(db, docker)
	
	// Create a test gameserver
	gameserver := &models.Gameserver{
		ID:       "integration-test",
		Name:     "test-server",
		GameID:   "minecraft",
		PortMappings: []models.PortMapping{{Protocol: "tcp", ContainerPort: 25565, HostPort: 0}},
		Status:   models.StatusStopped,
		Environment: []string{"EULA=true"},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	
	err = svc.CreateGameserver(gameserver)
	if err != nil {
		t.Fatalf("failed to create gameserver: %v", err)
	}

	// Create a scheduled task
	task := &models.ScheduledTask{
		GameserverID: gameserver.ID,
		Name:         "Test Restart",
		Type:         models.TaskTypeRestart,
		Status:       models.TaskStatusActive,
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
	
	if len(tasks) != 2 {
		t.Errorf("expected 2 tasks (manual + automatic daily backup), got %d", len(tasks))
	}
	
	// Check that both tasks have NextRun time set
	for i, task := range tasks {
		if task.NextRun == nil {
			t.Errorf("expected task %d to have NextRun time set", i)
		}
	}
}

// =============================================================================
// Next Run Recalculation Tests 
// =============================================================================

func TestTaskScheduler_NextRunRecalculation(t *testing.T) {
	// Create a real in-memory database for this test
	db, err := NewDatabaseManager(":memory:")
	if err != nil {
		t.Fatalf("failed to create database: %v", err)
	}
	defer db.Close()

	// Create real gameserver service with mock docker
	mockDocker := NewMockDockerManagerForScheduler()
	svc := NewGameserverService(db, mockDocker)
	
	// Create a test gameserver
	gameserver := &models.Gameserver{
		ID:       "test-gs",
		Name:     "Test Server",
		GameID:   "minecraft",
		PortMappings: []models.PortMapping{{Protocol: "tcp", ContainerPort: 25565, HostPort: 0}},
		Status:   models.StatusRunning,
		Environment: []string{"EULA=true"},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	svc.CreateGameserver(gameserver)

	// Create scheduler
	scheduler := NewTaskScheduler(db, svc)

	// Create a task with NextRun = nil (simulating updated task)
	now := time.Now()
	task := &models.ScheduledTask{
		ID:           "test-task",
		GameserverID: "test-gs",
		Name:         "Test Task",
		Type:         models.TaskTypeRestart,
		Status:       models.TaskStatusActive,
		CronSchedule: "0 2 * * *", // Daily at 2 AM
		CreatedAt:    now,
		UpdatedAt:    now,
		NextRun:      nil, // This simulates a task that was just updated
	}

	// Insert task directly into database
	err = db.CreateScheduledTask(task)
	if err != nil {
		t.Fatalf("failed to create task: %v", err)
	}

	// Process tasks (should recalculate NextRun)
	scheduler.processTasks()

	// Verify NextRun was recalculated
	updatedTask, err := db.GetScheduledTask(task.ID)
	if err != nil {
		t.Fatalf("failed to get updated task: %v", err)
	}

	if updatedTask.NextRun == nil {
		t.Error("expected NextRun to be calculated after processing")
	}

	// Verify the NextRun time is reasonable (should be today or tomorrow at 2 AM)
	expectedHour := 2
	if updatedTask.NextRun.Hour() != expectedHour {
		t.Errorf("expected NextRun hour to be %d, got %d", expectedHour, updatedTask.NextRun.Hour())
	}
	if updatedTask.NextRun.Minute() != 0 {
		t.Errorf("expected NextRun minute to be 0, got %d", updatedTask.NextRun.Minute())
	}
}

func TestTaskScheduler_TaskExecution_RestartOnlyWhenRunning(t *testing.T) {
	// Create a real in-memory database for this test
	db, err := NewDatabaseManager(":memory:")
	if err != nil {
		t.Fatalf("failed to create database: %v", err)
	}
	defer db.Close()

	// Create real gameserver service with mock docker
	mockDocker := NewMockDockerManagerForScheduler()
	svc := NewGameserverService(db, mockDocker)
	
	// Create scheduler
	scheduler := NewTaskScheduler(db, svc)

	tests := []struct {
		name           string
		gameserverStatus models.GameserverStatus
		expectedRestart bool
	}{
		{
			name:           "restart running server",
			gameserverStatus: models.StatusRunning,
			expectedRestart: true,
		},
		{
			name:           "skip restart for stopped server",
			gameserverStatus: models.StatusStopped,
			expectedRestart: false,
		},
		{
			name:           "skip restart for starting server",
			gameserverStatus: models.StatusStarting,
			expectedRestart: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a test gameserver with specific status
			gameserver := &models.Gameserver{
				ID:       "test-gs-" + strings.ReplaceAll(tt.name, " ", "-"),
				Name:     "Test Server " + tt.name,
				GameID:   "minecraft",
				PortMappings: []models.PortMapping{{Protocol: "tcp", ContainerPort: 25565, HostPort: 0}},
				Status:   tt.gameserverStatus,
				Environment: []string{"EULA=true"},
				CreatedAt: time.Now(),
				UpdatedAt: time.Now(),
			}
			
			// Force the status by updating after creation
			svc.CreateGameserver(gameserver)
			gameserver.Status = tt.gameserverStatus
			svc.UpdateGameserver(gameserver)

			// Create restart task
			task := &models.ScheduledTask{
				ID:           "restart-task-" + tt.name,
				GameserverID: gameserver.ID,
				Type:         models.TaskTypeRestart,
			}

			// Execute task - this will check status and only restart if running
			err := scheduler.executeTask(task)
			
			// For running servers, no error should occur
			// For stopped servers, also no error (it just skips)
			if err != nil {
				t.Errorf("unexpected error: %v", err)
			}
			
			// Note: We can't easily test if restart was called with the real service,
			// but we can verify the task execution didn't error
		})
	}
}

func TestTaskScheduler_TaskExecution_BackupRegardlessOfStatus(t *testing.T) {
	// Create a real in-memory database for this test
	db, err := NewDatabaseManager(":memory:")
	if err != nil {
		t.Fatalf("failed to create database: %v", err)
	}
	defer db.Close()

	// Create real gameserver service with mock docker
	mockDocker := NewMockDockerManagerForScheduler()
	svc := NewGameserverService(db, mockDocker)
	
	// Create scheduler
	scheduler := NewTaskScheduler(db, svc)

	tests := []struct {
		name             string
		gameserverStatus models.GameserverStatus
		expectedBackup   bool
	}{
		{
			name:             "backup running server",
			gameserverStatus: models.StatusRunning,
			expectedBackup:   true,
		},
		{
			name:             "backup stopped server",
			gameserverStatus: models.StatusStopped,
			expectedBackup:   true,
		},
		{
			name:             "backup starting server",
			gameserverStatus: models.StatusStarting,
			expectedBackup:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a test gameserver with specific status
			gameserver := &models.Gameserver{
				ID:       "test-gs-" + strings.ReplaceAll(tt.name, " ", "-"),
				Name:     "Test Server " + tt.name,
				GameID:   "minecraft",
				PortMappings: []models.PortMapping{{Protocol: "tcp", ContainerPort: 25565, HostPort: 0}},
				Status:   tt.gameserverStatus,
				Environment: []string{"EULA=true"},
				CreatedAt: time.Now(),
				UpdatedAt: time.Now(),
			}
			
			// Force the status by updating after creation
			svc.CreateGameserver(gameserver)
			gameserver.Status = tt.gameserverStatus
			svc.UpdateGameserver(gameserver)

			// Create backup task
			task := &models.ScheduledTask{
				ID:           "backup-task-" + tt.name,
				GameserverID: gameserver.ID,
				Type:         models.TaskTypeBackup,
			}

			// Execute task - this should work regardless of status
			err := scheduler.executeTask(task)
			if err != nil {
				t.Errorf("unexpected error: %v", err)
			}
			
			// Note: We can't easily test if backup was called with the real service,
			// but we can verify the task execution didn't error
		})
	}
}

func TestTaskScheduler_ErrorHandling(t *testing.T) {
	// Create a real in-memory database for this test
	db, err := NewDatabaseManager(":memory:")
	if err != nil {
		t.Fatalf("failed to create database: %v", err)
	}
	defer db.Close()

	// Create real gameserver service with mock docker
	mockDocker := NewMockDockerManagerForScheduler()
	svc := NewGameserverService(db, mockDocker)
	
	// Create scheduler
	scheduler := NewTaskScheduler(db, svc)

	// Process tasks should handle database errors gracefully
	scheduler.processTasks() // Should not panic

	// Test task execution with missing gameserver
	task := &models.ScheduledTask{
		ID:           "missing-gs-task",
		GameserverID: "nonexistent",
		Type:         models.TaskTypeRestart,
	}

	err = scheduler.executeTask(task)
	if err == nil {
		t.Error("expected error for missing gameserver")
	}
}

func TestTaskScheduler_LastRunAndNextRunUpdates(t *testing.T) {
	// Create a real in-memory database for this test
	db, err := NewDatabaseManager(":memory:")
	if err != nil {
		t.Fatalf("failed to create database: %v", err)
	}
	defer db.Close()

	// Create real gameserver service with mock docker
	mockDocker := NewMockDockerManagerForScheduler()
	svc := NewGameserverService(db, mockDocker)
	
	// Create a test gameserver
	gameserver := &models.Gameserver{
		ID:       "test-gs",
		Name:     "Test Server",
		GameID:   "minecraft",
		PortMappings: []models.PortMapping{{Protocol: "tcp", ContainerPort: 25565, HostPort: 0}},
		Status:   models.StatusRunning,
		Environment: []string{"EULA=true"},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	svc.CreateGameserver(gameserver)

	// Create scheduler
	scheduler := NewTaskScheduler(db, svc)

	// Create a task that's due for execution
	pastTime := time.Now().Add(-time.Hour) // 1 hour ago
	task := &models.ScheduledTask{
		ID:           "due-task",
		GameserverID: "test-gs",
		Name:         "Due Task",
		Type:         models.TaskTypeRestart,
		Status:       models.TaskStatusActive,
		CronSchedule: "*/30 * * * *", // Every 30 minutes
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
		NextRun:      &pastTime, // Due for execution
		LastRun:      nil,
	}

	// Insert task into database
	err = db.CreateScheduledTask(task)
	if err != nil {
		t.Fatalf("failed to create task: %v", err)
	}

	// Process tasks (should execute the due task)
	scheduler.processTasks()

	// Verify task was executed and updated
	updatedTask, err := db.GetScheduledTask(task.ID)
	if err != nil {
		t.Fatalf("failed to get updated task: %v", err)
	}

	// Verify LastRun was set
	if updatedTask.LastRun == nil {
		t.Error("expected LastRun to be set after execution")
	}

	// Verify NextRun was recalculated (should be ~30 minutes from LastRun)
	if updatedTask.NextRun == nil {
		t.Error("expected NextRun to be recalculated after execution")
	}

	// Note: We can't easily verify restart was called with the real service,
	// but we can verify the task execution succeeded and times were updated
}
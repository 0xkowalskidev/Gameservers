package main

import (
	"testing"
	"time"
)

func TestDatabaseManager_CRUD(t *testing.T) {
	db, err := NewDatabaseManager(":memory:")
	if err != nil {
		t.Fatalf("Failed to create database: %v", err)
	}
	defer db.Close()

	server := &Gameserver{
		ID: "test-1", Name: "Test Server", GameID: "minecraft",
		Port: 25565, Environment: []string{"ENV=prod"}, Volumes: []string{"/data:/mc"},
		Status: StatusStopped, CreatedAt: time.Now(), UpdatedAt: time.Now(),
	}

	// Create
	if err := db.CreateGameserver(server); err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	// Get
	retrieved, err := db.GetGameserver(server.ID)
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if retrieved.Name != server.Name || len(retrieved.Environment) != 1 {
		t.Errorf("Retrieved data mismatch")
	}

	// Update
	retrieved.Status = StatusRunning
	retrieved.ContainerID = "container-123"
	if err := db.UpdateGameserver(retrieved); err != nil {
		t.Fatalf("Update failed: %v", err)
	}

	updated, _ := db.GetGameserver(server.ID)
	if updated.Status != StatusRunning || updated.ContainerID != "container-123" {
		t.Errorf("Update data mismatch")
	}

	// List
	servers, _ := db.ListGameservers()
	if len(servers) != 1 {
		t.Errorf("Expected 1 server, got %d", len(servers))
	}

	// Delete
	if err := db.DeleteGameserver(server.ID); err != nil {
		t.Fatalf("Delete failed: %v", err)
	}
	if _, err := db.GetGameserver(server.ID); err == nil {
		t.Error("Expected error after deletion")
	}
}

func TestDatabaseManager_DuplicateName(t *testing.T) {
	db, _ := NewDatabaseManager(":memory:")
	defer db.Close()

	server1 := &Gameserver{ID: "1", Name: "dup", GameID: "minecraft", Port: 25565, Status: StatusStopped, CreatedAt: time.Now(), UpdatedAt: time.Now()}
	server2 := &Gameserver{ID: "2", Name: "dup", GameID: "minecraft", Port: 25566, Status: StatusStopped, CreatedAt: time.Now(), UpdatedAt: time.Now()}

	db.CreateGameserver(server1)
	if err := db.CreateGameserver(server2); err == nil {
		t.Error("Expected duplicate name error")
	}
}

// =============================================================================
// Scheduled Task Database Tests
// =============================================================================

func TestDatabaseManager_ScheduledTaskCRUD(t *testing.T) {
	db, err := NewDatabaseManager(":memory:")
	if err != nil {
		t.Fatalf("Failed to create database: %v", err)
	}
	defer db.Close()

	// Create a gameserver first
	gameserver := &Gameserver{
		ID: "test-gs", Name: "Test Server", GameID: "minecraft", 
		Port: 25565, Status: StatusStopped, 
		CreatedAt: time.Now(), UpdatedAt: time.Now(),
	}
	if err := db.CreateGameserver(gameserver); err != nil {
		t.Fatalf("Failed to create gameserver: %v", err)
	}

	// Create scheduled task
	now := time.Now()
	nextRun := now.Add(time.Hour)
	task := &ScheduledTask{
		ID:           "test-task",
		GameserverID: gameserver.ID,
		Name:         "Test Restart",
		Type:         TaskTypeRestart,
		Status:       TaskStatusActive,
		CronSchedule: "0 2 * * *",
		CreatedAt:    now,
		UpdatedAt:    now,
		NextRun:      &nextRun,
	}

	// Create
	if err := db.CreateScheduledTask(task); err != nil {
		t.Fatalf("Create task failed: %v", err)
	}

	// Get
	retrieved, err := db.GetScheduledTask(task.ID)
	if err != nil {
		t.Fatalf("Get task failed: %v", err)
	}
	if retrieved.Name != task.Name || retrieved.Type != task.Type {
		t.Errorf("Retrieved task data mismatch")
	}
	if retrieved.NextRun == nil || retrieved.NextRun.Unix() != nextRun.Unix() {
		t.Errorf("NextRun time mismatch")
	}

	// Update
	retrieved.Status = TaskStatusDisabled
	retrieved.Name = "Updated Task"
	if err := db.UpdateScheduledTask(retrieved); err != nil {
		t.Fatalf("Update task failed: %v", err)
	}

	updated, _ := db.GetScheduledTask(task.ID)
	if updated.Status != TaskStatusDisabled || updated.Name != "Updated Task" {
		t.Errorf("Update task data mismatch")
	}

	// List for gameserver
	tasks, err := db.ListScheduledTasksForGameserver(gameserver.ID)
	if err != nil {
		t.Fatalf("List tasks failed: %v", err)
	}
	if len(tasks) != 1 {
		t.Errorf("Expected 1 task, got %d", len(tasks))
	}

	// List active tasks
	activeTasks, err := db.ListActiveScheduledTasks()
	if err != nil {
		t.Fatalf("List active tasks failed: %v", err)
	}
	if len(activeTasks) != 0 { // Should be 0 since we disabled the task
		t.Errorf("Expected 0 active tasks, got %d", len(activeTasks))
	}

	// Re-enable task
	updated.Status = TaskStatusActive
	db.UpdateScheduledTask(updated)

	activeTasks, _ = db.ListActiveScheduledTasks()
	if len(activeTasks) != 1 {
		t.Errorf("Expected 1 active task, got %d", len(activeTasks))
	}

	// Delete
	if err := db.DeleteScheduledTask(task.ID); err != nil {
		t.Fatalf("Delete task failed: %v", err)
	}
	if _, err := db.GetScheduledTask(task.ID); err == nil {
		t.Error("Expected error after deletion")
	}
}

func TestDatabaseManager_ScheduledTaskCascadeDelete(t *testing.T) {
	db, err := NewDatabaseManager(":memory:")
	if err != nil {
		t.Fatalf("Failed to create database: %v", err)
	}
	defer db.Close()

	// Create gameserver and task
	gameserver := &Gameserver{
		ID: "test-gs", Name: "Test Server", GameID: "minecraft", 
		Port: 25565, Status: StatusStopped, 
		CreatedAt: time.Now(), UpdatedAt: time.Now(),
	}
	db.CreateGameserver(gameserver)

	task := &ScheduledTask{
		ID:           "test-task",
		GameserverID: gameserver.ID,
		Name:         "Test Task",
		Type:         TaskTypeRestart,
		Status:       TaskStatusActive,
		CronSchedule: "0 2 * * *",
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}
	db.CreateScheduledTask(task)

	// Delete gameserver - should cascade delete the task
	if err := db.DeleteGameserver(gameserver.ID); err != nil {
		t.Fatalf("Delete gameserver failed: %v", err)
	}

	// Verify task was also deleted (cascade)
	if _, err := db.GetScheduledTask(task.ID); err == nil {
		t.Error("Expected task to be cascade deleted")
	}
}

// =============================================================================
// GameserverService Tests for New Features
// =============================================================================

func TestGameserverService_ScheduledTaskLifecycle(t *testing.T) {
	db, err := NewDatabaseManager(":memory:")
	if err != nil {
		t.Fatalf("Failed to create database: %v", err)
	}
	defer db.Close()

	// Mock docker manager
	mockDocker := NewMockDockerManager()

	svc := NewGameserverService(db, mockDocker)

	// Create gameserver first
	gameserver := &Gameserver{
		ID: "test-gs", Name: "Test Server", GameID: "minecraft", 
		Port: 25565, Status: StatusStopped,
	}

	err = svc.CreateGameserver(gameserver)
	if err != nil {
		t.Fatalf("Failed to create gameserver: %v", err)
	}

	// Test creating a scheduled task with next run calculation
	task := &ScheduledTask{
		GameserverID: gameserver.ID,
		Name:         "Test Task",
		Type:         TaskTypeRestart,
		Status:       TaskStatusActive,
		CronSchedule: "0 2 * * *", // Daily at 2 AM
	}

	err = svc.CreateScheduledTask(task)
	if err != nil {
		t.Fatalf("Failed to create scheduled task: %v", err)
	}

	// Verify task was created with NextRun calculated
	if task.NextRun == nil {
		t.Error("Expected NextRun to be calculated during task creation")
	}

	// Test updating task (should clear NextRun)
	task.Name = "Updated Task"
	task.CronSchedule = "0 3 * * *" // 3 AM instead
	err = svc.UpdateScheduledTask(task)
	if err != nil {
		t.Fatalf("Failed to update scheduled task: %v", err)
	}

	// Verify NextRun was cleared
	if task.NextRun != nil {
		t.Error("Expected NextRun to be cleared after update")
	}

	// Retrieve task and verify it's cleared in database too
	retrieved, err := svc.GetScheduledTask(task.ID)
	if err != nil {
		t.Fatalf("Failed to retrieve updated task: %v", err)
	}
	if retrieved.NextRun != nil {
		t.Error("Expected NextRun to be nil in database after update")
	}
}

func TestGameserverService_BackupOperations(t *testing.T) {
	db, err := NewDatabaseManager(":memory:")
	if err != nil {
		t.Fatalf("Failed to create database: %v", err)
	}
	defer db.Close()

	// Mock docker manager
	mockDocker := NewMockDockerManager()

	svc := NewGameserverService(db, mockDocker)

	// Create gameserver
	gameserver := &Gameserver{
		ID: "backup-test-gs", Name: "Backup Test Server", GameID: "minecraft", 
		Port: 25565, Status: StatusStopped, MaxBackups: 5,
	}

	err = svc.CreateGameserver(gameserver)
	if err != nil {
		t.Fatalf("Failed to create gameserver: %v", err)
	}

	// Test backup creation
	err = svc.CreateGameserverBackup(gameserver.ID)
	if err != nil {
		t.Errorf("Failed to create backup: %v", err)
	}

	// Test backup restoration
	err = svc.RestoreGameserverBackup(gameserver.ID, "test-backup.tar.gz")
	if err != nil {
		t.Errorf("Failed to restore backup: %v", err)
	}

	// Test backup creation failure
	mockDocker.shouldFail["create_backup"] = true
	err = svc.CreateGameserverBackup(gameserver.ID)
	if err == nil {
		t.Error("Expected backup creation to fail")
	}

	// Test backup restoration failure
	mockDocker.shouldFail["restore_backup"] = true
	err = svc.RestoreGameserverBackup(gameserver.ID, "test-backup.tar.gz")
	if err == nil {
		t.Error("Expected backup restoration to fail")
	}
}

func TestGameserverService_AutomaticDailyBackupTask(t *testing.T) {
	db, err := NewDatabaseManager(":memory:")
	if err != nil {
		t.Fatalf("Failed to create database: %v", err)
	}
	defer db.Close()

	// Mock docker manager
	mockDocker := NewMockDockerManager()

	svc := NewGameserverService(db, mockDocker)

	// Create gameserver
	gameserver := &Gameserver{
		ID: "auto-backup-test", Name: "Auto Backup Test", GameID: "minecraft", 
		Port: 25565, Status: StatusStopped,
	}

	err = svc.CreateGameserver(gameserver)
	if err != nil {
		t.Fatalf("Failed to create gameserver: %v", err)
	}

	// Verify automatic daily backup task was created
	tasks, err := svc.ListScheduledTasksForGameserver(gameserver.ID)
	if err != nil {
		t.Fatalf("Failed to list tasks: %v", err)
	}

	if len(tasks) != 1 {
		t.Errorf("Expected 1 automatic backup task, got %d", len(tasks))
	}

	backupTask := tasks[0]
	if backupTask.Type != TaskTypeBackup {
		t.Errorf("Expected backup task type, got %s", backupTask.Type)
	}
	if backupTask.CronSchedule != "0 2 * * *" {
		t.Errorf("Expected daily 2 AM schedule, got %s", backupTask.CronSchedule)
	}
	if backupTask.Name != "Daily Backup" {
		t.Errorf("Expected 'Daily Backup' name, got %s", backupTask.Name)
	}
	if backupTask.Status != TaskStatusActive {
		t.Errorf("Expected active status, got %s", backupTask.Status)
	}
}


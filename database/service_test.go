package database

import (
	"testing"

	"0xkowalskidev/gameservers/models"
)

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
	gameserver := &models.Gameserver{
		ID: "test-gs", Name: "Test Server", GameID: "minecraft", 
		PortMappings: []models.PortMapping{{Name: "game", Protocol: "tcp", ContainerPort: 25565, HostPort: 0}}, Status: models.StatusStopped,
		Environment: []string{"EULA=true"},
	}

	err = svc.CreateGameserver(gameserver)
	if err != nil {
		t.Fatalf("Failed to create gameserver: %v", err)
	}

	// Test creating a scheduled task with next run calculation
	task := &models.ScheduledTask{
		GameserverID: gameserver.ID,
		Name:         "Test Task",
		Type:         models.TaskTypeRestart,
		Status:       models.TaskStatusActive,
		CronSchedule: "0 3 * * *", // Daily at 3 AM (different from backup task)
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
	gameserver := &models.Gameserver{
		ID: "backup-test-gs", Name: "Backup Test Server", GameID: "minecraft", 
		PortMappings: []models.PortMapping{{Name: "game", Protocol: "tcp", ContainerPort: 25565, HostPort: 0}}, Status: models.StatusStopped, MaxBackups: 5,
		Environment: []string{"EULA=true"},
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
	mockDocker.SetShouldFail("create_backup", true)
	err = svc.CreateGameserverBackup(gameserver.ID)
	if err == nil {
		t.Error("Expected backup creation to fail")
	}

	// Test backup restoration failure
	mockDocker.SetShouldFail("restore_backup", true)
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
	gameserver := &models.Gameserver{
		ID: "auto-backup-test", Name: "Auto Backup Test", GameID: "minecraft", 
		PortMappings: []models.PortMapping{{Name: "game", Protocol: "tcp", ContainerPort: 25565, HostPort: 0}}, Status: models.StatusStopped,
		Environment: []string{"EULA=true"},
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
	if backupTask.Type != models.TaskTypeBackup {
		t.Errorf("Expected backup task type, got %s", backupTask.Type)
	}
	if backupTask.CronSchedule != "0 2 * * *" {
		t.Errorf("Expected daily 2 AM schedule, got %s", backupTask.CronSchedule)
	}
	if backupTask.Name != "Daily Backup" {
		t.Errorf("Expected 'Daily Backup' name, got %s", backupTask.Name)
	}
	if backupTask.Status != models.TaskStatusActive {
		t.Errorf("Expected active status, got %s", backupTask.Status)
	}
}

func TestGameserverService_CreateGameserverWithValidation(t *testing.T) {
	db, err := NewDatabaseManager(":memory:")
	if err != nil {
		t.Fatalf("Failed to create database: %v", err)
	}
	defer db.Close()

	mockDocker := NewMockDockerManager()
	svc := NewGameserverService(db, mockDocker)

	// Test creating gameserver without required environment variables
	gameserver := &models.Gameserver{
		ID: "fail-test", Name: "Fail Test", GameID: "minecraft", 
		PortMappings: []models.PortMapping{{Protocol: "tcp", ContainerPort: 25565, HostPort: 0}}, 
		Status: models.StatusStopped,
		// Missing required EULA environment variable
	}

	err = svc.CreateGameserver(gameserver)
	if err == nil {
		t.Error("Expected error when required config is missing")
	}

	// Verify gameserver was not saved to database
	_, err = db.GetGameserver(gameserver.ID)
	if err == nil {
		t.Error("Expected gameserver not to be in database after validation failure")
	}
}

func TestGameserverService_StartGameserver(t *testing.T) {
	db, err := NewDatabaseManager(":memory:")
	if err != nil {
		t.Fatalf("Failed to create database: %v", err)
	}
	defer db.Close()

	mockDocker := NewMockDockerManager()
	svc := NewGameserverService(db, mockDocker)

	// Create gameserver first
	gameserver := &models.Gameserver{
		ID: "start-test", Name: "Start Test", GameID: "minecraft", 
		PortMappings: []models.PortMapping{{Protocol: "tcp", ContainerPort: 25565, HostPort: 0}}, 
		Environment: []string{"EULA=true"},
		Status: models.StatusStopped,
	}
	svc.CreateGameserver(gameserver)

	// Test start
	err = svc.StartGameserver(gameserver.ID)
	if err != nil {
		t.Errorf("Failed to start gameserver: %v", err)
	}
}

func TestGameserverService_StopGameserver(t *testing.T) {
	db, err := NewDatabaseManager(":memory:")
	if err != nil {
		t.Fatalf("Failed to create database: %v", err)
	}
	defer db.Close()

	mockDocker := NewMockDockerManager()
	svc := NewGameserverService(db, mockDocker)

	// Create gameserver first
	gameserver := &models.Gameserver{
		ID: "stop-test", Name: "Stop Test", GameID: "minecraft", 
		PortMappings: []models.PortMapping{{Protocol: "tcp", ContainerPort: 25565, HostPort: 0}}, 
		Environment: []string{"EULA=true"},
		Status: models.StatusStopped,
	}
	svc.CreateGameserver(gameserver)

	// Test stop
	err = svc.StopGameserver(gameserver.ID)
	if err != nil {
		t.Errorf("Failed to stop gameserver: %v", err)
	}
}

func TestGameserverService_RestartGameserver(t *testing.T) {
	db, err := NewDatabaseManager(":memory:")
	if err != nil {
		t.Fatalf("Failed to create database: %v", err)
	}
	defer db.Close()

	mockDocker := NewMockDockerManager()
	svc := NewGameserverService(db, mockDocker)

	// Create gameserver first
	gameserver := &models.Gameserver{
		ID: "restart-test", Name: "Restart Test", GameID: "minecraft", 
		PortMappings: []models.PortMapping{{Protocol: "tcp", ContainerPort: 25565, HostPort: 0}}, 
		Environment: []string{"EULA=true"},
		Status: models.StatusStopped,
	}
	svc.CreateGameserver(gameserver)

	// Test restart
	err = svc.RestartGameserver(gameserver.ID)
	if err != nil {
		t.Errorf("Failed to restart gameserver: %v", err)
	}
}

func TestGameserverService_DeleteGameserver(t *testing.T) {
	db, err := NewDatabaseManager(":memory:")
	if err != nil {
		t.Fatalf("Failed to create database: %v", err)
	}
	defer db.Close()

	mockDocker := NewMockDockerManager()
	svc := NewGameserverService(db, mockDocker)

	// Create gameserver first
	gameserver := &models.Gameserver{
		ID: "delete-test", Name: "Delete Test", GameID: "minecraft", 
		PortMappings: []models.PortMapping{{Protocol: "tcp", ContainerPort: 25565, HostPort: 0}}, 
		Environment: []string{"EULA=true"},
		Status: models.StatusStopped,
	}
	svc.CreateGameserver(gameserver)

	// Verify it exists
	_, err = svc.GetGameserver(gameserver.ID)
	if err != nil {
		t.Fatalf("Gameserver should exist before deletion: %v", err)
	}

	// Test delete
	err = svc.DeleteGameserver(gameserver.ID)
	if err != nil {
		t.Errorf("Failed to delete gameserver: %v", err)
	}

	// Verify it's gone
	_, err = svc.GetGameserver(gameserver.ID)
	if err == nil {
		t.Error("Expected gameserver to be deleted")
	}
}

func TestGameserverService_UpdateGameserver(t *testing.T) {
	db, err := NewDatabaseManager(":memory:")
	if err != nil {
		t.Fatalf("Failed to create database: %v", err)
	}
	defer db.Close()

	mockDocker := NewMockDockerManager()
	svc := NewGameserverService(db, mockDocker)

	// Create gameserver first
	gameserver := &models.Gameserver{
		ID: "update-test", Name: "Update Test", GameID: "minecraft", 
		PortMappings: []models.PortMapping{{Protocol: "tcp", ContainerPort: 25565, HostPort: 0}}, 
		Environment: []string{"EULA=true"},
		Status: models.StatusStopped,
	}
	svc.CreateGameserver(gameserver)

	// Update it (only fields that are allowed to be updated)
	gameserver.Name = "Updated Name"
	gameserver.MemoryMB = 4096 // Test resource limit update
	err = svc.UpdateGameserver(gameserver)
	if err != nil {
		t.Errorf("Failed to update gameserver: %v", err)
	}

	// Verify update
	updated, err := svc.GetGameserver(gameserver.ID)
	if err != nil {
		t.Fatalf("Failed to get updated gameserver: %v", err)
	}
	if updated.Name != "Updated Name" {
		t.Errorf("Name not updated: expected 'Updated Name', got %s", updated.Name)
	}
	if updated.MemoryMB != 4096 {
		t.Errorf("MemoryMB not updated: expected 4096, got %d", updated.MemoryMB)
	}
	// Status should remain the same (preserved by service)
	if updated.Status != models.StatusStopped {
		t.Errorf("Status should be preserved: expected stopped, got %s", updated.Status)
	}
}

func TestGameserverService_NonExistentGameserver(t *testing.T) {
	db, err := NewDatabaseManager(":memory:")
	if err != nil {
		t.Fatalf("Failed to create database: %v", err)
	}
	defer db.Close()

	mockDocker := NewMockDockerManager()
	svc := NewGameserverService(db, mockDocker)

	// Test operations on non-existent gameserver
	err = svc.StartGameserver("non-existent")
	if err == nil {
		t.Error("Expected error for starting non-existent gameserver")
	}

	err = svc.StopGameserver("non-existent")
	if err == nil {
		t.Error("Expected error for stopping non-existent gameserver")
	}

	err = svc.RestartGameserver("non-existent")
	if err == nil {
		t.Error("Expected error for restarting non-existent gameserver")
	}

	err = svc.DeleteGameserver("non-existent")
	if err == nil {
		t.Error("Expected error for deleting non-existent gameserver")
	}

	err = svc.CreateGameserverBackup("non-existent")
	if err == nil {
		t.Error("Expected error for backing up non-existent gameserver")
	}

	err = svc.RestoreGameserverBackup("non-existent", "backup.tar.gz")
	if err == nil {
		t.Error("Expected error for restoring non-existent gameserver")
	}
}

func TestGameserverService_ListGameservers(t *testing.T) {
	db, err := NewDatabaseManager(":memory:")
	if err != nil {
		t.Fatalf("Failed to create database: %v", err)
	}
	defer db.Close()

	mockDocker := NewMockDockerManager()
	svc := NewGameserverService(db, mockDocker)

	// List empty
	servers, err := svc.ListGameservers()
	if err != nil {
		t.Fatalf("Failed to list gameservers: %v", err)
	}
	if len(servers) != 0 {
		t.Errorf("Expected 0 gameservers, got %d", len(servers))
	}

	// Create some gameservers
	gameservers := []*models.Gameserver{
		{ID: "gs-1", Name: "Server 1", GameID: "minecraft", PortMappings: []models.PortMapping{{Protocol: "tcp", ContainerPort: 25565, HostPort: 0}}, Environment: []string{"EULA=true"}, Status: models.StatusStopped},
		{ID: "gs-2", Name: "Server 2", GameID: "valheim", PortMappings: []models.PortMapping{{Protocol: "udp", ContainerPort: 2456, HostPort: 0}}, Environment: []string{"SERVER_NAME=Test Server", "WORLD_NAME=TestWorld"}, Status: models.StatusStopped},
	}

	for _, gs := range gameservers {
		if err := svc.CreateGameserver(gs); err != nil {
			t.Fatalf("Failed to create gameserver %s: %v", gs.ID, err)
		}
	}

	// List again
	servers, err = svc.ListGameservers()
	if err != nil {
		t.Fatalf("Failed to list gameservers: %v", err)
	}
	if len(servers) != 2 {
		t.Errorf("Expected 2 gameservers, got %d", len(servers))
	}
}
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
		Status:       models.StatusStopped,
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
		Environment:  []string{"EULA=true"},
		Status:       models.StatusStopped,
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
		Environment:  []string{"EULA=true"},
		Status:       models.StatusStopped,
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
		Environment:  []string{"EULA=true"},
		Status:       models.StatusStopped,
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
		Environment:  []string{"EULA=true"},
		Status:       models.StatusStopped,
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
		Environment:  []string{"EULA=true"},
		Status:       models.StatusStopped,
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

func TestGameserverService_GameSpecificPortAllocation(t *testing.T) {
	db, err := NewDatabaseManager(":memory:")
	if err != nil {
		t.Fatalf("Failed to create database: %v", err)
	}
	defer db.Close()

	mockDocker := NewMockDockerManager()
	svc := NewGameserverService(db, mockDocker)

	// Test 1: Create Minecraft server - should get port 25565 (game-specific default)
	minecraftServer := &models.Gameserver{
		ID: "mc-test", Name: "Minecraft Test", GameID: "minecraft",
		PortMappings: []models.PortMapping{{Name: "game", Protocol: "tcp", ContainerPort: 25565, HostPort: 0}},
		Environment:  []string{"EULA=true"},
		Status:       models.StatusStopped,
	}

	err = svc.CreateGameserver(minecraftServer)
	if err != nil {
		t.Fatalf("Failed to create Minecraft server: %v", err)
	}

	// Verify Minecraft server got port 25565
	if minecraftServer.PortMappings[0].HostPort != 25565 {
		t.Errorf("Expected Minecraft server to get port 25565, got %d", minecraftServer.PortMappings[0].HostPort)
	}

	// Test 2: Create CS2 server - should get port 27015 (game-specific default)
	cs2Server := &models.Gameserver{
		ID: "cs2-test", Name: "CS2 Test", GameID: "cs2",
		PortMappings: []models.PortMapping{
			{Name: "game", Protocol: "tcp", ContainerPort: 27015, HostPort: 0},
			{Name: "game", Protocol: "udp", ContainerPort: 27015, HostPort: 0},
		},
		Environment: []string{"HOSTNAME=CS2 Server", "RCON_PASSWORD=test123"},
		Status:      models.StatusStopped,
	}

	err = svc.CreateGameserver(cs2Server)
	if err != nil {
		t.Fatalf("Failed to create CS2 server: %v", err)
	}

	// Verify CS2 server got port 27015 for both TCP and UDP
	if cs2Server.PortMappings[0].HostPort != 27015 {
		t.Errorf("Expected CS2 server TCP to get port 27015, got %d", cs2Server.PortMappings[0].HostPort)
	}
	if cs2Server.PortMappings[1].HostPort != 27015 {
		t.Errorf("Expected CS2 server UDP to get port 27015, got %d", cs2Server.PortMappings[1].HostPort)
	}

	// Test 3: Create second Minecraft server - should get fallback port (not 25565)
	minecraftServer2 := &models.Gameserver{
		ID: "mc-test-2", Name: "Minecraft Test 2", GameID: "minecraft",
		PortMappings: []models.PortMapping{{Name: "game", Protocol: "tcp", ContainerPort: 25565, HostPort: 0}},
		Environment:  []string{"EULA=true"},
		Status:       models.StatusStopped,
	}

	err = svc.CreateGameserver(minecraftServer2)
	if err != nil {
		t.Fatalf("Failed to create second Minecraft server: %v", err)
	}

	// Verify second Minecraft server got a different port (fallback from allowed ports)
	if minecraftServer2.PortMappings[0].HostPort == 25565 {
		t.Errorf("Expected second Minecraft server to get fallback port, not 25565")
	}
	// With the new logic, it should get the next available port from allowed ports (7777, 2456, etc.)
	allowedPorts := []int{7777, 2456, 2457, 8211} // remaining allowed ports after 25565 and 27015 are taken
	found := false
	for _, port := range allowedPorts {
		if minecraftServer2.PortMappings[0].HostPort == port {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("Expected second Minecraft server to get port from allowed ports, got %d", minecraftServer2.PortMappings[0].HostPort)
	}

	// Test 4: Create Terraria server - should get next available port from allowed list
	terrariaServer := &models.Gameserver{
		ID: "terraria-test", Name: "Terraria Test", GameID: "terraria",
		PortMappings: []models.PortMapping{{Name: "game", Protocol: "tcp", ContainerPort: 7777, HostPort: 0}},
		Status:       models.StatusStopped,
	}

	err = svc.CreateGameserver(terrariaServer)
	if err != nil {
		t.Fatalf("Failed to create Terraria server: %v", err)
	}

	// Verify Terraria server got an allowed port (7777 might be taken by second MC server)
	terrariaPort := terrariaServer.PortMappings[0].HostPort
	if terrariaPort != 7777 && terrariaPort != 2456 && terrariaPort != 2457 && terrariaPort != 8211 {
		t.Errorf("Expected Terraria server to get an allowed port, got %d", terrariaPort)
	}
}

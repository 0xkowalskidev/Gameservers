package database

import (
	"testing"
	"time"

	"0xkowalskidev/gameservers/models"
)

func TestDatabaseManager_CRUD(t *testing.T) {
	db, err := NewDatabaseManager(":memory:")
	if err != nil {
		t.Fatalf("Failed to create database: %v", err)
	}
	defer db.Close()

	server := &models.Gameserver{
		ID: "test-1", Name: "Test Server", GameID: "minecraft",
		PortMappings: []models.PortMapping{{Name: "game", Protocol: "tcp", ContainerPort: 25565, HostPort: 0}}, 
		Environment: []string{"ENV=prod"}, Volumes: []string{"/data:/mc"},
		Status: models.StatusStopped, CreatedAt: time.Now(), UpdatedAt: time.Now(),
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
	retrieved.Status = models.StatusRunning
	retrieved.ContainerID = "container-123"
	if err := db.UpdateGameserver(retrieved); err != nil {
		t.Fatalf("Update failed: %v", err)
	}

	updated, _ := db.GetGameserver(server.ID)
	if updated.Status != models.StatusRunning || updated.ContainerID != "container-123" {
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

	server1 := &models.Gameserver{ID: "1", Name: "dup", GameID: "minecraft", PortMappings: []models.PortMapping{{Protocol: "tcp", ContainerPort: 25565, HostPort: 0}}, Status: models.StatusStopped, CreatedAt: time.Now(), UpdatedAt: time.Now()}
	server2 := &models.Gameserver{ID: "2", Name: "dup", GameID: "minecraft", PortMappings: []models.PortMapping{{Protocol: "tcp", ContainerPort: 25566, HostPort: 0}}, Status: models.StatusStopped, CreatedAt: time.Now(), UpdatedAt: time.Now()}

	db.CreateGameserver(server1)
	if err := db.CreateGameserver(server2); err == nil {
		t.Error("Expected duplicate name error")
	}
}

func TestDatabaseManager_GetNonExistentGameserver(t *testing.T) {
	db, _ := NewDatabaseManager(":memory:")
	defer db.Close()

	_, err := db.GetGameserver("non-existent")
	if err == nil {
		t.Error("Expected error for non-existent gameserver")
	}
}

func TestDatabaseManager_UpdateNonExistentGameserver(t *testing.T) {
	db, _ := NewDatabaseManager(":memory:")
	defer db.Close()

	server := &models.Gameserver{
		ID: "non-existent", Name: "Test", GameID: "minecraft",
		Status: models.StatusStopped, CreatedAt: time.Now(), UpdatedAt: time.Now(),
	}

	err := db.UpdateGameserver(server)
	if err == nil {
		t.Error("Expected error for updating non-existent gameserver")
	}
}

func TestDatabaseManager_DeleteNonExistentGameserver(t *testing.T) {
	db, _ := NewDatabaseManager(":memory:")
	defer db.Close()

	err := db.DeleteGameserver("non-existent")
	if err == nil {
		t.Error("Expected error for deleting non-existent gameserver")
	}
}

func TestDatabaseManager_ListEmptyGameservers(t *testing.T) {
	db, _ := NewDatabaseManager(":memory:")
	defer db.Close()

	servers, err := db.ListGameservers()
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}
	if len(servers) != 0 {
		t.Errorf("Expected 0 servers, got %d", len(servers))
	}
}

func TestDatabaseManager_MultipleGameservers(t *testing.T) {
	db, _ := NewDatabaseManager(":memory:")
	defer db.Close()

	servers := []*models.Gameserver{
		{ID: "1", Name: "Server 1", GameID: "minecraft", PortMappings: []models.PortMapping{{Protocol: "tcp", ContainerPort: 25565, HostPort: 0}}, Status: models.StatusStopped, CreatedAt: time.Now(), UpdatedAt: time.Now()},
		{ID: "2", Name: "Server 2", GameID: "valheim", PortMappings: []models.PortMapping{{Protocol: "udp", ContainerPort: 2456, HostPort: 0}}, Status: models.StatusStopped, CreatedAt: time.Now(), UpdatedAt: time.Now()},
		{ID: "3", Name: "Server 3", GameID: "cs2", PortMappings: []models.PortMapping{{Protocol: "tcp", ContainerPort: 27015, HostPort: 0}}, Status: models.StatusStopped, CreatedAt: time.Now(), UpdatedAt: time.Now()},
	}

	// Create all servers
	for _, server := range servers {
		if err := db.CreateGameserver(server); err != nil {
			t.Fatalf("Failed to create server %s: %v", server.ID, err)
		}
	}

	// List and verify count
	retrievedServers, err := db.ListGameservers()
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}
	if len(retrievedServers) != 3 {
		t.Errorf("Expected 3 servers, got %d", len(retrievedServers))
	}

	// Verify each server can be retrieved
	for _, server := range servers {
		retrieved, err := db.GetGameserver(server.ID)
		if err != nil {
			t.Errorf("Failed to get server %s: %v", server.ID, err)
		}
		if retrieved.Name != server.Name {
			t.Errorf("Name mismatch for server %s", server.ID)
		}
	}
}

func TestDatabaseManager_GameserverWithComplexData(t *testing.T) {
	db, _ := NewDatabaseManager(":memory:")
	defer db.Close()

	server := &models.Gameserver{
		ID:       "complex-test",
		Name:     "Complex Server",
		GameID:   "minecraft",
		PortMappings: []models.PortMapping{
			{Name: "game", Protocol: "tcp", ContainerPort: 25565, HostPort: 25565},
			{Name: "rcon", Protocol: "tcp", ContainerPort: 25575, HostPort: 25575},
		},
		Environment: []string{
			"EULA=true",
			"MAX_PLAYERS=20",
			"DIFFICULTY=normal",
		},
		Volumes: []string{
			"/data/minecraft:/data",
			"/backups:/backups",
		},
		MemoryMB:   2048,
		CPUCores:   2,
		MaxBackups: 10,
		Status:     models.StatusStopped,
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
	}

	// Create
	if err := db.CreateGameserver(server); err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	// Retrieve and verify all fields
	retrieved, err := db.GetGameserver(server.ID)
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}

	if retrieved.Name != server.Name {
		t.Errorf("Name mismatch: expected %s, got %s", server.Name, retrieved.Name)
	}
	if retrieved.GameID != server.GameID {
		t.Errorf("GameID mismatch: expected %s, got %s", server.GameID, retrieved.GameID)
	}
	// Image field is not set by DatabaseManager directly (it's populated by GameserverService)
	// So we don't test it here
	if len(retrieved.PortMappings) != 2 {
		t.Errorf("PortMappings count mismatch: expected 2, got %d", len(retrieved.PortMappings))
	}
	if len(retrieved.Environment) != 3 {
		t.Errorf("Environment count mismatch: expected 3, got %d", len(retrieved.Environment))
	}
	if len(retrieved.Volumes) != 2 {
		t.Errorf("Volumes count mismatch: expected 2, got %d", len(retrieved.Volumes))
	}
	if retrieved.MemoryMB != server.MemoryMB {
		t.Errorf("MemoryMB mismatch: expected %d, got %d", server.MemoryMB, retrieved.MemoryMB)
	}
	if retrieved.CPUCores != server.CPUCores {
		t.Errorf("CPUCores mismatch: expected %f, got %f", server.CPUCores, retrieved.CPUCores)
	}
	if retrieved.MaxBackups != server.MaxBackups {
		t.Errorf("MaxBackups mismatch: expected %d, got %d", server.MaxBackups, retrieved.MaxBackups)
	}
}
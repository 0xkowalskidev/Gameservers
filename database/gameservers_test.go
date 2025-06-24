package database

import (
	"testing"
	"time"

	"0xkowalskidev/gameservers/models"
)

func TestDatabaseManager_GameserverPortMappings(t *testing.T) {
	db, err := NewDatabaseManager(":memory:")
	if err != nil {
		t.Fatalf("Failed to create database: %v", err)
	}
	defer db.Close()

	server := &models.Gameserver{
		ID:     "port-test",
		Name:   "Port Test Server",
		GameID: "minecraft",
		PortMappings: []models.PortMapping{
			{Name: "game", Protocol: "tcp", ContainerPort: 25565, HostPort: 25565},
			{Name: "rcon", Protocol: "tcp", ContainerPort: 25575, HostPort: 25575},
			{Name: "query", Protocol: "udp", ContainerPort: 25565, HostPort: 25566},
		},
		Status:    models.StatusStopped,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	// Create
	if err := db.CreateGameserver(server); err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	// Retrieve and verify port mappings
	retrieved, err := db.GetGameserver(server.ID)
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}

	if len(retrieved.PortMappings) != 3 {
		t.Errorf("Expected 3 port mappings, got %d", len(retrieved.PortMappings))
	}

	// Verify each port mapping
	expectedMappings := map[string]models.PortMapping{
		"game":  {Name: "game", Protocol: "tcp", ContainerPort: 25565, HostPort: 25565},
		"rcon":  {Name: "rcon", Protocol: "tcp", ContainerPort: 25575, HostPort: 25575},
		"query": {Name: "query", Protocol: "udp", ContainerPort: 25565, HostPort: 25566},
	}

	for _, mapping := range retrieved.PortMappings {
		expected, exists := expectedMappings[mapping.Name]
		if !exists {
			t.Errorf("Unexpected port mapping: %s", mapping.Name)
			continue
		}

		if mapping.Protocol != expected.Protocol {
			t.Errorf("Protocol mismatch for %s: expected %s, got %s",
				mapping.Name, expected.Protocol, mapping.Protocol)
		}
		if mapping.ContainerPort != expected.ContainerPort {
			t.Errorf("ContainerPort mismatch for %s: expected %d, got %d",
				mapping.Name, expected.ContainerPort, mapping.ContainerPort)
		}
		if mapping.HostPort != expected.HostPort {
			t.Errorf("HostPort mismatch for %s: expected %d, got %d",
				mapping.Name, expected.HostPort, mapping.HostPort)
		}
	}
}

func TestDatabaseManager_GameserverEnvironmentVariables(t *testing.T) {
	db, err := NewDatabaseManager(":memory:")
	if err != nil {
		t.Fatalf("Failed to create database: %v", err)
	}
	defer db.Close()

	server := &models.Gameserver{
		ID:     "env-test",
		Name:   "Environment Test Server",
		GameID: "minecraft",
		PortMappings: []models.PortMapping{
			{Name: "game", Protocol: "tcp", ContainerPort: 25565, HostPort: 0},
		},
		Environment: []string{
			"EULA=true",
			"MAX_PLAYERS=20",
			"DIFFICULTY=normal",
			"WHITELIST_ENABLED=false",
			"ONLINE_MODE=true",
		},
		Status:    models.StatusStopped,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	// Create
	if err := db.CreateGameserver(server); err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	// Retrieve and verify environment variables
	retrieved, err := db.GetGameserver(server.ID)
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}

	if len(retrieved.Environment) != 5 {
		t.Errorf("Expected 5 environment variables, got %d", len(retrieved.Environment))
	}

	// Convert to map for easier verification
	envMap := make(map[string]bool)
	for _, env := range retrieved.Environment {
		envMap[env] = true
	}

	expectedEnvs := []string{
		"EULA=true",
		"MAX_PLAYERS=20",
		"DIFFICULTY=normal",
		"WHITELIST_ENABLED=false",
		"ONLINE_MODE=true",
	}

	for _, expected := range expectedEnvs {
		if !envMap[expected] {
			t.Errorf("Missing environment variable: %s", expected)
		}
	}
}

func TestDatabaseManager_GameserverVolumes(t *testing.T) {
	db, err := NewDatabaseManager(":memory:")
	if err != nil {
		t.Fatalf("Failed to create database: %v", err)
	}
	defer db.Close()

	server := &models.Gameserver{
		ID:     "volume-test",
		Name:   "Volume Test Server",
		GameID: "minecraft",
		PortMappings: []models.PortMapping{
			{Name: "game", Protocol: "tcp", ContainerPort: 25565, HostPort: 0},
		},
		Volumes: []string{
			"/data/minecraft:/data",
			"/backups:/backups:ro",
			"/logs:/logs",
		},
		Status:    models.StatusStopped,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	// Create
	if err := db.CreateGameserver(server); err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	// Retrieve and verify volumes
	retrieved, err := db.GetGameserver(server.ID)
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}

	if len(retrieved.Volumes) != 3 {
		t.Errorf("Expected 3 volumes, got %d", len(retrieved.Volumes))
	}

	expectedVolumes := []string{
		"/data/minecraft:/data",
		"/backups:/backups:ro",
		"/logs:/logs",
	}

	volumeMap := make(map[string]bool)
	for _, volume := range retrieved.Volumes {
		volumeMap[volume] = true
	}

	for _, expected := range expectedVolumes {
		if !volumeMap[expected] {
			t.Errorf("Missing volume: %s", expected)
		}
	}
}

func TestDatabaseManager_GameserverResourceLimits(t *testing.T) {
	db, err := NewDatabaseManager(":memory:")
	if err != nil {
		t.Fatalf("Failed to create database: %v", err)
	}
	defer db.Close()

	server := &models.Gameserver{
		ID:     "resource-test",
		Name:   "Resource Test Server",
		GameID: "minecraft",
		PortMappings: []models.PortMapping{
			{Name: "game", Protocol: "tcp", ContainerPort: 25565, HostPort: 0},
		},
		MemoryMB:   4096,
		CPUCores:   4,
		MaxBackups: 15,
		Status:     models.StatusStopped,
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
	}

	// Create
	if err := db.CreateGameserver(server); err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	// Retrieve and verify resource limits
	retrieved, err := db.GetGameserver(server.ID)
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}

	if retrieved.MemoryMB != 4096 {
		t.Errorf("MemoryMB mismatch: expected 4096, got %d", retrieved.MemoryMB)
	}
	if retrieved.CPUCores != 4 {
		t.Errorf("CPUCores mismatch: expected 4, got %f", retrieved.CPUCores)
	}
	if retrieved.MaxBackups != 15 {
		t.Errorf("MaxBackups mismatch: expected 15, got %d", retrieved.MaxBackups)
	}
}

func TestDatabaseManager_GameserverStatusUpdates(t *testing.T) {
	db, err := NewDatabaseManager(":memory:")
	if err != nil {
		t.Fatalf("Failed to create database: %v", err)
	}
	defer db.Close()

	server := &models.Gameserver{
		ID:     "status-test",
		Name:   "Status Test Server",
		GameID: "minecraft",
		PortMappings: []models.PortMapping{
			{Name: "game", Protocol: "tcp", ContainerPort: 25565, HostPort: 0},
		},
		Status:    models.StatusStopped,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	// Create
	if err := db.CreateGameserver(server); err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	// Test status progression
	statuses := []models.GameserverStatus{
		models.StatusStarting,
		models.StatusRunning,
		models.StatusStopping,
		models.StatusStopped,
		models.StatusError,
	}

	for _, status := range statuses {
		server.Status = status
		server.UpdatedAt = time.Now()

		if err := db.UpdateGameserver(server); err != nil {
			t.Fatalf("Update failed for status %s: %v", status, err)
		}

		// Retrieve and verify
		retrieved, err := db.GetGameserver(server.ID)
		if err != nil {
			t.Fatalf("Get failed: %v", err)
		}

		if retrieved.Status != status {
			t.Errorf("Status mismatch: expected %s, got %s", status, retrieved.Status)
		}
	}
}

func TestDatabaseManager_GameserverContainerIDUpdate(t *testing.T) {
	db, err := NewDatabaseManager(":memory:")
	if err != nil {
		t.Fatalf("Failed to create database: %v", err)
	}
	defer db.Close()

	server := &models.Gameserver{
		ID:     "container-test",
		Name:   "Container Test Server",
		GameID: "minecraft",
		PortMappings: []models.PortMapping{
			{Name: "game", Protocol: "tcp", ContainerPort: 25565, HostPort: 0},
		},
		Status:    models.StatusStopped,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	// Create without container ID
	if err := db.CreateGameserver(server); err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	// Verify no container ID initially
	retrieved, err := db.GetGameserver(server.ID)
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if retrieved.ContainerID != "" {
		t.Errorf("Expected empty container ID, got %s", retrieved.ContainerID)
	}

	// Update with container ID
	containerID := "abc123def456"
	server.ContainerID = containerID
	server.UpdatedAt = time.Now()

	if err := db.UpdateGameserver(server); err != nil {
		t.Fatalf("Update failed: %v", err)
	}

	// Verify container ID was set
	retrieved, err = db.GetGameserver(server.ID)
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if retrieved.ContainerID != containerID {
		t.Errorf("ContainerID mismatch: expected %s, got %s", containerID, retrieved.ContainerID)
	}

	// Clear container ID
	server.ContainerID = ""
	server.UpdatedAt = time.Now()

	if err := db.UpdateGameserver(server); err != nil {
		t.Fatalf("Update failed: %v", err)
	}

	// Verify container ID was cleared
	retrieved, err = db.GetGameserver(server.ID)
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if retrieved.ContainerID != "" {
		t.Errorf("Expected empty container ID after clear, got %s", retrieved.ContainerID)
	}
}

func TestDatabaseManager_GameserverTimestamps(t *testing.T) {
	db, err := NewDatabaseManager(":memory:")
	if err != nil {
		t.Fatalf("Failed to create database: %v", err)
	}
	defer db.Close()

	now := time.Now()
	server := &models.Gameserver{
		ID:     "timestamp-test",
		Name:   "Timestamp Test Server",
		GameID: "minecraft",
		PortMappings: []models.PortMapping{
			{Name: "game", Protocol: "tcp", ContainerPort: 25565, HostPort: 0},
		},
		Status:    models.StatusStopped,
		CreatedAt: now,
		UpdatedAt: now,
	}

	// Create
	if err := db.CreateGameserver(server); err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	// Retrieve and verify timestamps
	retrieved, err := db.GetGameserver(server.ID)
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}

	// Check that timestamps are close (within 1 second)
	if abs(retrieved.CreatedAt.Unix()-now.Unix()) > 1 {
		t.Errorf("CreatedAt timestamp mismatch: expected ~%v, got %v", now, retrieved.CreatedAt)
	}
	if abs(retrieved.UpdatedAt.Unix()-now.Unix()) > 1 {
		t.Errorf("UpdatedAt timestamp mismatch: expected ~%v, got %v", now, retrieved.UpdatedAt)
	}

	// Update and verify UpdatedAt changes
	time.Sleep(time.Millisecond * 10) // Small delay to ensure different timestamp
	laterTime := time.Now()
	server.Name = "Updated Name"
	server.UpdatedAt = laterTime

	if err := db.UpdateGameserver(server); err != nil {
		t.Fatalf("Update failed: %v", err)
	}

	updated, err := db.GetGameserver(server.ID)
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}

	// CreatedAt should remain the same
	if abs(updated.CreatedAt.Unix()-now.Unix()) > 1 {
		t.Errorf("CreatedAt should not change on update")
	}

	// UpdatedAt should be updated
	if abs(updated.UpdatedAt.Unix()-laterTime.Unix()) > 1 {
		t.Errorf("UpdatedAt timestamp mismatch after update: expected ~%v, got %v", laterTime, updated.UpdatedAt)
	}
}

// Helper function for timestamp comparison
func abs(x int64) int64 {
	if x < 0 {
		return -x
	}
	return x
}

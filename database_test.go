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
		ID: "test-1", Name: "Test Server", GameType: "minecraft", Image: "minecraft:latest",
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

	server1 := &Gameserver{ID: "1", Name: "dup", GameType: "mc", Image: "mc:latest", Port: 25565, Status: StatusStopped, CreatedAt: time.Now(), UpdatedAt: time.Now()}
	server2 := &Gameserver{ID: "2", Name: "dup", GameType: "mc", Image: "mc:latest", Port: 25566, Status: StatusStopped, CreatedAt: time.Now(), UpdatedAt: time.Now()}

	db.CreateGameserver(server1)
	if err := db.CreateGameserver(server2); err == nil {
		t.Error("Expected duplicate name error")
	}
}
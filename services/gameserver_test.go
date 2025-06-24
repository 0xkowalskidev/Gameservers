package services

import (
	"context"
	"testing"

	"0xkowalskidev/gameservers/models"
)

// mockDB implements models.GameserverServiceInterface for testing
type mockDB struct {
	models.GameserverServiceInterface // Embed interface with nil methods
	gameservers map[string]*models.Gameserver
	games       map[string]*models.Game
}

func newMockDB() *mockDB {
	return &mockDB{
		gameservers: make(map[string]*models.Gameserver),
		games: map[string]*models.Game{
			"minecraft": {
				ID:           "minecraft",
				Name:         "minecraft",
				Image:        "minecraft:latest",
				PortMappings: []models.PortMapping{{Protocol: "tcp", ContainerPort: 25565, HostPort: 0}},
			},
		},
	}
}

func (db *mockDB) GetGame(id string) (*models.Game, error) {
	if game, ok := db.games[id]; ok {
		return game, nil
	}
	return nil, NotFound("game")
}

func (db *mockDB) CreateGameserver(gs *models.Gameserver) error {
	if gs.ID == "" {
		gs.ID = "test-id-1"
	}
	db.gameservers[gs.ID] = gs
	return nil
}

func (db *mockDB) GetGameserver(id string) (*models.Gameserver, error) {
	if gs, ok := db.gameservers[id]; ok {
		return gs, nil
	}
	return nil, NotFound("gameserver")
}

func (db *mockDB) UpdateGameserver(gs *models.Gameserver) error {
	if _, ok := db.gameservers[gs.ID]; !ok {
		return NotFound("gameserver")
	}
	db.gameservers[gs.ID] = gs
	return nil
}

func (db *mockDB) DeleteGameserver(id string) error {
	if _, ok := db.gameservers[id]; !ok {
		return NotFound("gameserver")
	}
	delete(db.gameservers, id)
	return nil
}

// mockDocker implements models.DockerManagerInterface for testing
type mockDocker struct {
	models.DockerManagerInterface // Embed interface with nil methods
	containers map[string]*models.Gameserver
}

func newMockDocker() *mockDocker {
	return &mockDocker{
		containers: make(map[string]*models.Gameserver),
	}
}

func (d *mockDocker) CreateContainer(server *models.Gameserver) error {
	server.ContainerID = "container-" + server.ID
	d.containers[server.ContainerID] = server
	return nil
}

func (d *mockDocker) StartContainer(containerID string) error { return nil }
func (d *mockDocker) StopContainer(containerID string) error { return nil }
func (d *mockDocker) RemoveContainer(containerID string) error { return nil }
func (d *mockDocker) CreateBackup(gameserverID, backupPath string) error { return nil }

func TestServiceCreateGameserver(t *testing.T) {
	tests := []struct {
		name        string
		req         CreateGameserverRequest
		expectError bool
	}{
		{
			name: "valid_request",
			req: CreateGameserverRequest{
				Name:     "test-server",
				GameID:   "minecraft",
				MemoryMB: 2048,
				CPUCores: 1.0,
			},
			expectError: false,
		},
		{
			name: "missing_name",
			req: CreateGameserverRequest{
				GameID:   "minecraft",
				MemoryMB: 2048,
			},
			expectError: true,
		},
		{
			name: "invalid_game",
			req: CreateGameserverRequest{
				Name:   "test-server",
				GameID: "nonexistent",
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			service := NewGameserverService(newMockDB(), newMockDocker(), "/tmp")

			ctx := context.Background()
			gameserver, err := service.CreateGameserver(ctx, tt.req)

			if tt.expectError {
				if err == nil {
					t.Errorf("Expected error but got none")
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
				}
				if gameserver == nil {
					t.Errorf("Expected gameserver but got nil")
				}
				if gameserver != nil && gameserver.Name != tt.req.Name {
					t.Errorf("Expected name %s, got %s", tt.req.Name, gameserver.Name)
				}
			}
		})
	}
}

func TestServiceLifecycle(t *testing.T) {
	service := NewGameserverService(newMockDB(), newMockDocker(), "/tmp")
	ctx := context.Background()

	// Create
	req := CreateGameserverRequest{
		Name:     "test-server",
		GameID:   "minecraft",
		MemoryMB: 1024,
	}

	gameserver, err := service.CreateGameserver(ctx, req)
	if err != nil {
		t.Fatalf("Failed to create gameserver: %v", err)
	}

	// Get
	retrieved, err := service.GetGameserver(ctx, gameserver.ID)
	if err != nil {
		t.Fatalf("Failed to get gameserver: %v", err)
	}
	if retrieved.Name != req.Name {
		t.Errorf("Expected name %s, got %s", req.Name, retrieved.Name)
	}

	// Start
	err = service.StartGameserver(ctx, gameserver.ID)
	if err != nil {
		t.Fatalf("Failed to start gameserver: %v", err)
	}

	// Stop
	err = service.StopGameserver(ctx, gameserver.ID)
	if err != nil {
		t.Fatalf("Failed to stop gameserver: %v", err)
	}

	// Delete
	err = service.DeleteGameserver(ctx, gameserver.ID)
	if err != nil {
		t.Fatalf("Failed to delete gameserver: %v", err)
	}

	// Verify deletion
	_, err = service.GetGameserver(ctx, gameserver.ID)
	if err == nil {
		t.Errorf("Expected gameserver to be deleted but still exists")
	}
}
package services

import (
	"context"
	"io"
	"testing"

	"0xkowalskidev/gameservers/models"
)

// Mock implementations for testing the service layer
type testGameserverDB struct {
	gameservers map[string]*models.Gameserver
	games       map[string]*models.Game
	nextID      int
}

func newTestGameserverDB() *testGameserverDB {
	return &testGameserverDB{
		gameservers: make(map[string]*models.Gameserver),
		games: map[string]*models.Game{
			"minecraft": {
				ID:           "minecraft",
				Name:         "minecraft",
				Image:        "minecraft:latest",
				PortMappings: []models.PortMapping{{Protocol: "tcp", ContainerPort: 25565, HostPort: 0}},
			},
		},
		nextID: 1,
	}
}

func (db *testGameserverDB) GetGame(id string) (*models.Game, error) {
	if game, ok := db.games[id]; ok {
		return game, nil
	}
	return nil, NotFound("game")
}

func (db *testGameserverDB) CreateGameserver(gs *models.Gameserver) error {
	if gs.ID == "" {
		gs.ID = "test-id-1"
	}
	db.gameservers[gs.ID] = gs
	return nil
}

func (db *testGameserverDB) GetGameserver(id string) (*models.Gameserver, error) {
	if gs, ok := db.gameservers[id]; ok {
		return gs, nil
	}
	return nil, NotFound("gameserver")
}

func (db *testGameserverDB) UpdateGameserver(gs *models.Gameserver) error {
	if _, ok := db.gameservers[gs.ID]; !ok {
		return NotFound("gameserver")
	}
	db.gameservers[gs.ID] = gs
	return nil
}

func (db *testGameserverDB) ListGameservers() ([]*models.Gameserver, error) {
	var servers []*models.Gameserver
	for _, gs := range db.gameservers {
		servers = append(servers, gs)
	}
	return servers, nil
}

func (db *testGameserverDB) DeleteGameserver(id string) error {
	if _, ok := db.gameservers[id]; !ok {
		return NotFound("gameserver")
	}
	delete(db.gameservers, id)
	return nil
}

// Implement remaining interface methods (not used in these tests)
func (db *testGameserverDB) StartGameserver(id string) error             { return nil }
func (db *testGameserverDB) StopGameserver(id string) error              { return nil }
func (db *testGameserverDB) RestartGameserver(id string) error           { return nil }
func (db *testGameserverDB) SendGameserverCommand(id string, command string) error { return nil }
func (db *testGameserverDB) StreamGameserverLogs(id string) (io.ReadCloser, error) {
	return nil, nil
}
func (db *testGameserverDB) StreamGameserverStats(id string) (io.ReadCloser, error) {
	return nil, nil
}
func (db *testGameserverDB) ListGames() ([]*models.Game, error) { return nil, nil }
func (db *testGameserverDB) CreateGame(game *models.Game) error { return nil }
func (db *testGameserverDB) CreateScheduledTask(task *models.ScheduledTask) error { return nil }
func (db *testGameserverDB) GetScheduledTask(id string) (*models.ScheduledTask, error) {
	return nil, nil
}
func (db *testGameserverDB) UpdateScheduledTask(task *models.ScheduledTask) error { return nil }
func (db *testGameserverDB) DeleteScheduledTask(id string) error                  { return nil }
func (db *testGameserverDB) ListScheduledTasksForGameserver(gameserverID string) ([]*models.ScheduledTask, error) {
	return nil, nil
}
func (db *testGameserverDB) CreateGameserverBackup(gameserverID string) error { return nil }
func (db *testGameserverDB) RestoreGameserverBackup(gameserverID, backupFilename string) error {
	return nil
}
func (db *testGameserverDB) ListGameserverBackups(gameserverID string) ([]*models.FileInfo, error) {
	return nil, nil
}
func (db *testGameserverDB) ListFiles(containerID string, path string) ([]*models.FileInfo, error) {
	return nil, nil
}
func (db *testGameserverDB) ReadFile(containerID string, path string) ([]byte, error) {
	return nil, nil
}
func (db *testGameserverDB) WriteFile(containerID string, path string, content []byte) error {
	return nil
}
func (db *testGameserverDB) CreateDirectory(containerID string, path string) error { return nil }
func (db *testGameserverDB) DeletePath(containerID string, path string) error     { return nil }
func (db *testGameserverDB) DownloadFile(containerID string, path string) (io.ReadCloser, error) {
	return nil, nil
}
func (db *testGameserverDB) RenameFile(containerID string, oldPath string, newPath string) error {
	return nil
}
func (db *testGameserverDB) UploadFile(containerID string, destPath string, reader io.Reader) error {
	return nil
}

type testDockerManager struct {
	containers map[string]*models.Gameserver
}

func newTestDockerManager() *testDockerManager {
	return &testDockerManager{
		containers: make(map[string]*models.Gameserver),
	}
}

func (d *testDockerManager) CreateContainer(server *models.Gameserver) error {
	server.ContainerID = "container-" + server.ID
	d.containers[server.ContainerID] = server
	return nil
}

func (d *testDockerManager) StartContainer(containerID string) error  { return nil }
func (d *testDockerManager) StopContainer(containerID string) error   { return nil }
func (d *testDockerManager) RemoveContainer(containerID string) error { return nil }
func (d *testDockerManager) SendCommand(containerID string, command string) error {
	return nil
}
func (d *testDockerManager) GetContainerStatus(containerID string) (models.GameserverStatus, error) {
	return models.StatusStopped, nil
}
func (d *testDockerManager) StreamContainerLogs(containerID string) (io.ReadCloser, error) {
	return nil, nil
}
func (d *testDockerManager) StreamContainerStats(containerID string) (io.ReadCloser, error) {
	return nil, nil
}
func (d *testDockerManager) ListContainers() ([]string, error)                     { return nil, nil }
func (d *testDockerManager) CreateVolume(volumeName string) error                  { return nil }
func (d *testDockerManager) RemoveVolume(volumeName string) error                  { return nil }
func (d *testDockerManager) GetVolumeInfo(volumeName string) (*models.VolumeInfo, error) {
	return nil, nil
}
func (d *testDockerManager) CreateBackup(gameserverID, backupPath string) error { return nil }
func (d *testDockerManager) RestoreBackup(gameserverID, backupPath string) error {
	return nil
}
func (d *testDockerManager) CleanupOldBackups(containerID string, maxBackups int) error {
	return nil
}
func (d *testDockerManager) ListFiles(containerID string, path string) ([]*models.FileInfo, error) {
	return nil, nil
}
func (d *testDockerManager) ReadFile(containerID string, path string) ([]byte, error) {
	return nil, nil
}
func (d *testDockerManager) WriteFile(containerID string, path string, content []byte) error {
	return nil
}
func (d *testDockerManager) CreateDirectory(containerID string, path string) error { return nil }
func (d *testDockerManager) DeletePath(containerID string, path string) error     { return nil }
func (d *testDockerManager) DownloadFile(containerID string, path string) (io.ReadCloser, error) {
	return nil, nil
}
func (d *testDockerManager) UploadFile(containerID string, destPath string, reader io.Reader) error {
	return nil
}
func (d *testDockerManager) RenameFile(containerID string, oldPath string, newPath string) error {
	return nil
}

func TestServiceCreateGameserver(t *testing.T) {
	tests := []struct {
		name        string
		req         CreateGameserverRequest
		expectError bool
		errorType   string
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
			errorType:   "BadRequest",
		},
		{
			name: "invalid_game",
			req: CreateGameserverRequest{
				Name:   "test-server",
				GameID: "nonexistent",
			},
			expectError: true,
			errorType:   "NotFound",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db := newTestGameserverDB()
			docker := newTestDockerManager()
			service := NewGameserverService(db, docker, "/tmp")

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
	db := newTestGameserverDB()
	docker := newTestDockerManager()
	service := NewGameserverService(db, docker, "/tmp")

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
package main

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
			"minecraft": {ID: "minecraft", Name: "minecraft", Image: "minecraft:latest", PortMappings: []models.PortMapping{{Protocol: "tcp", ContainerPort: 25565, HostPort: 0}}},
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
	db.gameservers[gs.ID] = gs
	return nil
}

func (db *testGameserverDB) DeleteGameserver(id string) error {
	delete(db.gameservers, id)
	return nil
}

func (db *testGameserverDB) ListGameservers() ([]*models.Gameserver, error) {
	var result []*models.Gameserver
	for _, gs := range db.gameservers {
		result = append(result, gs)
	}
	return result, nil
}

func (db *testGameserverDB) StartGameserver(id string) error         { return nil }
func (db *testGameserverDB) StopGameserver(id string) error          { return nil }
func (db *testGameserverDB) RestartGameserver(id string) error       { return nil }
func (db *testGameserverDB) SendGameserverCommand(id string, command string) error { return nil }
func (db *testGameserverDB) StreamGameserverLogs(id string) (io.ReadCloser, error) { return nil, nil }
func (db *testGameserverDB) StreamGameserverStats(id string) (io.ReadCloser, error) { return nil, nil }
func (db *testGameserverDB) ListGames() ([]*models.Game, error)                           { return nil, nil }
func (db *testGameserverDB) CreateGame(game *models.Game) error                           { return nil }
func (db *testGameserverDB) CreateScheduledTask(task *models.ScheduledTask) error         { return nil }
func (db *testGameserverDB) GetScheduledTask(id string) (*models.ScheduledTask, error)    { return nil, nil }
func (db *testGameserverDB) UpdateScheduledTask(task *models.ScheduledTask) error         { return nil }
func (db *testGameserverDB) DeleteScheduledTask(id string) error                   { return nil }
func (db *testGameserverDB) ListScheduledTasksForGameserver(gameserverID string) ([]*models.ScheduledTask, error) { return nil, nil }
func (db *testGameserverDB) CreateGameserverBackup(gameserverID string) error     { return nil }
func (db *testGameserverDB) RestoreGameserverBackup(gameserverID, backupFilename string) error { return nil }
func (db *testGameserverDB) ListGameserverBackups(gameserverID string) ([]*models.FileInfo, error) { return nil, nil }
func (db *testGameserverDB) ListFiles(containerID string, path string) ([]*models.FileInfo, error) { return nil, nil }
func (db *testGameserverDB) ReadFile(containerID string, path string) ([]byte, error) { return nil, nil }
func (db *testGameserverDB) WriteFile(containerID string, path string, content []byte) error { return nil }
func (db *testGameserverDB) CreateDirectory(containerID string, path string) error { return nil }
func (db *testGameserverDB) DeletePath(containerID string, path string) error     { return nil }
func (db *testGameserverDB) DownloadFile(containerID string, path string) (io.ReadCloser, error) { return nil, nil }
func (db *testGameserverDB) RenameFile(containerID string, oldPath string, newPath string) error { return nil }
func (db *testGameserverDB) UploadFile(containerID string, destPath string, reader io.Reader) error { return nil }

type testDockerManager struct {
	containers map[string]bool
}

func newTestDockerManager() *testDockerManager {
	return &testDockerManager{
		containers: make(map[string]bool),
	}
}

func (d *testDockerManager) CreateContainer(gs *models.Gameserver) error {
	gs.ContainerID = "container-" + gs.Name
	d.containers[gs.ContainerID] = false
	return nil
}

func (d *testDockerManager) StartContainer(containerID string) error {
	d.containers[containerID] = true
	return nil
}

func (d *testDockerManager) StopContainer(containerID string) error {
	d.containers[containerID] = false
	return nil
}

func (d *testDockerManager) RemoveContainer(containerID string) error {
	delete(d.containers, containerID)
	return nil
}

func (d *testDockerManager) CreateBackup(gameserverID, path string) error {
	return nil
}

func (d *testDockerManager) RestoreBackup(gameserverID, path string) error {
	return nil
}

func (d *testDockerManager) SendCommand(containerID string, command string) error { return nil }
func (d *testDockerManager) GetContainerStatus(containerID string) (models.GameserverStatus, error) {
	return models.StatusStopped, nil
}
func (d *testDockerManager) StreamContainerLogs(containerID string) (io.ReadCloser, error) { return nil, nil }
func (d *testDockerManager) StreamContainerStats(containerID string) (io.ReadCloser, error) { return nil, nil }
func (d *testDockerManager) ListContainers() ([]string, error)               { return nil, nil }
func (d *testDockerManager) CreateVolume(volumeName string) error            { return nil }
func (d *testDockerManager) RemoveVolume(volumeName string) error            { return nil }
func (d *testDockerManager) GetVolumeInfo(volumeName string) (*models.VolumeInfo, error) { return nil, nil }
func (d *testDockerManager) CleanupOldBackups(containerID string, maxBackups int) error { return nil }
func (d *testDockerManager) ListFiles(containerID string, path string) ([]*models.FileInfo, error) { return nil, nil }
func (d *testDockerManager) ReadFile(containerID string, path string) ([]byte, error) { return nil, nil }
func (d *testDockerManager) WriteFile(containerID string, path string, content []byte) error { return nil }
func (d *testDockerManager) CreateDirectory(containerID string, path string) error { return nil }
func (d *testDockerManager) DeletePath(containerID string, path string) error { return nil }
func (d *testDockerManager) DownloadFile(containerID string, path string) (io.ReadCloser, error) { return nil, nil }
func (d *testDockerManager) UploadFile(containerID string, destPath string, reader io.Reader) error { return nil }
func (d *testDockerManager) RenameFile(containerID string, oldPath string, newPath string) error { return nil }

func TestServiceCreateGameserver(t *testing.T) {
	db := newTestGameserverDB()
	docker := newTestDockerManager()
	svc := NewService(db, docker, "/tmp")

	tests := []struct {
		name    string
		req     CreateGameserverRequest
		wantErr bool
	}{
		{
			name: "valid request",
			req: CreateGameserverRequest{
				Name:     "test-server",
				GameID:   "minecraft",
				Port:     25565,
				MemoryMB: 1024,
			},
			wantErr: false,
		},
		{
			name: "missing name",
			req: CreateGameserverRequest{
				GameID: "minecraft",
			},
			wantErr: true,
		},
		{
			name: "invalid game",
			req: CreateGameserverRequest{
				Name:   "test",
				GameID: "invalid",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gs, err := svc.CreateGameserver(context.Background(), tt.req)

			if tt.wantErr && err == nil {
				t.Error("expected error but got none")
			}
			if !tt.wantErr && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
			if !tt.wantErr && gs == nil {
				t.Error("expected gameserver but got nil")
			}
		})
	}
}

func TestServiceLifecycle(t *testing.T) {
	db := newTestGameserverDB()
	docker := newTestDockerManager()
	svc := NewService(db, docker, "/tmp")

	// Create gameserver
	gs, err := svc.CreateGameserver(context.Background(), CreateGameserverRequest{
		Name:   "test-server",
		GameID: "minecraft",
	})
	if err != nil {
		t.Fatalf("failed to create gameserver: %v", err)
	}

	// Start gameserver
	if err := svc.StartGameserver(context.Background(), gs.ID); err != nil {
		t.Errorf("failed to start gameserver: %v", err)
	}

	// Check status
	gs, _ = db.GetGameserver(gs.ID)
	if gs.Status != models.StatusRunning {
		t.Errorf("expected status running, got %s", gs.Status)
	}

	// Stop gameserver
	if err := svc.StopGameserver(context.Background(), gs.ID); err != nil {
		t.Errorf("failed to stop gameserver: %v", err)
	}

	// Check status
	gs, _ = db.GetGameserver(gs.ID)
	if gs.Status != models.StatusStopped {
		t.Errorf("expected status stopped, got %s", gs.Status)
	}

	// Delete gameserver
	if err := svc.DeleteGameserver(context.Background(), gs.ID); err != nil {
		t.Errorf("failed to delete gameserver: %v", err)
	}

	// Verify deletion
	if _, err := db.GetGameserver(gs.ID); err == nil {
		t.Error("expected gameserver to be deleted")
	}
}
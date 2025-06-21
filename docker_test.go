package main

import (
	"io"
	"strings"
	"testing"
	"time"
)

// =============================================================================
// Mock Docker Manager for Testing
// =============================================================================

type MockDockerManager struct {
	containers map[string]*Gameserver
	logs       map[string][]string
	shouldFail map[string]bool
}

func NewMockDockerManager() *MockDockerManager {
	return &MockDockerManager{
		containers: make(map[string]*Gameserver),
		logs:       make(map[string][]string),
		shouldFail: make(map[string]bool),
	}
}

func (m *MockDockerManager) CreateContainer(server *Gameserver) error {
	if m.shouldFail["create"] {
		return &DockerError{Op: "create", Msg: "mock create error"}
	}
	server.ContainerID = "mock-container-" + server.ID
	server.Status = StatusStopped
	m.containers[server.ContainerID] = server
	return nil
}

func (m *MockDockerManager) StartContainer(containerID string) error {
	if m.shouldFail["start"] {
		return &DockerError{Op: "start", Msg: "mock start error"}
	}
	if server, exists := m.containers[containerID]; exists {
		server.Status = StatusRunning
		return nil
	}
	return &DockerError{Op: "start", Msg: "container not found"}
}



func (m *MockDockerManager) RemoveContainer(containerID string) error {
	if m.shouldFail["remove"] {
		return &DockerError{Op: "remove", Msg: "mock remove error"}
	}
	delete(m.containers, containerID)
	return nil
}

func (m *MockDockerManager) GetContainerStatus(containerID string) (GameserverStatus, error) {
	if m.shouldFail["status"] {
		return StatusError, &DockerError{Op: "status", Msg: "mock status error"}
	}
	if server, exists := m.containers[containerID]; exists {
		return server.Status, nil
	}
	return StatusError, &DockerError{Op: "status", Msg: "container not found"}
}


func (m *MockDockerManager) StreamContainerLogs(containerID string) (io.ReadCloser, error) {
	if m.shouldFail["stream_logs"] {
		return nil, &DockerError{Op: "stream_logs", Msg: "mock stream logs error"}
	}
	return io.NopCloser(strings.NewReader("Mock log stream")), nil
}

func (m *MockDockerManager) StreamContainerStats(containerID string) (io.ReadCloser, error) {
	if m.shouldFail["stream_stats"] {
		return nil, &DockerError{Op: "stream_stats", Msg: "mock stream stats error"}
	}
	return io.NopCloser(strings.NewReader(`{"cpu_stats":{"cpu_usage":{"total_usage":100},"system_cpu_usage":200},"precpu_stats":{"cpu_usage":{"total_usage":50},"system_cpu_usage":100},"memory_stats":{"usage":536870912,"limit":1073741824}}`)), nil
}

func (m *MockDockerManager) ListContainers() ([]string, error) {
	if m.shouldFail["list"] {
		return nil, &DockerError{Op: "list", Msg: "mock list error"}
	}
	containers := make([]string, 0, len(m.containers))
	for id := range m.containers {
		containers = append(containers, id)
	}
	return containers, nil
}

func (m *MockDockerManager) CreateVolume(volumeName string) error {
	if m.shouldFail["create_volume"] {
		return &DockerError{Op: "create_volume", Msg: "mock create volume error"}
	}
	return nil
}

func (m *MockDockerManager) RemoveVolume(volumeName string) error {
	if m.shouldFail["remove_volume"] {
		return &DockerError{Op: "remove_volume", Msg: "mock remove volume error"}
	}
	return nil
}

func (m *MockDockerManager) GetVolumeInfo(volumeName string) (*VolumeInfo, error) {
	if m.shouldFail["get_volume_info"] {
		return nil, &DockerError{Op: "get_volume_info", Msg: "mock get volume info error"}
	}
	return &VolumeInfo{
		Name:       volumeName,
		MountPoint: "/var/lib/docker/volumes/" + volumeName + "/_data",
		Driver:     "local",
		CreatedAt:  "2025-06-21T04:00:00Z",
		Labels:     map[string]string{"gameserver.managed": "true"},
	}, nil
}

func (m *MockDockerManager) CreateBackup(containerID, gameserverName string) error {
	if m.shouldFail["create_backup"] {
		return &DockerError{Op: "create_backup", Msg: "mock create backup error"}
	}
	return nil
}

func (m *MockDockerManager) RestoreBackup(containerID, backupFilename string) error {
	if m.shouldFail["restore_backup"] {
		return &DockerError{Op: "restore_backup", Msg: "mock restore backup error"}
	}
	return nil
}

// File manager methods
func (m *MockDockerManager) ListFiles(containerID string, path string) ([]*FileInfo, error) {
	if m.shouldFail["list_files"] {
		return nil, &DockerError{Op: "list_files", Msg: "mock list files error"}
	}
	modTime, _ := time.Parse(time.RFC3339, "2025-06-21T00:00:00Z")
	return []*FileInfo{
		{Name: "server.properties", Size: 1024, IsDir: false, Modified: modTime},
		{Name: "logs", Size: 0, IsDir: true, Modified: modTime},
	}, nil
}

func (m *MockDockerManager) ReadFile(containerID string, path string) ([]byte, error) {
	if m.shouldFail["read_file"] {
		return nil, &DockerError{Op: "read_file", Msg: "mock read file error"}
	}
	return []byte("mock file content"), nil
}

func (m *MockDockerManager) WriteFile(containerID string, path string, content []byte) error {
	if m.shouldFail["write_file"] {
		return &DockerError{Op: "write_file", Msg: "mock write file error"}
	}
	return nil
}

func (m *MockDockerManager) CreateDirectory(containerID string, path string) error {
	if m.shouldFail["create_directory"] {
		return &DockerError{Op: "create_directory", Msg: "mock create directory error"}
	}
	return nil
}

func (m *MockDockerManager) DeletePath(containerID string, path string) error {
	if m.shouldFail["delete_path"] {
		return &DockerError{Op: "delete_path", Msg: "mock delete path error"}
	}
	return nil
}

func (m *MockDockerManager) DownloadFile(containerID string, path string) (io.ReadCloser, error) {
	if m.shouldFail["download_file"] {
		return nil, &DockerError{Op: "download_file", Msg: "mock download file error"}
	}
	return io.NopCloser(strings.NewReader("mock file content")), nil
}

func (m *MockDockerManager) RenameFile(containerID string, oldPath string, newPath string) error {
	if m.shouldFail["rename_file"] {
		return &DockerError{Op: "rename_file", Msg: "mock rename file error"}
	}
	return nil
}

func (m *MockDockerManager) SendCommand(containerID string, command string) error {
	if m.shouldFail["send_command"] {
		return &DockerError{Op: "send_command", Msg: "mock send command error"}
	}
	return nil
}

func (m *MockDockerManager) ExecCommand(containerID string, cmd []string) ([]byte, error) {
	if m.shouldFail["exec_command"] {
		return nil, &DockerError{Op: "exec_command", Msg: "mock exec command error"}
	}
	return []byte("mock command output"), nil
}

func (m *MockDockerManager) UploadFile(containerID string, destPath string, reader io.Reader) error {
	if m.shouldFail["upload_file"] {
		return &DockerError{Op: "upload_file", Msg: "mock upload file error"}
	}
	return nil
}

func (m *MockDockerManager) CleanupOldBackups(containerID string, maxBackups int) error {
	if m.shouldFail["cleanup_backups"] {
		return &DockerError{Op: "cleanup_backups", Msg: "mock cleanup backups error"}
	}
	return nil
}

// Smart pull methods for testing
func (m *MockDockerManager) pullImageIfNeeded(imageName string) error {
	if m.shouldFail["pull_image"] {
		return &DockerError{Op: "pull_image", Msg: "mock pull image error"}
	}
	return nil
}

func (m *MockDockerManager) shouldPullImage(imageName string) (bool, error) {
	if m.shouldFail["check_image"] {
		return false, &DockerError{Op: "check_image", Msg: "mock check image error"}
	}
	// Default behavior: only pull if image name contains "latest" or "new"
	return strings.Contains(imageName, "latest") || strings.Contains(imageName, "new"), nil
}

// =============================================================================
// Container Creation Tests
// =============================================================================

func TestCreateContainer(t *testing.T) {
	tests := []struct {
		name      string
		server    *Gameserver
		shouldErr bool
	}{
		{
			name: "successful creation",
			server: &Gameserver{
				ID:       "test-1",
				Name:     "Test Minecraft Server",
				GameType: "minecraft",
				Image:    "ghcr.io/0xkowalskidev/gameservers/minecraft:1.20.4",
				Port:     25565,
			},
			shouldErr: false,
		},
		{
			name: "creation with environment variables",
			server: &Gameserver{
				ID:          "test-2",
				Name:        "Test CS2 Server",
				GameType:    "cs2",
				Image:       "ghcr.io/0xkowalskidev/gameservers/cs2:latest",
				Port:        27015,
				Environment: []string{"GSLT_TOKEN=abc123", "MAP=de_dust2"},
			},
			shouldErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := NewMockDockerManager()
			if tt.shouldErr {
				mock.shouldFail["create"] = true
			}

			err := mock.CreateContainer(tt.server)

			if tt.shouldErr && err == nil {
				t.Errorf("expected error but got none")
			}
			if !tt.shouldErr && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
			if !tt.shouldErr {
				if tt.server.ContainerID == "" {
					t.Errorf("expected ContainerID to be set")
				}
				if tt.server.Status != StatusStopped {
					t.Errorf("expected status to be %s, got %s", StatusStopped, tt.server.Status)
				}
			}
		})
	}
}

// =============================================================================
// Container Lifecycle Tests (Start/Stop/Restart)
// =============================================================================

func TestStartContainer(t *testing.T) {
	mock := NewMockDockerManager()
	server := &Gameserver{
		ID:       "test-1",
		Name:     "Test Server",
		GameType: "minecraft",
		Image:    "minecraft:latest",
		Port:     25565,
	}

	// Create container first
	err := mock.CreateContainer(server)
	if err != nil {
		t.Fatalf("failed to create container: %v", err)
	}

	tests := []struct {
		name        string
		containerID string
		shouldErr   bool
	}{
		{
			name:        "successful start",
			containerID: server.ContainerID,
			shouldErr:   false,
		},
		{
			name:        "start non-existent container",
			containerID: "non-existent",
			shouldErr:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := mock.StartContainer(tt.containerID)

			if tt.shouldErr && err == nil {
				t.Errorf("expected error but got none")
			}
			if !tt.shouldErr && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
			if !tt.shouldErr {
				status, _ := mock.GetContainerStatus(tt.containerID)
				if status != StatusRunning {
					t.Errorf("expected status to be %s, got %s", StatusRunning, status)
				}
			}
		})
	}
}



// =============================================================================
// Container Monitoring Tests (Stats/Logs/Status)
// =============================================================================


// =============================================================================
// Container Management Tests (List/Remove)
// =============================================================================

func TestListContainers(t *testing.T) {
	mock := NewMockDockerManager()

	// Create a few containers
	servers := []*Gameserver{
		{ID: "test-1", Name: "Server 1", GameType: "minecraft", Image: "minecraft:latest", Port: 25565},
		{ID: "test-2", Name: "Server 2", GameType: "cs2", Image: "cs2:latest", Port: 27015},
	}

	for _, server := range servers {
		mock.CreateContainer(server)
	}

	containers, err := mock.ListContainers()
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if len(containers) != len(servers) {
		t.Errorf("expected %d containers, got %d", len(servers), len(containers))
	}
}

// =============================================================================
// Error Handling Tests
// =============================================================================

func TestDockerError(t *testing.T) {
	mock := NewMockDockerManager()
	mock.shouldFail["create"] = true

	server := &Gameserver{
		ID:       "test-1",
		Name:     "Test Server",
		GameType: "minecraft",
		Image:    "minecraft:latest",
		Port:     25565,
	}

	err := mock.CreateContainer(server)
	if err == nil {
		t.Errorf("expected error but got none")
	}

	if dockerErr, ok := err.(*DockerError); ok {
		if dockerErr.Op != "create" {
			t.Errorf("expected operation to be 'create', got %s", dockerErr.Op)
		}
	} else {
		t.Errorf("expected DockerError type")
	}
}

// =============================================================================
// Smart Pull Strategy Tests
// =============================================================================

func TestSmartPullStrategy(t *testing.T) {
	tests := []struct {
		name        string
		imageName   string
		shouldPull  bool
		expectError bool
	}{
		{
			name:        "pull latest tag",
			imageName:   "minecraft:latest",
			shouldPull:  true,
			expectError: false,
		},
		{
			name:        "pull new version",
			imageName:   "minecraft:new-version",
			shouldPull:  true,
			expectError: false,
		},
		{
			name:        "skip stable version",
			imageName:   "minecraft:1.20.4",
			shouldPull:  false,
			expectError: false,
		},
		{
			name:        "skip specific digest",
			imageName:   "minecraft@sha256:abcd1234",
			shouldPull:  false,
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := NewMockDockerManager()
			
			// Test should pull logic
			shouldPull, err := mock.shouldPullImage(tt.imageName)
			if tt.expectError && err == nil {
				t.Errorf("expected error but got none")
			}
			if !tt.expectError && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
			if shouldPull != tt.shouldPull {
				t.Errorf("expected shouldPull=%v, got %v", tt.shouldPull, shouldPull)
			}
			
			// Test pull if needed
			err = mock.pullImageIfNeeded(tt.imageName)
			if !tt.expectError && err != nil {
				t.Errorf("unexpected error in pullImageIfNeeded: %v", err)
			}
		})
	}
}

func TestSmartPullStrategy_Errors(t *testing.T) {
	tests := []struct {
		name        string
		failMethod  string
		imageName   string
		expectError bool
	}{
		{
			name:        "check image fails",
			failMethod:  "check_image",
			imageName:   "minecraft:latest",
			expectError: true,
		},
		{
			name:        "pull image fails",
			failMethod:  "pull_image", 
			imageName:   "minecraft:latest",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := NewMockDockerManager()
			mock.shouldFail[tt.failMethod] = true
			
			if tt.failMethod == "check_image" {
				_, err := mock.shouldPullImage(tt.imageName)
				if tt.expectError && err == nil {
					t.Errorf("expected error but got none")
				}
			} else if tt.failMethod == "pull_image" {
				err := mock.pullImageIfNeeded(tt.imageName)
				if tt.expectError && err == nil {
					t.Errorf("expected error but got none")
				}
			}
		})
	}
}

// =============================================================================
// Volume Management Tests
// =============================================================================

func TestGetVolumeInfo(t *testing.T) {
	mock := NewMockDockerManager()
	
	volumeName := "gameservers-test-data"
	
	volumeInfo, err := mock.GetVolumeInfo(volumeName)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	
	if volumeInfo.Name != volumeName {
		t.Errorf("expected volume name %s, got %s", volumeName, volumeInfo.Name)
	}
	
	if volumeInfo.Driver != "local" {
		t.Errorf("expected driver 'local', got %s", volumeInfo.Driver)
	}
}

func TestGetVolumeInfoError(t *testing.T) {
	mock := NewMockDockerManager()
	mock.shouldFail["get_volume_info"] = true
	
	volumeName := "gameservers-test-data"
	
	_, err := mock.GetVolumeInfo(volumeName)
	if err == nil {
		t.Errorf("expected error but got none")
	}
}

// =============================================================================
// Backup and Restore Tests
// =============================================================================

func TestCreateBackup(t *testing.T) {
	tests := []struct {
		name           string
		containerID    string
		gameserverName string
		shouldErr      bool
	}{
		{
			name:           "successful backup",
			containerID:    "container-123",
			gameserverName: "test-server",
			shouldErr:      false,
		},
		{
			name:           "backup failure",
			containerID:    "failing-container",
			gameserverName: "failing-server",
			shouldErr:      true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := NewMockDockerManager()
			if tt.shouldErr {
				mock.shouldFail["create_backup"] = true
			}

			err := mock.CreateBackup(tt.containerID, tt.gameserverName)

			if tt.shouldErr && err == nil {
				t.Errorf("expected error but got none")
			}
			if !tt.shouldErr && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}

func TestRestoreBackup(t *testing.T) {
	tests := []struct {
		name            string
		containerID     string
		backupFilename  string
		shouldErr       bool
	}{
		{
			name:           "successful restore",
			containerID:    "container-123",
			backupFilename: "test-server_2025-06-21.tar.gz",
			shouldErr:      false,
		},
		{
			name:           "restore failure",
			containerID:    "failing-container",
			backupFilename: "failing-backup.tar.gz",
			shouldErr:      true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := NewMockDockerManager()
			if tt.shouldErr {
				mock.shouldFail["restore_backup"] = true
			}

			err := mock.RestoreBackup(tt.containerID, tt.backupFilename)

			if tt.shouldErr && err == nil {
				t.Errorf("expected error but got none")
			}
			if !tt.shouldErr && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}
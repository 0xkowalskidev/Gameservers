package main

import (
	"testing"
)

// =============================================================================
// Mock Docker Manager for Testing
// =============================================================================

type MockDockerManager struct {
	containers map[string]*GameServer
	logs       map[string][]string
	stats      map[string]*ContainerStats
	shouldFail map[string]bool
}

func NewMockDockerManager() *MockDockerManager {
	return &MockDockerManager{
		containers: make(map[string]*GameServer),
		logs:       make(map[string][]string),
		stats:      make(map[string]*ContainerStats),
		shouldFail: make(map[string]bool),
	}
}

func (m *MockDockerManager) CreateContainer(server *GameServer) error {
	if m.shouldFail["create"] {
		return &DockerError{Operation: "create", Message: "mock create error"}
	}
	server.ContainerID = "mock-container-" + server.ID
	server.Status = StatusStopped
	m.containers[server.ContainerID] = server
	return nil
}

func (m *MockDockerManager) StartContainer(containerID string) error {
	if m.shouldFail["start"] {
		return &DockerError{Operation: "start", Message: "mock start error"}
	}
	if server, exists := m.containers[containerID]; exists {
		server.Status = StatusRunning
		return nil
	}
	return &DockerError{Operation: "start", Message: "container not found"}
}

func (m *MockDockerManager) StopContainer(containerID string) error {
	if m.shouldFail["stop"] {
		return &DockerError{Operation: "stop", Message: "mock stop error"}
	}
	if server, exists := m.containers[containerID]; exists {
		server.Status = StatusStopped
		return nil
	}
	return &DockerError{Operation: "stop", Message: "container not found"}
}

func (m *MockDockerManager) RestartContainer(containerID string) error {
	if m.shouldFail["restart"] {
		return &DockerError{Operation: "restart", Message: "mock restart error"}
	}
	if server, exists := m.containers[containerID]; exists {
		server.Status = StatusRunning
		return nil
	}
	return &DockerError{Operation: "restart", Message: "container not found"}
}

func (m *MockDockerManager) RemoveContainer(containerID string) error {
	if m.shouldFail["remove"] {
		return &DockerError{Operation: "remove", Message: "mock remove error"}
	}
	delete(m.containers, containerID)
	return nil
}

func (m *MockDockerManager) GetContainerStatus(containerID string) (GameServerStatus, error) {
	if m.shouldFail["status"] {
		return StatusError, &DockerError{Operation: "status", Message: "mock status error"}
	}
	if server, exists := m.containers[containerID]; exists {
		return server.Status, nil
	}
	return StatusError, &DockerError{Operation: "status", Message: "container not found"}
}

func (m *MockDockerManager) GetContainerStats(containerID string) (*ContainerStats, error) {
	if m.shouldFail["stats"] {
		return nil, &DockerError{Operation: "stats", Message: "mock stats error"}
	}
	if stats, exists := m.stats[containerID]; exists {
		return stats, nil
	}
	return &ContainerStats{
		CPUPercent:    25.5,
		MemoryUsage:   1024 * 1024 * 512, // 512MB
		MemoryLimit:   1024 * 1024 * 1024, // 1GB
		MemoryPercent: 50.0,
		NetworkRx:     1024 * 100,
		NetworkTx:     1024 * 200,
	}, nil
}

func (m *MockDockerManager) GetContainerLogs(containerID string, lines int) ([]string, error) {
	if m.shouldFail["logs"] {
		return nil, &DockerError{Operation: "logs", Message: "mock logs error"}
	}
	if logs, exists := m.logs[containerID]; exists {
		if lines > len(logs) {
			return logs, nil
		}
		return logs[len(logs)-lines:], nil
	}
	return []string{"Mock log line 1", "Mock log line 2"}, nil
}

func (m *MockDockerManager) ListContainers() ([]string, error) {
	if m.shouldFail["list"] {
		return nil, &DockerError{Operation: "list", Message: "mock list error"}
	}
	containers := make([]string, 0, len(m.containers))
	for id := range m.containers {
		containers = append(containers, id)
	}
	return containers, nil
}

// =============================================================================
// Container Creation Tests
// =============================================================================

func TestCreateContainer(t *testing.T) {
	tests := []struct {
		name      string
		server    *GameServer
		shouldErr bool
	}{
		{
			name: "successful creation",
			server: &GameServer{
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
			server: &GameServer{
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
	server := &GameServer{
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

func TestStopContainer(t *testing.T) {
	mock := NewMockDockerManager()
	server := &GameServer{
		ID:       "test-1",
		Name:     "Test Server",
		GameType: "minecraft",
		Image:    "minecraft:latest",
		Port:     25565,
	}

	// Create and start container
	mock.CreateContainer(server)
	mock.StartContainer(server.ContainerID)

	err := mock.StopContainer(server.ContainerID)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	status, _ := mock.GetContainerStatus(server.ContainerID)
	if status != StatusStopped {
		t.Errorf("expected status to be %s, got %s", StatusStopped, status)
	}
}

func TestRestartContainer(t *testing.T) {
	mock := NewMockDockerManager()
	server := &GameServer{
		ID:       "test-1",
		Name:     "Test Server",
		GameType: "minecraft",
		Image:    "minecraft:latest",
		Port:     25565,
	}

	// Create container
	mock.CreateContainer(server)

	err := mock.RestartContainer(server.ContainerID)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	status, _ := mock.GetContainerStatus(server.ContainerID)
	if status != StatusRunning {
		t.Errorf("expected status to be %s, got %s", StatusRunning, status)
	}
}

// =============================================================================
// Container Monitoring Tests (Stats/Logs/Status)
// =============================================================================

func TestGetContainerStats(t *testing.T) {
	mock := NewMockDockerManager()
	server := &GameServer{
		ID:       "test-1",
		Name:     "Test Server",
		GameType: "minecraft",
		Image:    "minecraft:latest",
		Port:     25565,
	}

	mock.CreateContainer(server)

	stats, err := mock.GetContainerStats(server.ContainerID)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if stats == nil {
		t.Errorf("expected stats to not be nil")
	}

	if stats.CPUPercent <= 0 {
		t.Errorf("expected CPUPercent to be positive, got %f", stats.CPUPercent)
	}

	if stats.MemoryUsage <= 0 {
		t.Errorf("expected MemoryUsage to be positive, got %d", stats.MemoryUsage)
	}
}

func TestGetContainerLogs(t *testing.T) {
	mock := NewMockDockerManager()
	server := &GameServer{
		ID:       "test-1",
		Name:     "Test Server",
		GameType: "minecraft",
		Image:    "minecraft:latest",
		Port:     25565,
	}

	mock.CreateContainer(server)

	logs, err := mock.GetContainerLogs(server.ContainerID, 10)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if len(logs) == 0 {
		t.Errorf("expected logs to not be empty")
	}
}

// =============================================================================
// Container Management Tests (List/Remove)
// =============================================================================

func TestListContainers(t *testing.T) {
	mock := NewMockDockerManager()

	// Create a few containers
	servers := []*GameServer{
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

	server := &GameServer{
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
		if dockerErr.Operation != "create" {
			t.Errorf("expected operation to be 'create', got %s", dockerErr.Operation)
		}
	} else {
		t.Errorf("expected DockerError type")
	}
}
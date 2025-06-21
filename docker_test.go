package main

import (
	"io"
	"strings"
	"testing"
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
package docker

import (
	"testing"

	"0xkowalskidev/gameservers/models"
)

func TestMockDockerManager_CreateContainer(t *testing.T) {
	tests := []struct {
		name        string
		server      *models.Gameserver
		shouldFail  bool
		expectError bool
	}{
		{
			name: "successful creation",
			server: &models.Gameserver{
				ID:       "test-1",
				Name:     "test-server",
				GameID:   "minecraft",
				Image:    "minecraft:latest",
				MemoryMB: 1024,
				CPUCores: 1.0,
				PortMappings: []models.PortMapping{
					{Name: "game", Protocol: "tcp", ContainerPort: 25565, HostPort: 25565},
				},
				Environment: []string{"EULA=true"},
			},
			shouldFail:  false,
			expectError: false,
		},
		{
			name: "creation with environment variables",
			server: &models.Gameserver{
				ID:       "test-2",
				Name:     "env-server",
				GameID:   "minecraft",
				Image:    "minecraft:latest",
				MemoryMB: 2048,
				PortMappings: []models.PortMapping{
					{Name: "game", Protocol: "tcp", ContainerPort: 25565, HostPort: 25566},
				},
				Environment: []string{"EULA=true", "DIFFICULTY=hard", "MOTD=Test Server"},
			},
			shouldFail:  false,
			expectError: false,
		},
		{
			name: "creation failure",
			server: &models.Gameserver{
				ID:       "test-3",
				Name:     "fail-server",
				GameID:   "minecraft",
				Image:    "minecraft:latest",
				MemoryMB: 1024,
			},
			shouldFail:  true,
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := NewMockDockerManager()
			if tt.shouldFail {
				mock.SetShouldFail("create", true)
			}

			err := mock.CreateContainer(tt.server)

			if tt.expectError {
				if err == nil {
					t.Error("Expected error but got none")
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
				}

				// Check that container was created correctly
				if tt.server.ContainerID == "" {
					t.Error("Container ID was not set")
				}

				if tt.server.Status != models.StatusStopped {
					t.Errorf("Expected status %v, got %v", models.StatusStopped, tt.server.Status)
				}

				// Check that container exists in mock
				container := mock.GetContainer(tt.server.ContainerID)
				if container == nil {
					t.Error("Container not found in mock")
				}
			}
		})
	}
}

func TestMockDockerManager_StartContainer(t *testing.T) {
	tests := []struct {
		name           string
		containerID    string
		setupContainer bool
		shouldFail     bool
		expectError    bool
		expectedStatus models.GameserverStatus
	}{
		{
			name:           "successful start",
			containerID:    "mock-container-test",
			setupContainer: true,
			shouldFail:     false,
			expectError:    false,
			expectedStatus: models.StatusRunning,
		},
		{
			name:           "start non-existent container",
			containerID:    "non-existent",
			setupContainer: false,
			shouldFail:     false,
			expectError:    true,
		},
		{
			name:           "start failure",
			containerID:    "mock-container-test",
			setupContainer: true,
			shouldFail:     true,
			expectError:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := NewMockDockerManager()

			if tt.setupContainer {
				server := &models.Gameserver{
					ID:     "test",
					Name:   "test-server",
					Status: models.StatusStopped,
				}
				mock.CreateContainer(server)
				tt.containerID = server.ContainerID
			}

			if tt.shouldFail {
				mock.SetShouldFail("start", true)
			}

			err := mock.StartContainer(tt.containerID)

			if tt.expectError {
				if err == nil {
					t.Error("Expected error but got none")
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
				}

				// Check status was updated
				status, statusErr := mock.GetContainerStatus(tt.containerID)
				if statusErr != nil {
					t.Errorf("Failed to get container status: %v", statusErr)
				}
				if status != tt.expectedStatus {
					t.Errorf("Expected status %v, got %v", tt.expectedStatus, status)
				}
			}
		})
	}
}

func TestMockDockerManager_StopContainer(t *testing.T) {
	mock := NewMockDockerManager()

	// Setup container
	server := &models.Gameserver{
		ID:     "test",
		Name:   "test-server",
		Status: models.StatusRunning,
	}
	mock.CreateContainer(server)
	mock.StartContainer(server.ContainerID)

	err := mock.StopContainer(server.ContainerID)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	// Check status was updated
	status, _ := mock.GetContainerStatus(server.ContainerID)
	if status != models.StatusStopped {
		t.Errorf("Expected status %v, got %v", models.StatusStopped, status)
	}
}

func TestMockDockerManager_RemoveContainer(t *testing.T) {
	mock := NewMockDockerManager()

	// Setup container
	server := &models.Gameserver{
		ID:   "test",
		Name: "test-server",
	}
	mock.CreateContainer(server)

	err := mock.RemoveContainer(server.ContainerID)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	// Check container was removed
	container := mock.GetContainer(server.ContainerID)
	if container != nil {
		t.Error("Container should have been removed")
	}
}

func TestMockDockerManager_GetContainerStatus(t *testing.T) {
	mock := NewMockDockerManager()

	// Test non-existent container
	_, err := mock.GetContainerStatus("non-existent")
	if err == nil {
		t.Error("Expected error for non-existent container")
	}

	// Setup container and test status
	server := &models.Gameserver{
		ID:     "test",
		Name:   "test-server",
		Status: models.StatusStopped,
	}
	mock.CreateContainer(server)

	status, err := mock.GetContainerStatus(server.ContainerID)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	if status != models.StatusStopped {
		t.Errorf("Expected status %v, got %v", models.StatusStopped, status)
	}
}

func TestMockDockerManager_ListContainers(t *testing.T) {
	mock := NewMockDockerManager()

	// Initially should be empty
	containers, err := mock.ListContainers()
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	if len(containers) != 0 {
		t.Errorf("Expected 0 containers, got %d", len(containers))
	}

	// Add some containers
	server1 := &models.Gameserver{ID: "test1", Name: "server1"}
	server2 := &models.Gameserver{ID: "test2", Name: "server2"}

	mock.CreateContainer(server1)
	mock.CreateContainer(server2)

	containers, err = mock.ListContainers()
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	if len(containers) != 2 {
		t.Errorf("Expected 2 containers, got %d", len(containers))
	}
}

func TestContainerPortMappings(t *testing.T) {
	server := &models.Gameserver{
		ID:       "port-test",
		Name:     "port-server",
		GameID:   "minecraft",
		Image:    "minecraft:latest",
		MemoryMB: 1024,
		PortMappings: []models.PortMapping{
			{Name: "game", Protocol: "tcp", ContainerPort: 25565, HostPort: 25565},
			{Name: "query", Protocol: "udp", ContainerPort: 25565, HostPort: 25566},
		},
	}

	mock := NewMockDockerManager()
	err := mock.CreateContainer(server)
	if err != nil {
		t.Fatalf("Failed to create container: %v", err)
	}

	// Verify port mappings are preserved
	if len(server.PortMappings) != 2 {
		t.Errorf("Expected 2 port mappings, got %d", len(server.PortMappings))
	}

	// Check specific port mapping
	tcpMapping := server.PortMappings[0]
	if tcpMapping.Protocol != "tcp" || tcpMapping.ContainerPort != 25565 || tcpMapping.HostPort != 25565 {
		t.Errorf("TCP port mapping incorrect: %+v", tcpMapping)
	}
}

func TestContainerResourceConstraints(t *testing.T) {
	server := &models.Gameserver{
		ID:       "resource-test",
		Name:     "resource-server",
		GameID:   "minecraft",
		Image:    "minecraft:latest",
		MemoryMB: 2048,
		CPUCores: 1.5,
	}

	mock := NewMockDockerManager()
	err := mock.CreateContainer(server)
	if err != nil {
		t.Fatalf("Failed to create container: %v", err)
	}

	// Verify resource constraints are preserved in the gameserver
	if server.MemoryMB != 2048 {
		t.Errorf("Expected memory 2048MB, got %d", server.MemoryMB)
	}
	if server.CPUCores != 1.5 {
		t.Errorf("Expected CPU cores 1.5, got %f", server.CPUCores)
	}
}

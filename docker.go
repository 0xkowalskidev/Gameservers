package main

import (
	"context"
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/client"
	"github.com/docker/go-connections/nat"
)

// =============================================================================
// Docker Error Types
// =============================================================================

type DockerError struct {
	Operation string
	Message   string
	Err       error
}

func (e *DockerError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("docker %s failed: %s: %v", e.Operation, e.Message, e.Err)
	}
	return fmt.Sprintf("docker %s failed: %s", e.Operation, e.Message)
}

// =============================================================================
// Docker Manager Implementation
// =============================================================================

type DockerManager struct {
	client *client.Client
}

func NewDockerManager() (*DockerManager, error) {
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return nil, &DockerError{
			Operation: "connect",
			Message:   "failed to create Docker client",
			Err:       err,
		}
	}
	return &DockerManager{client: cli}, nil
}

func (d *DockerManager) CreateContainer(server *GameServer) error {
	ctx := context.Background()

	// Convert port to nat.Port
	exposedPort := nat.Port(fmt.Sprintf("%d/tcp", server.Port))
	
	// Container configuration
	config := &container.Config{
		Image: server.Image,
		Env:   server.Environment,
		ExposedPorts: nat.PortSet{
			exposedPort: struct{}{},
		},
		Labels: map[string]string{
			"gameserver.id":   server.ID,
			"gameserver.name": server.Name,
			"gameserver.type": server.GameType,
		},
	}

	// Host configuration
	hostConfig := &container.HostConfig{
		PortBindings: nat.PortMap{
			exposedPort: []nat.PortBinding{
				{
					HostIP:   "0.0.0.0",
					HostPort: strconv.Itoa(server.Port),
				},
			},
		},
		RestartPolicy: container.RestartPolicy{
			Name: "unless-stopped",
		},
	}

	// Add volumes if specified
	if len(server.Volumes) > 0 {
		hostConfig.Binds = server.Volumes
	}

	// Network configuration
	networkConfig := &network.NetworkingConfig{}

	// Create container
	// TODO: Make namespace configurable via config file/env var
	containerName := fmt.Sprintf("gameservers-%s", server.Name)
	resp, err := d.client.ContainerCreate(
		ctx,
		config,
		hostConfig,
		networkConfig,
		nil,
		containerName,
	)
	if err != nil {
		return &DockerError{
			Operation: "create",
			Message:   fmt.Sprintf("failed to create container for server %s", server.Name),
			Err:       err,
		}
	}

	server.ContainerID = resp.ID
	server.Status = StatusStopped
	server.UpdatedAt = time.Now()

	return nil
}

func (d *DockerManager) StartContainer(containerID string) error {
	ctx := context.Background()

	err := d.client.ContainerStart(ctx, containerID, container.StartOptions{})
	if err != nil {
		return &DockerError{
			Operation: "start",
			Message:   fmt.Sprintf("failed to start container %s", containerID),
			Err:       err,
		}
	}

	return nil
}

func (d *DockerManager) StopContainer(containerID string) error {
	ctx := context.Background()

	timeout := 30 // seconds
	err := d.client.ContainerStop(ctx, containerID, container.StopOptions{
		Timeout: &timeout,
	})
	if err != nil {
		return &DockerError{
			Operation: "stop",
			Message:   fmt.Sprintf("failed to stop container %s", containerID),
			Err:       err,
		}
	}

	return nil
}

func (d *DockerManager) RestartContainer(containerID string) error {
	ctx := context.Background()

	timeout := 30 // seconds
	err := d.client.ContainerRestart(ctx, containerID, container.StopOptions{
		Timeout: &timeout,
	})
	if err != nil {
		return &DockerError{
			Operation: "restart",
			Message:   fmt.Sprintf("failed to restart container %s", containerID),
			Err:       err,
		}
	}

	return nil
}

func (d *DockerManager) RemoveContainer(containerID string) error {
	ctx := context.Background()

	err := d.client.ContainerRemove(ctx, containerID, container.RemoveOptions{
		Force: true,
	})
	if err != nil {
		return &DockerError{
			Operation: "remove",
			Message:   fmt.Sprintf("failed to remove container %s", containerID),
			Err:       err,
		}
	}

	return nil
}

func (d *DockerManager) GetContainerStatus(containerID string) (GameServerStatus, error) {
	ctx := context.Background()

	inspect, err := d.client.ContainerInspect(ctx, containerID)
	if err != nil {
		return StatusError, &DockerError{
			Operation: "status",
			Message:   fmt.Sprintf("failed to inspect container %s", containerID),
			Err:       err,
		}
	}

	// Map Docker states to our GameServerStatus
	switch inspect.State.Status {
	case "running":
		return StatusRunning, nil
	case "exited", "dead":
		return StatusStopped, nil
	case "created":
		return StatusStopped, nil
	case "restarting":
		return StatusStarting, nil
	case "paused":
		return StatusStopped, nil
	default:
		return StatusError, nil
	}
}

func (d *DockerManager) GetContainerStats(containerID string) (*ContainerStats, error) {
	// TODO: Implement proper stats collection
	// For now, return mock stats to satisfy the interface
	return &ContainerStats{
		CPUPercent:    25.5,
		MemoryUsage:   1024 * 1024 * 512, // 512MB
		MemoryLimit:   1024 * 1024 * 1024, // 1GB
		MemoryPercent: 50.0,
		NetworkRx:     1024 * 100,
		NetworkTx:     1024 * 200,
	}, nil
}

func (d *DockerManager) GetContainerLogs(containerID string, lines int) ([]string, error) {
	ctx := context.Background()

	options := container.LogsOptions{
		ShowStdout: true,
		ShowStderr: true,
		Tail:       strconv.Itoa(lines),
	}

	logs, err := d.client.ContainerLogs(ctx, containerID, options)
	if err != nil {
		return nil, &DockerError{
			Operation: "logs",
			Message:   fmt.Sprintf("failed to get logs for container %s", containerID),
			Err:       err,
		}
	}
	defer logs.Close()

	// Read logs
	logData, err := io.ReadAll(logs)
	if err != nil {
		return nil, &DockerError{
			Operation: "logs",
			Message:   "failed to read logs response",
			Err:       err,
		}
	}

	// Split logs into lines and clean them
	logLines := strings.Split(string(logData), "\n")
	var cleanLines []string
	for _, line := range logLines {
		// Docker logs include headers, we need to strip them
		if len(line) > 8 {
			cleanLine := line[8:] // Remove Docker log header
			if strings.TrimSpace(cleanLine) != "" {
				cleanLines = append(cleanLines, cleanLine)
			}
		}
	}

	return cleanLines, nil
}

func (d *DockerManager) ListContainers() ([]string, error) {
	ctx := context.Background()

	filterArgs := filters.NewArgs()
	filterArgs.Add("label", "gameserver.id")
	
	containers, err := d.client.ContainerList(ctx, container.ListOptions{
		All: true,
		Filters: filterArgs,
	})
	if err != nil {
		return nil, &DockerError{
			Operation: "list",
			Message:   "failed to list containers",
			Err:       err,
		}
	}

	var containerIDs []string
	for _, container := range containers {
		containerIDs = append(containerIDs, container.ID)
	}

	return containerIDs, nil
}

// =============================================================================
// Helper Functions
// =============================================================================
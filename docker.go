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
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/client"
	"github.com/docker/go-connections/nat"
	"github.com/rs/zerolog/log"
)

// =============================================================================
// Docker Error Types
// =============================================================================

type DockerError struct {
	Op  string
	Msg string
	Err error
}

func (e *DockerError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("docker %s: %s: %v", e.Op, e.Msg, e.Err)
	}
	return fmt.Sprintf("docker %s: %s", e.Op, e.Msg)
}

// =============================================================================
// Docker Manager Implementation
// =============================================================================

type DockerManager struct {
	client *client.Client
}

func NewDockerManager() (*DockerManager, error) {
	log.Info().Msg("Connecting to Docker daemon")
	
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		log.Error().Err(err).Msg("Failed to create Docker client")
		return nil, &DockerError{
			Op: "connect",
			Msg:   "failed to create Docker client",
			Err:       err,
		}
	}
	
	log.Info().Msg("Docker client connected successfully")
	return &DockerManager{client: cli}, nil
}

func (d *DockerManager) CreateContainer(server *Gameserver) error {
	ctx := context.Background()
	log.Info().Str("gameserver_id", server.ID).Str("name", server.Name).Str("image", server.Image).Msg("Creating Docker container")

	// Try to pull image if it doesn't exist locally
	if err := d.pullImageIfNeeded(ctx, server.Image); err != nil {
		log.Warn().Err(err).Str("image", server.Image).Msg("Failed to pull Docker image, proceeding anyway")
	}

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
		log.Error().Err(err).Str("gameserver_id", server.ID).Str("name", server.Name).Msg("Failed to create Docker container")
		return &DockerError{
			Op: "create",
			Msg:   fmt.Sprintf("failed to create container for server %s", server.Name),
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
			Op: "start",
			Msg:   fmt.Sprintf("failed to start container %s", containerID),
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
			Op: "stop",
			Msg:   fmt.Sprintf("failed to stop container %s", containerID),
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
			Op: "restart",
			Msg:   fmt.Sprintf("failed to restart container %s", containerID),
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
			Op: "remove",
			Msg:   fmt.Sprintf("failed to remove container %s", containerID),
			Err:       err,
		}
	}

	return nil
}

func (d *DockerManager) GetContainerStatus(containerID string) (GameserverStatus, error) {
	ctx := context.Background()

	inspect, err := d.client.ContainerInspect(ctx, containerID)
	if err != nil {
		return StatusError, &DockerError{
			Op: "status",
			Msg:   fmt.Sprintf("failed to inspect container %s", containerID),
			Err:       err,
		}
	}

	// Map Docker states to our GameserverStatus
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
			Op: "logs",
			Msg:   fmt.Sprintf("failed to get logs for container %s", containerID),
			Err:       err,
		}
	}
	defer logs.Close()

	// Read logs
	logData, err := io.ReadAll(logs)
	if err != nil {
		return nil, &DockerError{
			Op: "logs",
			Msg:   "failed to read logs response",
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

func (d *DockerManager) StreamContainerLogs(containerID string) (io.ReadCloser, error) {
	ctx := context.Background()

	options := container.LogsOptions{
		ShowStdout: true,
		ShowStderr: true,
		Follow:     true,
		Tail:       "100",
		Timestamps: true,
	}

	logs, err := d.client.ContainerLogs(ctx, containerID, options)
	if err != nil {
		return nil, &DockerError{
			Op: "stream_logs",
			Msg:   fmt.Sprintf("failed to stream logs for container %s", containerID),
			Err:       err,
		}
	}

	return logs, nil
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
			Op: "list",
			Msg:   "failed to list containers",
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

func (d *DockerManager) pullImageIfNeeded(ctx context.Context, imageName string) error {
	// Check if image exists locally
	_, _, err := d.client.ImageInspectWithRaw(ctx, imageName)
	if err == nil {
		// Image exists locally, no need to pull
		log.Debug().Str("image", imageName).Msg("Image exists locally")
		return nil
	}

	// Image doesn't exist, try to pull it
	log.Info().Str("image", imageName).Msg("Pulling Docker image")
	
	// Use empty struct as options - this should work with most Docker API versions
	reader, err := d.client.ImagePull(ctx, imageName, image.PullOptions{})
	if err != nil {
		return err
	}
	defer reader.Close()

	// Read the pull response to completion (required for the pull to actually happen)
	buf := make([]byte, 1024)
	for {
		_, err := reader.Read(buf)
		if err != nil {
			if err == io.EOF {
				break
			}
			return err
		}
	}

	log.Info().Str("image", imageName).Msg("Successfully pulled Docker image")
	return nil
}
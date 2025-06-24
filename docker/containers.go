package docker

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/go-connections/nat"
	"github.com/rs/zerolog/log"

	"0xkowalskidev/gameservers/models"
)

// CreateContainer creates a new Docker container for a gameserver
func (d *DockerManager) CreateContainer(server *models.Gameserver) error {
	ctx := context.Background()
	log.Info().Str("gameserver_id", server.ID).Str("name", server.Name).Str("image", server.Image).Msg("Creating Docker container")

	// Try to pull image if it doesn't exist locally
	if err := d.pullImageIfNeeded(ctx, server.Image); err != nil {
		log.Warn().Err(err).Str("image", server.Image).Msg("Failed to pull Docker image, proceeding anyway")
	}

	// Prepare environment variables with automatic resource settings
	env := make([]string, len(server.Environment))
	copy(env, server.Environment)

	// Automatically set MEMORY_MB for images that need it
	if server.MemoryMB > 0 {
		env = append(env, fmt.Sprintf("MEMORY_MB=%d", server.MemoryMB))
	}

	// Set up port mappings
	exposedPorts := make(nat.PortSet)
	for _, portMapping := range server.PortMappings {
		port := nat.Port(fmt.Sprintf("%d/%s", portMapping.ContainerPort, portMapping.Protocol))
		exposedPorts[port] = struct{}{}
	}

	// Container configuration
	config := &container.Config{
		Image:        server.Image,
		Env:          env,
		ExposedPorts: exposedPorts,
		Labels: map[string]string{
			"gameserver.id":   server.ID,
			"gameserver.name": server.Name,
			"gameserver.type": server.GameType,
		},
	}

	// Set up port bindings using pre-assigned ports
	portBindings := make(nat.PortMap)
	for _, portMapping := range server.PortMappings {
		port := nat.Port(fmt.Sprintf("%d/%s", portMapping.ContainerPort, portMapping.Protocol))
		if portMapping.HostPort == 0 {
			return &DockerError{
				Op:  "create",
				Msg: fmt.Sprintf("port mapping for %s:%d has no assigned host port", portMapping.Protocol, portMapping.ContainerPort),
				Err: nil,
			}
		}
		hostPort := strconv.Itoa(portMapping.HostPort)
		portBindings[port] = []nat.PortBinding{
			{
				HostIP:   "0.0.0.0",
				HostPort: hostPort,
			},
		}
	}

	// Host configuration with resource constraints
	hostConfig := &container.HostConfig{
		PortBindings: portBindings,
		RestartPolicy: container.RestartPolicy{
			Name: "unless-stopped",
		},
	}

	// Apply memory constraint (always required)
	hostConfig.Memory = int64(server.MemoryMB) * 1024 * 1024 // Convert MB to bytes

	// Apply CPU constraint (optional - 0 means unlimited)
	if server.CPUCores > 0 {
		// Convert CPU cores to Docker's quota/period system
		// 1 core = 100000 quota with 100000 period
		hostConfig.CPUQuota = int64(server.CPUCores * 100000)
		hostConfig.CPUPeriod = 100000
	}

	// Create and mount auto-managed volume for data persistence
	volumeName := d.getVolumeNameForServer(server)
	if err := d.CreateVolume(volumeName); err != nil {
		log.Error().Err(err).Str("volume", volumeName).Msg("Failed to create volume")
		return err
	}

	// Mount the volume to /data in the container (standard gameserver path)
	hostConfig.Binds = []string{
		fmt.Sprintf("%s:/data", volumeName),
	}

	// Add any additional volumes if specified
	if len(server.Volumes) > 0 {
		hostConfig.Binds = append(hostConfig.Binds, server.Volumes...)
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
			Op:  "create",
			Msg: fmt.Sprintf("failed to create container for server %s", server.Name),
			Err: err,
		}
	}

	server.ContainerID = resp.ID
	server.Status = models.StatusStopped
	server.UpdatedAt = time.Now()

	log.Info().
		Str("gameserver_id", server.ID).
		Str("container_id", server.ContainerID).
		Interface("port_mappings", server.PortMappings).
		Msg("Container created successfully with pre-assigned ports")

	return nil
}

// StartContainer starts a Docker container
func (d *DockerManager) StartContainer(containerID string) error {
	ctx := context.Background()

	err := d.client.ContainerStart(ctx, containerID, container.StartOptions{})
	if err != nil {
		return &DockerError{
			Op:  "start",
			Msg: fmt.Sprintf("failed to start container %s", containerID),
			Err: err,
		}
	}

	return nil
}

// StopContainer stops a Docker container
func (d *DockerManager) StopContainer(containerID string) error {
	ctx := context.Background()

	timeout := 30 // 30 seconds timeout
	err := d.client.ContainerStop(ctx, containerID, container.StopOptions{
		Timeout: &timeout,
	})
	if err != nil {
		return &DockerError{
			Op:  "stop",
			Msg: fmt.Sprintf("failed to stop container %s", containerID),
			Err: err,
		}
	}

	return nil
}

// RemoveContainer removes a Docker container
func (d *DockerManager) RemoveContainer(containerID string) error {
	ctx := context.Background()

	err := d.client.ContainerRemove(ctx, containerID, container.RemoveOptions{
		Force: true,
	})
	if err != nil {
		return &DockerError{
			Op:  "remove",
			Msg: fmt.Sprintf("failed to remove container %s", containerID),
			Err: err,
		}
	}

	return nil
}

// GetContainerStatus returns the status of a container
func (d *DockerManager) GetContainerStatus(containerID string) (models.GameserverStatus, error) {
	ctx := context.Background()

	inspect, err := d.client.ContainerInspect(ctx, containerID)
	if err != nil {
		return models.StatusError, &DockerError{
			Op:  "inspect",
			Msg: fmt.Sprintf("failed to inspect container %s", containerID),
			Err: err,
		}
	}

	switch inspect.State.Status {
	case "running":
		return models.StatusRunning, nil
	case "exited":
		return models.StatusStopped, nil
	case "created":
		return models.StatusStopped, nil
	case "restarting":
		return models.StatusStarting, nil
	default:
		return models.StatusError, nil
	}
}

// ListContainers returns a list of all gameserver containers
func (d *DockerManager) ListContainers() ([]string, error) {
	ctx := context.Background()

	filter := filters.NewArgs()
	filter.Add("label", "gameserver.id")

	containers, err := d.client.ContainerList(ctx, container.ListOptions{
		All:     true,
		Filters: filter,
	})
	if err != nil {
		return nil, &DockerError{
			Op:  "list",
			Msg: "failed to list containers",
			Err: err,
		}
	}

	var result []string
	for _, c := range containers {
		result = append(result, c.ID)
	}

	return result, nil
}

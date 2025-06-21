package main

import (
	"context"
	"fmt"
	"io"
	"strconv"
	"time"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/api/types/volume"
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
	
	// Prepare environment variables with automatic resource settings
	env := make([]string, len(server.Environment))
	copy(env, server.Environment)
	
	// Automatically set MEMORY_MB for images that need it
	if server.MemoryMB > 0 {
		env = append(env, fmt.Sprintf("MEMORY_MB=%d", server.MemoryMB))
	}
	
	// Container configuration
	config := &container.Config{
		Image: server.Image,
		Env:   env,
		ExposedPorts: nat.PortSet{
			exposedPort: struct{}{},
		},
		Labels: map[string]string{
			"gameserver.id":   server.ID,
			"gameserver.name": server.Name,
			"gameserver.type": server.GameType,
		},
	}

	// Host configuration with resource constraints
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

func (d *DockerManager) StreamContainerStats(containerID string) (io.ReadCloser, error) {
	ctx := context.Background()

	stats, err := d.client.ContainerStats(ctx, containerID, true)
	if err != nil {
		return nil, &DockerError{
			Op:  "stream_stats",
			Msg: fmt.Sprintf("failed to stream stats for container %s", containerID),
			Err: err,
		}
	}

	return stats.Body, nil
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

// =============================================================================
// Volume Management
// =============================================================================

func (d *DockerManager) CreateVolume(volumeName string) error {
	ctx := context.Background()
	
	// Check if volume already exists
	_, err := d.client.VolumeInspect(ctx, volumeName)
	if err == nil {
		// Volume already exists, no need to create
		log.Debug().Str("volume", volumeName).Msg("Volume already exists")
		return nil
	}
	
	log.Info().Str("volume", volumeName).Msg("Creating Docker volume")
	
	_, err = d.client.VolumeCreate(ctx, volume.CreateOptions{
		Name: volumeName,
		Labels: map[string]string{
			"gameserver.managed": "true",
		},
	})
	if err != nil {
		return &DockerError{
			Op:  "create_volume",
			Msg: fmt.Sprintf("failed to create volume %s", volumeName),
			Err: err,
		}
	}
	
	log.Info().Str("volume", volumeName).Msg("Successfully created Docker volume")
	return nil
}

func (d *DockerManager) RemoveVolume(volumeName string) error {
	ctx := context.Background()
	
	log.Info().Str("volume", volumeName).Msg("Removing Docker volume")
	
	err := d.client.VolumeRemove(ctx, volumeName, true) // force=true
	if err != nil {
		return &DockerError{
			Op:  "remove_volume",
			Msg: fmt.Sprintf("failed to remove volume %s", volumeName),
			Err: err,
		}
	}
	
	log.Info().Str("volume", volumeName).Msg("Successfully removed Docker volume")
	return nil
}

func (d *DockerManager) getVolumeNameForServer(server *Gameserver) string {
	return fmt.Sprintf("gameservers-%s-data", server.Name)
}

func (d *DockerManager) GetVolumeInfo(volumeName string) (*VolumeInfo, error) {
	ctx := context.Background()
	
	vol, err := d.client.VolumeInspect(ctx, volumeName)
	if err != nil {
		return nil, &DockerError{
			Op:  "inspect_volume",
			Msg: fmt.Sprintf("failed to inspect volume %s", volumeName),
			Err: err,
		}
	}
	
	return &VolumeInfo{
		Name:       vol.Name,
		MountPoint: vol.Mountpoint,
		Driver:     vol.Driver,
		CreatedAt:  vol.CreatedAt,
		Labels:     vol.Labels,
	}, nil
}

// =============================================================================
// Backup and Restore Operations
// =============================================================================

func (d *DockerManager) CreateBackup(gameserverID, backupPath string) error {
	ctx := context.Background()
	
	// Get the volume name for the gameserver
	// Assume gameserverID maps to server name for volume naming
	volumeName := fmt.Sprintf("gameservers-%s-data", gameserverID)
	
	log.Info().Str("volume", volumeName).Str("backup_path", backupPath).Msg("Creating backup")
	
	// Create a temporary container to access the volume
	config := &container.Config{
		Image: "alpine:latest", // Use lightweight alpine for backup operations
		Cmd:   []string{"tar", "-czf", "/backup.tar.gz", "-C", "/data", "."},
		WorkingDir: "/",
	}
	
	hostConfig := &container.HostConfig{
		Binds: []string{
			fmt.Sprintf("%s:/data", volumeName),
			fmt.Sprintf("%s:/backup.tar.gz", backupPath),
		},
		AutoRemove: true,
	}
	
	// Create container
	resp, err := d.client.ContainerCreate(ctx, config, hostConfig, nil, nil, "")
	if err != nil {
		return &DockerError{
			Op:  "create_backup_container",
			Msg: fmt.Sprintf("failed to create backup container for %s", gameserverID),
			Err: err,
		}
	}
	
	// Start container and wait for completion
	if err := d.client.ContainerStart(ctx, resp.ID, container.StartOptions{}); err != nil {
		return &DockerError{
			Op:  "start_backup_container",
			Msg: fmt.Sprintf("failed to start backup container for %s", gameserverID),
			Err: err,
		}
	}
	
	// Wait for container to finish
	statusCh, errCh := d.client.ContainerWait(ctx, resp.ID, container.WaitConditionNotRunning)
	select {
	case err := <-errCh:
		if err != nil {
			return &DockerError{
				Op:  "wait_backup_container",
				Msg: fmt.Sprintf("failed to wait for backup container for %s", gameserverID),
				Err: err,
			}
		}
	case status := <-statusCh:
		if status.StatusCode != 0 {
			return &DockerError{
				Op:  "backup_container_failed",
				Msg: fmt.Sprintf("backup container exited with status %d for %s", status.StatusCode, gameserverID),
				Err: nil,
			}
		}
	}
	
	log.Info().Str("gameserver_id", gameserverID).Str("backup_path", backupPath).Msg("Backup created successfully")
	return nil
}

func (d *DockerManager) RestoreBackup(gameserverID, backupPath string) error {
	ctx := context.Background()
	
	// Get the volume name for the gameserver
	volumeName := fmt.Sprintf("gameservers-%s-data", gameserverID)
	
	log.Info().Str("volume", volumeName).Str("backup_path", backupPath).Msg("Restoring backup")
	
	// Create a temporary container to restore the volume
	config := &container.Config{
		Image: "alpine:latest",
		Cmd:   []string{"sh", "-c", "cd /data && tar -xzf /backup.tar.gz"},
		WorkingDir: "/",
	}
	
	hostConfig := &container.HostConfig{
		Binds: []string{
			fmt.Sprintf("%s:/data", volumeName),
			fmt.Sprintf("%s:/backup.tar.gz", backupPath),
		},
		AutoRemove: true,
	}
	
	// Create container
	resp, err := d.client.ContainerCreate(ctx, config, hostConfig, nil, nil, "")
	if err != nil {
		return &DockerError{
			Op:  "create_restore_container",
			Msg: fmt.Sprintf("failed to create restore container for %s", gameserverID),
			Err: err,
		}
	}
	
	// Start container and wait for completion
	if err := d.client.ContainerStart(ctx, resp.ID, container.StartOptions{}); err != nil {
		return &DockerError{
			Op:  "start_restore_container",
			Msg: fmt.Sprintf("failed to start restore container for %s", gameserverID),
			Err: err,
		}
	}
	
	// Wait for container to finish
	statusCh, errCh := d.client.ContainerWait(ctx, resp.ID, container.WaitConditionNotRunning)
	select {
	case err := <-errCh:
		if err != nil {
			return &DockerError{
				Op:  "wait_restore_container",
				Msg: fmt.Sprintf("failed to wait for restore container for %s", gameserverID),
				Err: err,
			}
		}
	case status := <-statusCh:
		if status.StatusCode != 0 {
			return &DockerError{
				Op:  "restore_container_failed",
				Msg: fmt.Sprintf("restore container exited with status %d for %s", status.StatusCode, gameserverID),
				Err: nil,
			}
		}
	}
	
	log.Info().Str("gameserver_id", gameserverID).Str("backup_path", backupPath).Msg("Backup restored successfully")
	return nil
}
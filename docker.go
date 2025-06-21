package main

import (
	"archive/tar"
	"bytes"
	"context"
	"fmt"
	"io"
	"path/filepath"
	"strconv"
	"strings"
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
	
	// Pull alpine image if needed
	if err := d.pullImageIfNeeded(ctx, "alpine:latest"); err != nil {
		return &DockerError{
			Op:  "pull_alpine_for_backup",
			Msg: fmt.Sprintf("failed to pull alpine image for backup of %s", gameserverID),
			Err: err,
		}
	}
	
	// Extract directory and filename from backup path
	backupDir := filepath.Dir(backupPath)
	backupFile := filepath.Base(backupPath)
	
	// Create a temporary container to access the volume
	config := &container.Config{
		Image: "alpine:latest", // Use lightweight alpine for backup operations
		Cmd:   []string{"tar", "-czf", fmt.Sprintf("/backup/%s", backupFile), "-C", "/data/server", "."},
		WorkingDir: "/",
	}
	
	hostConfig := &container.HostConfig{
		Binds: []string{
			fmt.Sprintf("%s:/data", volumeName),
			fmt.Sprintf("%s:/backup", backupDir), // Mount the backup directory, not the file
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
	
	// Ensure cleanup of container (belt and suspenders approach with AutoRemove)
	defer func() {
		if err := d.client.ContainerRemove(ctx, resp.ID, container.RemoveOptions{Force: true}); err != nil {
			// Only log as debug if it's the expected "removal already in progress" error
			if strings.Contains(err.Error(), "removal of container") && strings.Contains(err.Error(), "is already in progress") {
				log.Debug().Str("container_id", resp.ID).Msg("Backup container already being removed by AutoRemove")
			} else {
				log.Warn().Err(err).Str("container_id", resp.ID).Msg("Failed to cleanup backup container")
			}
		}
	}()
	
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
			// Get container logs for debugging
			logMsg := "no logs available"
			if logs, err := d.client.ContainerLogs(ctx, resp.ID, container.LogsOptions{
				ShowStdout: true,
				ShowStderr: true,
			}); err == nil && logs != nil {
				defer logs.Close()
				if logBytes, err := io.ReadAll(logs); err == nil {
					logMsg = string(logBytes)
				}
			}
			
			log.Error().Str("container_logs", logMsg).Int64("exit_code", status.StatusCode).Str("gameserver_id", gameserverID).Msg("Backup container failed")
			
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
	
	// Pull alpine image if needed
	if err := d.pullImageIfNeeded(ctx, "alpine:latest"); err != nil {
		return &DockerError{
			Op:  "pull_alpine_for_restore",
			Msg: fmt.Sprintf("failed to pull alpine image for restore of %s", gameserverID),
			Err: err,
		}
	}
	
	// Extract directory and filename from backup path
	backupDir := filepath.Dir(backupPath)
	backupFile := filepath.Base(backupPath)
	
	// Create a temporary container to restore the volume
	config := &container.Config{
		Image: "alpine:latest",
		Cmd:   []string{"sh", "-c", fmt.Sprintf("cd /data/server && tar -xzf /backup/%s", backupFile)},
		WorkingDir: "/",
	}
	
	hostConfig := &container.HostConfig{
		Binds: []string{
			fmt.Sprintf("%s:/data", volumeName),
			fmt.Sprintf("%s:/backup", backupDir), // Mount the backup directory, not the file
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
	
	// Ensure cleanup of container (belt and suspenders approach with AutoRemove)
	defer func() {
		if err := d.client.ContainerRemove(ctx, resp.ID, container.RemoveOptions{Force: true}); err != nil {
			// Only log as debug if it's the expected "removal already in progress" error
			if strings.Contains(err.Error(), "removal of container") && strings.Contains(err.Error(), "is already in progress") {
				log.Debug().Str("container_id", resp.ID).Msg("Restore container already being removed by AutoRemove")
			} else {
				log.Warn().Err(err).Str("container_id", resp.ID).Msg("Failed to cleanup restore container")
			}
		}
	}()
	
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
			// Get container logs for debugging
			logMsg := "no logs available"
			if logs, err := d.client.ContainerLogs(ctx, resp.ID, container.LogsOptions{
				ShowStdout: true,
				ShowStderr: true,
			}); err == nil && logs != nil {
				defer logs.Close()
				if logBytes, err := io.ReadAll(logs); err == nil {
					logMsg = string(logBytes)
				}
			}
			
			log.Error().Str("container_logs", logMsg).Int64("exit_code", status.StatusCode).Str("gameserver_id", gameserverID).Msg("Restore container failed")
			
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

// =============================================================================
// File Operations
// =============================================================================

type FileInfo struct {
	Name     string    `json:"name"`
	Path     string    `json:"path"`
	IsDir    bool      `json:"isDir"`
	Size     int64     `json:"size"`
	Mode     string    `json:"mode"`
	Modified time.Time `json:"modified"`
	Owner    string    `json:"owner"`
	Group    string    `json:"group"`
}

func (d *DockerManager) ExecCommand(containerID string, cmd []string) (string, error) {
	ctx := context.Background()
	
	execConfig := container.ExecOptions{
		Cmd:          cmd,
		AttachStdout: true,
		AttachStderr: true,
	}
	
	// Create exec instance
	execID, err := d.client.ContainerExecCreate(ctx, containerID, execConfig)
	if err != nil {
		return "", &DockerError{
			Op:  "exec_create",
			Msg: fmt.Sprintf("failed to create exec for container %s", containerID),
			Err: err,
		}
	}
	
	// Attach to the exec instance
	resp, err := d.client.ContainerExecAttach(ctx, execID.ID, container.ExecAttachOptions{})
	if err != nil {
		return "", &DockerError{
			Op:  "exec_attach",
			Msg: fmt.Sprintf("failed to attach to exec for container %s", containerID),
			Err: err,
		}
	}
	defer resp.Close()
	
	// Start the exec instance
	err = d.client.ContainerExecStart(ctx, execID.ID, container.ExecStartOptions{})
	if err != nil {
		return "", &DockerError{
			Op:  "exec_start",
			Msg: fmt.Sprintf("failed to start exec for container %s", containerID),
			Err: err,
		}
	}
	
	// Read output
	output, err := io.ReadAll(resp.Reader)
	if err != nil {
		return "", &DockerError{
			Op:  "exec_read",
			Msg: fmt.Sprintf("failed to read exec output for container %s", containerID),
			Err: err,
		}
	}
	
	// Check exec exit code
	inspectResp, err := d.client.ContainerExecInspect(ctx, execID.ID)
	if err != nil {
		return "", &DockerError{
			Op:  "exec_inspect",
			Msg: fmt.Sprintf("failed to inspect exec for container %s", containerID),
			Err: err,
		}
	}
	
	if inspectResp.ExitCode != 0 {
		return "", &DockerError{
			Op:  "exec_failed",
			Msg: fmt.Sprintf("command failed with exit code %d: %s", inspectResp.ExitCode, string(output)),
			Err: nil,
		}
	}
	
	return string(output), nil
}

func (d *DockerManager) ListFiles(containerID string, path string) ([]*FileInfo, error) {
	// Ensure path starts with /data/server (user root)
	if path == "" || path == "/" {
		path = "/data/server"
	}
	if !strings.HasPrefix(path, "/data/server") {
		path = "/data/server"
	}
	
	// Use simple ls -la command
	cmd := []string{"ls", "-la", path}
	
	output, err := d.ExecCommand(containerID, cmd)
	if err != nil {
		return nil, err
	}
	
	// Parse ls output and sort
	return sortFiles(parseLsOutput(output, path)), nil
}

func (d *DockerManager) ReadFile(containerID string, path string) ([]byte, error) {
	// Ensure path is within /data/server
	if !strings.HasPrefix(path, "/data/server") {
		return nil, &DockerError{
			Op:  "read_file",
			Msg: "access denied: path must be within /data/server directory",
			Err: nil,
		}
	}
	
	// Use cat to read file contents
	cmd := []string{"cat", path}
	
	output, err := d.ExecCommand(containerID, cmd)
	if err != nil {
		return nil, err
	}
	
	// Clean the output to remove any Docker control characters
	cleanOutput := cleanDockerOutput(output)
	
	return []byte(cleanOutput), nil
}

func (d *DockerManager) WriteFile(containerID string, path string, content []byte) error {
	// Ensure path is within /data/server
	if !strings.HasPrefix(path, "/data/server") {
		return &DockerError{
			Op:  "write_file",
			Msg: "access denied: path must be within /data/server directory",
			Err: nil,
		}
	}
	
	ctx := context.Background()
	
	// Create a tar archive with the file
	tarContent, err := createTarArchive(filepath.Base(path), content)
	if err != nil {
		return &DockerError{
			Op:  "create_tar",
			Msg: fmt.Sprintf("failed to create tar archive for file %s", path),
			Err: err,
		}
	}
	
	// Copy to container
	err = d.client.CopyToContainer(ctx, containerID, filepath.Dir(path), tarContent, container.CopyToContainerOptions{})
	if err != nil {
		return &DockerError{
			Op:  "copy_to_container",
			Msg: fmt.Sprintf("failed to copy file to container %s", containerID),
			Err: err,
		}
	}
	
	return nil
}

func (d *DockerManager) CreateDirectory(containerID string, path string) error {
	// Ensure path is within /data/server
	if !strings.HasPrefix(path, "/data/server") {
		return &DockerError{
			Op:  "create_directory",
			Msg: "access denied: path must be within /data/server directory",
			Err: nil,
		}
	}
	
	cmd := []string{"mkdir", "-p", path}
	_, err := d.ExecCommand(containerID, cmd)
	return err
}

func (d *DockerManager) DeletePath(containerID string, path string) error {
	// Ensure path is within /data/server
	if !strings.HasPrefix(path, "/data/server") {
		return &DockerError{
			Op:  "delete_path",
			Msg: "access denied: path must be within /data/server directory",
			Err: nil,
		}
	}
	
	// Don't allow deleting /data/server itself
	if path == "/data/server" {
		return &DockerError{
			Op:  "delete_path",
			Msg: "cannot delete server root directory",
			Err: nil,
		}
	}
	
	cmd := []string{"rm", "-rf", path}
	_, err := d.ExecCommand(containerID, cmd)
	return err
}

func (d *DockerManager) DownloadFile(containerID string, path string) (io.ReadCloser, error) {
	// Ensure path is within /data/server
	if !strings.HasPrefix(path, "/data/server") {
		return nil, &DockerError{
			Op:  "download_file",
			Msg: "access denied: path must be within /data/server directory",
			Err: nil,
		}
	}
	
	ctx := context.Background()
	
	reader, _, err := d.client.CopyFromContainer(ctx, containerID, path)
	if err != nil {
		return nil, &DockerError{
			Op:  "copy_from_container",
			Msg: fmt.Sprintf("failed to copy file from container %s", containerID),
			Err: err,
		}
	}
	
	return reader, nil
}

func (d *DockerManager) UploadFile(containerID string, destPath string, reader io.Reader) error {
	// Ensure path is within /data/server
	if !strings.HasPrefix(destPath, "/data/server") {
		return &DockerError{
			Op:  "upload_file",
			Msg: "access denied: path must be within /data/server directory",
			Err: nil,
		}
	}
	
	ctx := context.Background()
	
	// Copy to container
	err := d.client.CopyToContainer(ctx, containerID, destPath, reader, container.CopyToContainerOptions{})
	if err != nil {
		return &DockerError{
			Op:  "upload_file",
			Msg: fmt.Sprintf("failed to upload file to container %s", containerID),
			Err: err,
		}
	}
	
	return nil
}

func (d *DockerManager) RenameFile(containerID string, oldPath string, newPath string) error {
	// Ensure both paths are within /data/server
	if !strings.HasPrefix(oldPath, "/data/server") || !strings.HasPrefix(newPath, "/data/server") {
		return &DockerError{
			Op:  "rename_file",
			Msg: "access denied: paths must be within /data/server directory",
			Err: nil,
		}
	}
	
	// Use mv command to rename/move the file
	cmd := []string{"mv", oldPath, newPath}
	_, err := d.ExecCommand(containerID, cmd)
	return err
}

// =============================================================================
// Helper Functions for File Operations
// =============================================================================

func parseLsOutput(output string, basePath string) []*FileInfo {
	var files []*FileInfo
	lines := strings.Split(strings.TrimSpace(output), "\n")
	
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "total") {
			continue
		}
		
		// Parse ls -la output
		// Example: drwxr-xr-x 2 root root 4096 Jan 1 12:00 dirname
		fields := strings.Fields(line)
		if len(fields) < 9 {
			continue
		}
		
		// Get permissions and file type
		perms := fields[0]
		isDir := perms[0] == 'd'
		
		// Get size
		size, _ := strconv.ParseInt(fields[4], 10, 64)
		
		// Get name (everything after the time fields)
		// Fields: [0]perms [1]links [2]owner [3]group [4]size [5]month [6]day [7]time [8+]name
		name := strings.Join(fields[8:], " ")
		
		// Skip . and .. entries
		if name == "." || name == ".." {
			continue
		}
		
		// Clean the filename
		cleanName := cleanFilename(name)
		if cleanName == "" {
			continue
		}
		
		file := &FileInfo{
			Name:     cleanName,
			Path:     filepath.Join(basePath, cleanName),
			IsDir:    isDir,
			Size:     size,
			Mode:     perms[1:], // Skip file type indicator
			Owner:    fields[2],
			Group:    fields[3],
			Modified: time.Now(), // ls doesn't give us full timestamp
		}
		
		files = append(files, file)
	}
	
	return files
}

func sortFiles(files []*FileInfo) []*FileInfo {
	if len(files) == 0 {
		return files
	}
	
	// Separate directories and files
	var dirs []*FileInfo
	var regularFiles []*FileInfo
	
	for _, file := range files {
		if file.IsDir {
			dirs = append(dirs, file)
		} else {
			regularFiles = append(regularFiles, file)
		}
	}
	
	// Sort directories alphabetically by name
	for i := 0; i < len(dirs); i++ {
		for j := i + 1; j < len(dirs); j++ {
			if strings.ToLower(dirs[i].Name) > strings.ToLower(dirs[j].Name) {
				dirs[i], dirs[j] = dirs[j], dirs[i]
			}
		}
	}
	
	// Sort files by size (largest first)
	for i := 0; i < len(regularFiles); i++ {
		for j := i + 1; j < len(regularFiles); j++ {
			if regularFiles[i].Size < regularFiles[j].Size {
				regularFiles[i], regularFiles[j] = regularFiles[j], regularFiles[i]
			}
		}
	}
	
	// Combine: directories first, then files
	result := make([]*FileInfo, 0, len(files))
	result = append(result, dirs...)
	result = append(result, regularFiles...)
	
	return result
}

func createTarArchive(filename string, content []byte) (io.Reader, error) {
	var buf bytes.Buffer
	tw := tar.NewWriter(&buf)
	
	// Create tar header
	header := &tar.Header{
		Name: filename,
		Mode: 0644,
		Size: int64(len(content)),
		ModTime: time.Now(),
	}
	
	// Write header
	if err := tw.WriteHeader(header); err != nil {
		return nil, err
	}
	
	// Write content
	if _, err := tw.Write(content); err != nil {
		return nil, err
	}
	
	// Close tar writer
	if err := tw.Close(); err != nil {
		return nil, err
	}
	
	return &buf, nil
}

func cleanFilename(filename string) string {
	// Simple cleaning - just remove obvious problematic characters
	cleaned := strings.TrimSpace(filename)
	
	// Skip empty names and parent directory references
	if cleaned == "" || cleaned == "." || cleaned == ".." {
		return ""
	}
	
	// Remove any null bytes or other control characters
	cleaned = strings.ReplaceAll(cleaned, "\x00", "")
	cleaned = strings.ReplaceAll(cleaned, "\r", "")
	cleaned = strings.ReplaceAll(cleaned, "\n", "")
	
	return cleaned
}

func cleanDockerOutput(output string) string {
	// Docker exec output can contain stream multiplexing headers
	// These are 8-byte headers: [STREAM_TYPE, 0, 0, 0, SIZE_BYTE1, SIZE_BYTE2, SIZE_BYTE3, SIZE_BYTE4]
	// followed by the actual data
	
	// If the output starts with these control bytes, strip them
	if len(output) >= 8 {
		// Check if it looks like a Docker stream header (first byte is 1 or 2 for stdout/stderr)
		firstByte := output[0]
		if (firstByte == 1 || firstByte == 2) && output[1] == 0 && output[2] == 0 && output[3] == 0 {
			// Skip the 8-byte header
			if len(output) > 8 {
				return output[8:]
			}
		}
	}
	
	return output
}
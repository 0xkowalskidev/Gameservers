package main

import (
	"archive/tar"
	"bytes"
	"context"
	"encoding/base64"
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

func (d *DockerManager) StopContainer(containerID string) error {
	ctx := context.Background()

	timeout := 30 // 30 seconds timeout
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

func (d *DockerManager) CreateBackup(containerID, gameserverName string) error {
	// Generate timestamped backup filename
	timestamp := time.Now().Format("2006-01-02_15-04-05")
	backupFilename := fmt.Sprintf("backup-%s.tar.gz", timestamp)
	
	log.Info().Str("container_id", containerID).Str("backup_file", backupFilename).Msg("Creating backup")
	
	// First ensure the backups directory exists
	if err := d.execCommandSimple(containerID, []string{"mkdir", "-p", "/data/backups"}, "create_backup_dir"); err != nil {
		return err
	}
	
	// Create backup using tar inside the existing container
	cmd := []string{"tar", "-czf", fmt.Sprintf("/data/backups/%s", backupFilename), 
		"-C", "/data/server", "."}
	
	if err := d.execCommandSimple(containerID, cmd, "create_backup"); err != nil {
		return err
	}
	
	log.Info().Str("container_id", containerID).Str("backup_file", backupFilename).Msg("Backup created successfully")
	return nil
}

func (d *DockerManager) CleanupOldBackups(containerID string, maxBackups int) error {
	if maxBackups <= 0 {
		// Unlimited backups, no cleanup needed
		return nil
	}
	
	log.Info().Str("container_id", containerID).Int("max_backups", maxBackups).Msg("Cleaning up old backups")
	
	// List all backup files sorted by modification time (newest first)
	cmd := []string{"sh", "-c", "find /data/backups -name '*.tar.gz' -type f -printf '%T@ %p\\n' | sort -nr | cut -d' ' -f2-"}
	output, err := d.ExecCommand(containerID, cmd)
	if err != nil {
		return &DockerError{
			Op:  "list_backups",
			Msg: fmt.Sprintf("failed to list backups for cleanup in container %s", containerID),
			Err: err,
		}
	}
	
	backupFiles := strings.Split(strings.TrimSpace(string(output)), "\n")
	if len(backupFiles) <= maxBackups {
		// No cleanup needed
		log.Info().Str("container_id", containerID).Int("backup_count", len(backupFiles)).Int("max_backups", maxBackups).Msg("No backup cleanup needed")
		return nil
	}
	
	// Delete the oldest backups (files beyond maxBackups limit)
	filesToDelete := backupFiles[maxBackups:]
	for _, file := range filesToDelete {
		if strings.TrimSpace(file) == "" {
			continue
		}
		
		log.Info().Str("container_id", containerID).Str("backup_file", file).Msg("Deleting old backup")
		_, err := d.ExecCommand(containerID, []string{"rm", "-f", file})
		if err != nil {
			log.Error().Err(err).Str("container_id", containerID).Str("backup_file", file).Msg("Failed to delete old backup")
			// Continue with other files even if one fails
		}
	}
	
	log.Info().Str("container_id", containerID).Int("deleted_count", len(filesToDelete)).Msg("Backup cleanup completed")
	return nil
}

func (d *DockerManager) RestoreBackup(containerID, backupFilename string) error {
	log.Info().Str("container_id", containerID).Str("backup_file", backupFilename).Msg("Restoring backup")
	
	// Create temporary directory for backups during restore
	if err := d.execCommandSimple(containerID, []string{"mkdir", "-p", "/tmp/backups"}, "create_temp_dir"); err != nil {
		return err
	}
	
	// Clear server directory
	if err := d.execCommandSimple(containerID, []string{"sh", "-c", "find /data/server -mindepth 1 -delete"}, "clear_server_dir"); err != nil {
		return err
	}
	
	// Extract the backup
	backupPath := fmt.Sprintf("/data/backups/%s", backupFilename)
	if err := d.execCommandSimple(containerID, []string{"tar", "-xzf", backupPath, "-C", "/data/server"}, "extract_backup"); err != nil {
		return err
	}
	
	// Clean up temporary directory
	_, err := d.ExecCommand(containerID, []string{"rm", "-rf", "/tmp/backups"})
	if err != nil {
		log.Warn().Err(err).Msg("Failed to clean up temporary backup directory")
	}
	
	log.Info().Str("container_id", containerID).Str("backup_file", backupFilename).Msg("Backup restored successfully")
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

func (d *DockerManager) SendCommand(containerID string, command string) error {
	return d.execCommandSimple(containerID, []string{"/data/scripts/send-command.sh", command}, "send_command")
}

func (d *DockerManager) ExecCommand(containerID string, cmd []string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	
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
	
	// Read output - use a buffer with deadline
	var output []byte
	done := make(chan error, 1)
	go func() {
		var err error
		output, err = io.ReadAll(resp.Reader)
		done <- err
	}()
	
	select {
	case err := <-done:
		if err != nil {
			return "", &DockerError{
				Op:  "exec_read",
				Msg: fmt.Sprintf("failed to read exec output for container %s", containerID),
				Err: err,
			}
		}
	case <-ctx.Done():
		return "", &DockerError{
			Op:  "exec_timeout",
			Msg: fmt.Sprintf("exec timed out for container %s", containerID),
			Err: ctx.Err(),
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

// =============================================================================
// Path Validation Helpers
// =============================================================================

type pathValidation struct {
	allowedPrefixes []string
	defaultPath     string
}

var (
	serverOnlyValidation = pathValidation{
		allowedPrefixes: []string{"/data/server"},
		defaultPath:     "/data/server",
	}
	serverAndBackupsValidation = pathValidation{
		allowedPrefixes: []string{"/data/server", "/data/backups"},
		defaultPath:     "/data/server",
	}
)

func (d *DockerManager) validatePath(path string, validation pathValidation) (string, error) {
	// Handle empty paths
	if path == "" || path == "/" {
		return validation.defaultPath, nil
	}
	
	// Check if path has any allowed prefix
	for _, prefix := range validation.allowedPrefixes {
		if strings.HasPrefix(path, prefix) {
			return path, nil
		}
	}
	
	// If no valid prefix found, return default or error based on context
	if validation.defaultPath != "" {
		return validation.defaultPath, nil
	}
	
	return "", &DockerError{
		Op:  "validate_path",
		Msg: fmt.Sprintf("access denied: path must be within %v", validation.allowedPrefixes),
		Err: nil,
	}
}

// execCommandSimple is a helper for simple exec operations that just need to run a command
func (d *DockerManager) execCommandSimple(containerID string, cmd []string, operation string) error {
	_, err := d.ExecCommand(containerID, cmd)
	if err != nil {
		return &DockerError{
			Op:  operation,
			Msg: fmt.Sprintf("failed to %s in container %s", operation, containerID),
			Err: err,
		}
	}
	return nil
}

func (d *DockerManager) ListFiles(containerID string, path string) ([]*FileInfo, error) {
	// Validate and normalize path
	validPath, _ := d.validatePath(path, serverAndBackupsValidation)
	
	// Use simple ls -la command
	cmd := []string{"ls", "-la", validPath}
	
	output, err := d.ExecCommand(containerID, cmd)
	if err != nil {
		return nil, err
	}
	
	// Parse ls output and sort with context
	isBackupsPath := strings.Contains(validPath, "/backups")
	return sortFiles(parseLsOutput(output, validPath), isBackupsPath), nil
}

func (d *DockerManager) ReadFile(containerID string, path string) ([]byte, error) {
	// Validate path
	_, err := d.validatePath(path, serverOnlyValidation)
	if err != nil {
		return nil, err
	}
	
	// Use docker cp to safely read the file
	reader, err := d.copyFromContainer(containerID, path)
	if err != nil {
		return nil, err
	}
	defer reader.Close()
	
	// Extract file from tar archive
	tarReader := tar.NewReader(reader)
	header, err := tarReader.Next()
	if err != nil {
		return nil, &DockerError{
			Op:  "read_tar_header",
			Msg: fmt.Sprintf("failed to read tar header for file %s", path),
			Err: err,
		}
	}
	
	// Enforce size limit (10MB)
	const maxSize = 10 * 1024 * 1024
	if header.Size > maxSize {
		return nil, &DockerError{
			Op:  "read_file",
			Msg: fmt.Sprintf("file %s is too large (%d bytes, max %d bytes)", path, header.Size, maxSize),
			Err: fmt.Errorf("file too large"),
		}
	}
	
	// Read file content
	content := make([]byte, header.Size)
	n, err := io.ReadFull(tarReader, content)
	if err != nil && err != io.ErrUnexpectedEOF {
		return nil, &DockerError{
			Op:  "read_file_content",
			Msg: fmt.Sprintf("failed to read file content for %s", path),
			Err: err,
		}
	}
	
	// Return only the bytes that were actually read
	return content[:n], nil
}

func (d *DockerManager) WriteFile(containerID string, path string, content []byte) error {
	// Validate path
	_, err := d.validatePath(path, serverOnlyValidation)
	if err != nil {
		return err
	}
	
	return d.copyToContainer(containerID, path, content)
}

// copyToContainer is a helper that creates a tar archive and copies it to the container
func (d *DockerManager) copyToContainer(containerID string, path string, content []byte) error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	
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
	// Validate path
	_, err := d.validatePath(path, serverOnlyValidation)
	if err != nil {
		return err
	}
	
	return d.execCommandSimple(containerID, []string{"mkdir", "-p", path}, "create_directory")
}

func (d *DockerManager) DeletePath(containerID string, path string) error {
	// Validate path
	_, err := d.validatePath(path, serverAndBackupsValidation)
	if err != nil {
		return err
	}
	
	// Don't allow deleting root directories
	if path == "/data/server" || path == "/data/backups" {
		return &DockerError{
			Op:  "delete_path",
			Msg: "cannot delete root directories",
			Err: nil,
		}
	}
	
	return d.execCommandSimple(containerID, []string{"rm", "-rf", path}, "delete_path")
}

func (d *DockerManager) DownloadFile(containerID string, path string) (io.ReadCloser, error) {
	// Validate path
	_, err := d.validatePath(path, serverAndBackupsValidation)
	if err != nil {
		return nil, err
	}
	
	return d.copyFromContainer(containerID, path)
}

// copyFromContainer handles the Docker API path conversion and copy operation
func (d *DockerManager) copyFromContainer(containerID string, path string) (io.ReadCloser, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	
	// Use absolute path directly - Docker API can handle absolute paths
	// This is safer than assuming WORKDIR
	dockerPath := path
	
	reader, _, err := d.client.CopyFromContainer(ctx, containerID, dockerPath)
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
	// Validate path
	_, err := d.validatePath(destPath, serverOnlyValidation)
	if err != nil {
		return err
	}
	
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	
	// Copy to container
	err = d.client.CopyToContainer(ctx, containerID, destPath, reader, container.CopyToContainerOptions{})
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
	// Validate both paths
	_, err := d.validatePath(oldPath, serverOnlyValidation)
	if err != nil {
		return err
	}
	_, err = d.validatePath(newPath, serverOnlyValidation)
	if err != nil {
		return err
	}
	
	return d.execCommandSimple(containerID, []string{"mv", oldPath, newPath}, "rename_file")
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
		
		// Parse timestamp - for backup files, extract from filename; otherwise use ls output
		var modTime time.Time
		if strings.HasPrefix(cleanName, "backup-") && strings.HasSuffix(cleanName, ".tar.gz") {
			// Extract timestamp from backup filename: backup-YYYY-MM-DD_HH-MM-SS.tar.gz
			modTime = parseBackupTimestamp(cleanName)
		} else {
			// For other files, parse from ls output
			modTime = parseFileTimestamp(fields[5], fields[6], fields[7])
		}
		
		file := &FileInfo{
			Name:     cleanName,
			Path:     filepath.Join(basePath, cleanName),
			IsDir:    isDir,
			Size:     size,
			Mode:     perms[1:], // Skip file type indicator
			Owner:    fields[2],
			Group:    fields[3],
			Modified: modTime,
		}
		
		files = append(files, file)
	}
	
	return files
}

func parseBackupTimestamp(filename string) time.Time {
	// Extract timestamp from backup filename: backup-YYYY-MM-DD_HH-MM-SS.tar.gz
	// Remove "backup-" prefix and ".tar.gz" suffix
	if !strings.HasPrefix(filename, "backup-") || !strings.HasSuffix(filename, ".tar.gz") {
		return time.Now()
	}
	
	// Extract the timestamp part: YYYY-MM-DD_HH-MM-SS
	timestampPart := filename[7 : len(filename)-7] // Remove "backup-" and ".tar.gz"
	
	// Parse the timestamp: YYYY-MM-DD_HH-MM-SS
	parsedTime, err := time.Parse("2006-01-02_15-04-05", timestampPart)
	if err != nil {
		// Fallback to current time if parsing fails
		return time.Now()
	}
	
	return parsedTime
}

func parseFileTimestamp(month, day, timeOrYear string) time.Time {
	// Parse month
	monthMap := map[string]time.Month{
		"Jan": time.January, "Feb": time.February, "Mar": time.March,
		"Apr": time.April, "May": time.May, "Jun": time.June,
		"Jul": time.July, "Aug": time.August, "Sep": time.September,
		"Oct": time.October, "Nov": time.November, "Dec": time.December,
	}
	
	monthNum := monthMap[month]
	if monthNum == 0 {
		// Fallback to current time if parsing fails
		return time.Now()
	}
	
	// Parse day
	dayNum, err := strconv.Atoi(day)
	if err != nil {
		return time.Now()
	}
	
	now := time.Now()
	currentYear := now.Year()
	
	// Check if timeOrYear is a time (HH:MM) or year (YYYY)
	if strings.Contains(timeOrYear, ":") {
		// It's a time, assume current year
		timeParts := strings.Split(timeOrYear, ":")
		if len(timeParts) != 2 {
			return time.Now()
		}
		
		hour, err1 := strconv.Atoi(timeParts[0])
		minute, err2 := strconv.Atoi(timeParts[1])
		if err1 != nil || err2 != nil {
			return time.Now()
		}
		
		// Create date with current year
		fileTime := time.Date(currentYear, monthNum, dayNum, hour, minute, 0, 0, time.UTC)
		
		// If this date is in the future, it's probably from last year
		if fileTime.After(now) {
			fileTime = time.Date(currentYear-1, monthNum, dayNum, hour, minute, 0, 0, time.UTC)
		}
		
		return fileTime
	} else {
		// It's a year
		year, err := strconv.Atoi(timeOrYear)
		if err != nil {
			return time.Now()
		}
		
		// Assume noon for files from previous years
		return time.Date(year, monthNum, dayNum, 12, 0, 0, 0, time.UTC)
	}
}

func sortFiles(files []*FileInfo, isBackupsPath bool) []*FileInfo {
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
	
	// Sort files: by modification time for backups, by size for file manager
	if isBackupsPath {
		// Sort backups by modification time (newest first)
		for i := 0; i < len(regularFiles); i++ {
			for j := i + 1; j < len(regularFiles); j++ {
				if regularFiles[i].Modified.Before(regularFiles[j].Modified) {
					regularFiles[i], regularFiles[j] = regularFiles[j], regularFiles[i]
				}
			}
		}
	} else {
		// Sort files by size (largest first) for file manager
		for i := 0; i < len(regularFiles); i++ {
			for j := i + 1; j < len(regularFiles); j++ {
				if regularFiles[i].Size < regularFiles[j].Size {
					regularFiles[i], regularFiles[j] = regularFiles[j], regularFiles[i]
				}
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

// =============================================================================
// Smart Image Pull Strategy
// =============================================================================

// pullImageIfNeeded implements a smart pull strategy that only pulls if there's a newer image
func (d *DockerManager) pullImageIfNeeded(ctx context.Context, imageName string) error {
	log.Debug().Str("image", imageName).Msg("Checking if image pull is needed")
	
	// Check if we should pull the image
	shouldPull, err := d.shouldPullImage(ctx, imageName)
	if err != nil {
		log.Warn().Err(err).Str("image", imageName).Msg("Failed to check if image should be pulled, skipping pull")
		return nil // Don't fail container creation if we can't check
	}
	
	if !shouldPull {
		log.Debug().Str("image", imageName).Msg("Local image is up to date, skipping pull")
		return nil
	}
	
	log.Info().Str("image", imageName).Msg("Pulling newer image version")
	return d.pullImage(ctx, imageName)
}

// shouldPullImage determines if we should pull the image based on comparing local and remote digests
func (d *DockerManager) shouldPullImage(ctx context.Context, imageName string) (bool, error) {
	// First, check if the image exists locally
	localImage, err := d.client.ImageInspect(ctx, imageName)
	if err != nil {
		// Image doesn't exist locally, should pull
		log.Debug().Str("image", imageName).Msg("Image not found locally, should pull")
		return true, nil
	}
	
	// Get the local image digest
	var localDigest string
	if len(localImage.RepoDigests) > 0 {
		localDigest = localImage.RepoDigests[0]
		log.Debug().Str("image", imageName).Str("local_digest", localDigest).Msg("Found local image digest")
	} else {
		// No repo digest available (probably built locally), check by ID
		log.Debug().Str("image", imageName).Str("local_id", localImage.ID).Msg("No repo digest found, using image ID")
		localDigest = localImage.ID
	}
	
	// Get remote image digest
	remoteDigest, err := d.getRemoteImageDigest(ctx, imageName)
	if err != nil {
		log.Warn().Err(err).Str("image", imageName).Msg("Failed to get remote image digest, skipping pull")
		return false, nil // Don't pull if we can't check remote
	}
	
	// Compare digests
	needsPull := !strings.Contains(localDigest, remoteDigest) && localDigest != remoteDigest
	log.Debug().
		Str("image", imageName).
		Str("local_digest", localDigest).
		Str("remote_digest", remoteDigest).
		Bool("needs_pull", needsPull).
		Msg("Compared image digests")
		
	return needsPull, nil
}

// getRemoteImageDigest gets the digest of the remote image without pulling it
func (d *DockerManager) getRemoteImageDigest(ctx context.Context, imageName string) (string, error) {
	// Use Docker's built-in registry client to get image info
	// This approach uses the Docker daemon's registry authentication
	encodedAuth := base64.URLEncoding.EncodeToString([]byte("{}"))
	
	// Get image distribution inspect (this gets manifest info without pulling)
	resp, err := d.client.DistributionInspect(ctx, imageName, encodedAuth)
	if err != nil {
		return "", fmt.Errorf("failed to inspect remote image: %w", err)
	}
	
	digest := resp.Descriptor.Digest.String()
	log.Debug().Str("image", imageName).Str("remote_digest", digest).Msg("Retrieved remote image digest")
	
	return digest, nil
}

// pullImage pulls the specified image
func (d *DockerManager) pullImage(ctx context.Context, imageName string) error {
	log.Info().Str("image", imageName).Msg("Pulling Docker image")
	
	// Use default authentication (will use Docker daemon's auth)
	encodedAuth := base64.URLEncoding.EncodeToString([]byte("{}"))
	
	reader, err := d.client.ImagePull(ctx, imageName, image.PullOptions{
		RegistryAuth: encodedAuth,
	})
	if err != nil {
		return &DockerError{
			Op:  "pull",
			Msg: fmt.Sprintf("failed to pull image %s", imageName),
			Err: err,
		}
	}
	defer reader.Close()
	
	// Read the pull output for logging
	buf := new(bytes.Buffer)
	_, err = io.Copy(buf, reader)
	if err != nil {
		log.Warn().Err(err).Str("image", imageName).Msg("Failed to read pull output")
	} else {
		// Log pull progress (Docker returns JSON-lines format)
		output := buf.String()
		if strings.TrimSpace(output) != "" {
			log.Debug().Str("image", imageName).Str("pull_output", output).Msg("Image pull completed")
		}
	}
	
	log.Info().Str("image", imageName).Msg("Successfully pulled Docker image")
	return nil
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
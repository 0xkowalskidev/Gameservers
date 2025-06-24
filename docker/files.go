package docker

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
	"github.com/rs/zerolog/log"

	"0xkowalskidev/gameservers/models"
)

// SendCommand sends a command to the gameserver console
func (d *DockerManager) SendCommand(containerID string, command string) error {
	return d.execCommandSimple(containerID, []string{"/data/scripts/send-command.sh", command}, "send_command")
}

// ExecCommand executes a command in a container and returns the output
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

// StreamContainerLogs returns a stream of container logs
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
			Op:  "stream_logs",
			Msg: fmt.Sprintf("failed to stream logs for container %s", containerID),
			Err: err,
		}
	}

	return logs, nil
}

// StreamContainerStats returns a stream of container statistics
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

// Path validation types and helpers
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

// ListFiles lists files in a container directory
func (d *DockerManager) ListFiles(containerID string, path string) ([]*models.FileInfo, error) {
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

// ReadFile reads a file from a container
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

// WriteFile writes a file to a container
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

// CreateDirectory creates a directory in a container
func (d *DockerManager) CreateDirectory(containerID string, path string) error {
	// Validate path
	_, err := d.validatePath(path, serverOnlyValidation)
	if err != nil {
		return err
	}

	return d.execCommandSimple(containerID, []string{"mkdir", "-p", path}, "create_directory")
}

// DeletePath deletes a file or directory in a container
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

// DownloadFile downloads a file from a container
func (d *DockerManager) DownloadFile(containerID string, path string) (io.ReadCloser, error) {
	// Validate path
	validPath, err := d.validatePath(path, serverAndBackupsValidation)
	if err != nil {
		return nil, err
	}

	log.Info().Str("original_path", path).Str("valid_path", validPath).Str("container_id", containerID).Msg("Validated path for download")

	return d.copyFromContainer(containerID, validPath)
}

// copyFromContainer handles the Docker API path conversion and copy operation
func (d *DockerManager) copyFromContainer(containerID string, path string) (io.ReadCloser, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Use absolute path directly - Docker API can handle absolute paths
	dockerPath := path

	log.Info().Str("docker_path", dockerPath).Str("container_id", containerID).Msg("Attempting docker copy from container")

	reader, _, err := d.client.CopyFromContainer(ctx, containerID, dockerPath)
	if err != nil {
		log.Error().Err(err).Str("docker_path", dockerPath).Str("container_id", containerID).Msg("Docker copy from container failed")
		return nil, &DockerError{
			Op:  "copy_from_container",
			Msg: fmt.Sprintf("failed to copy file from container %s: %s", containerID, err.Error()),
			Err: err,
		}
	}

	log.Info().Str("docker_path", dockerPath).Str("container_id", containerID).Msg("Docker copy from container successful")
	return reader, nil
}

// UploadFile uploads a file to a container
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

// RenameFile renames a file in a container
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

// Helper functions for file operations

func parseLsOutput(output string, basePath string) []*models.FileInfo {
	var files []*models.FileInfo
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

		file := &models.FileInfo{
			Name:     cleanName,
			Path:     filepath.Join(basePath, cleanName),
			IsDir:    isDir,
			Size:     size,
			Modified: modTime.Format("2006-01-02 15:04:05"),
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

func sortFiles(files []*models.FileInfo, isBackupsPath bool) []*models.FileInfo {
	if len(files) == 0 {
		return files
	}

	// Separate directories and files
	var dirs []*models.FileInfo
	var regularFiles []*models.FileInfo

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
				// Compare modification time strings (YYYY-MM-DD HH:MM:SS format sorts correctly)
				if regularFiles[i].Modified < regularFiles[j].Modified {
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
	result := make([]*models.FileInfo, 0, len(files))
	result = append(result, dirs...)
	result = append(result, regularFiles...)

	return result
}

func createTarArchive(filename string, content []byte) (io.Reader, error) {
	var buf bytes.Buffer
	tw := tar.NewWriter(&buf)

	// Create tar header
	header := &tar.Header{
		Name:    filename,
		Mode:    0644,
		Size:    int64(len(content)),
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

package docker

import (
	"io"
	"strings"

	"0xkowalskidev/gameservers/models"
)

// MockDockerManager provides a mock implementation for testing
type MockDockerManager struct {
	containers  map[string]*models.Gameserver
	logs        map[string][]string
	shouldFail  map[string]bool
	volumes     map[string]*models.VolumeInfo
	files       map[string]map[string][]byte // containerID -> path -> content
	backups     map[string][]string         // containerID -> backup filenames
}

// NewMockDockerManager creates a new mock Docker manager
func NewMockDockerManager() *MockDockerManager {
	return &MockDockerManager{
		containers: make(map[string]*models.Gameserver),
		logs:       make(map[string][]string),
		shouldFail: make(map[string]bool),
		volumes:    make(map[string]*models.VolumeInfo),
		files:      make(map[string]map[string][]byte),
		backups:    make(map[string][]string),
	}
}

// SetShouldFail configures the mock to fail for specific operations
func (m *MockDockerManager) SetShouldFail(operation string, shouldFail bool) {
	m.shouldFail[operation] = shouldFail
}

// Container operations
func (m *MockDockerManager) CreateContainer(server *models.Gameserver) error {
	if m.shouldFail["create"] {
		return &DockerError{Op: "create", Msg: "mock create error"}
	}
	server.ContainerID = "mock-container-" + server.ID
	server.Status = models.StatusStopped
	m.containers[server.ContainerID] = server
	
	// Initialize file system for container
	if m.files[server.ContainerID] == nil {
		m.files[server.ContainerID] = make(map[string][]byte)
	}
	
	return nil
}

func (m *MockDockerManager) StartContainer(containerID string) error {
	if m.shouldFail["start"] {
		return &DockerError{Op: "start", Msg: "mock start error"}
	}
	if server, exists := m.containers[containerID]; exists {
		server.Status = models.StatusRunning
		return nil
	}
	return &DockerError{Op: "start", Msg: "container not found"}
}

func (m *MockDockerManager) StopContainer(containerID string) error {
	if m.shouldFail["stop"] {
		return &DockerError{Op: "stop", Msg: "mock stop error"}
	}
	if server, exists := m.containers[containerID]; exists {
		server.Status = models.StatusStopped
		return nil
	}
	return &DockerError{Op: "stop", Msg: "container not found"}
}

func (m *MockDockerManager) RemoveContainer(containerID string) error {
	if m.shouldFail["remove"] {
		return &DockerError{Op: "remove", Msg: "mock remove error"}
	}
	delete(m.containers, containerID)
	return nil
}

func (m *MockDockerManager) GetContainerStatus(containerID string) (models.GameserverStatus, error) {
	if m.shouldFail["status"] {
		return models.StatusError, &DockerError{Op: "status", Msg: "mock status error"}
	}
	if server, exists := m.containers[containerID]; exists {
		return server.Status, nil
	}
	return models.StatusError, &DockerError{Op: "status", Msg: "container not found"}
}

func (m *MockDockerManager) ListContainers() ([]string, error) {
	if m.shouldFail["list"] {
		return nil, &DockerError{Op: "list", Msg: "mock list error"}
	}
	var containerIDs []string
	for id := range m.containers {
		containerIDs = append(containerIDs, id)
	}
	return containerIDs, nil
}

// Logging operations
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

// Volume operations
func (m *MockDockerManager) CreateVolume(volumeName string) error {
	if m.shouldFail["create_volume"] {
		return &DockerError{Op: "create_volume", Msg: "mock create volume error"}
	}
	m.volumes[volumeName] = &models.VolumeInfo{
		Name:       volumeName,
		MountPoint: "/var/lib/docker/volumes/" + volumeName + "/_data",
		Driver:     "local",
		Labels:     map[string]string{"gameserver.managed": "true"},
	}
	return nil
}

func (m *MockDockerManager) RemoveVolume(volumeName string) error {
	if m.shouldFail["remove_volume"] {
		return &DockerError{Op: "remove_volume", Msg: "mock remove volume error"}
	}
	delete(m.volumes, volumeName)
	return nil
}

func (m *MockDockerManager) GetVolumeInfo(volumeName string) (*models.VolumeInfo, error) {
	if m.shouldFail["get_volume_info"] {
		return nil, &DockerError{Op: "get_volume_info", Msg: "mock get volume info error"}
	}
	if volume, exists := m.volumes[volumeName]; exists {
		return volume, nil
	}
	return nil, &DockerError{Op: "get_volume_info", Msg: "volume not found"}
}

// Backup operations
func (m *MockDockerManager) CreateBackup(containerID, gameserverName string) error {
	if m.shouldFail["create_backup"] {
		return &DockerError{Op: "create_backup", Msg: "mock create backup error"}
	}
	if m.backups[containerID] == nil {
		m.backups[containerID] = []string{}
	}
	backupName := "backup-2024-01-01_12-00-00.tar.gz"
	m.backups[containerID] = append(m.backups[containerID], backupName)
	return nil
}

func (m *MockDockerManager) CleanupOldBackups(containerID string, maxBackups int) error {
	if m.shouldFail["cleanup_backups"] {
		return &DockerError{Op: "cleanup_backups", Msg: "mock cleanup backups error"}
	}
	// If maxBackups is 0 or negative, unlimited backups - no cleanup
	if maxBackups <= 0 {
		return nil
	}
	if backups, exists := m.backups[containerID]; exists && len(backups) > maxBackups {
		m.backups[containerID] = backups[:maxBackups]
	}
	return nil
}

func (m *MockDockerManager) RestoreBackup(containerID, backupFilename string) error {
	if m.shouldFail["restore_backup"] {
		return &DockerError{Op: "restore_backup", Msg: "mock restore backup error"}
	}
	// Check if backup exists
	if backups, exists := m.backups[containerID]; exists {
		for _, backup := range backups {
			if backup == backupFilename {
				return nil
			}
		}
	}
	return &DockerError{Op: "restore_backup", Msg: "backup not found"}
}

// File operations
func (m *MockDockerManager) ListFiles(containerID string, path string) ([]*models.FileInfo, error) {
	if m.shouldFail["list_files"] {
		return nil, &DockerError{Op: "list_files", Msg: "mock list files error"}
	}
	
	files := []*models.FileInfo{
		{Name: "server.properties", Path: "/data/server/server.properties", IsDir: false, Size: 1024, Modified: "2024-01-01 12:00:00"},
		{Name: "world", Path: "/data/server/world", IsDir: true, Size: 0, Modified: "2024-01-01 12:00:00"},
	}
	
	if strings.Contains(path, "backup") {
		if backups, exists := m.backups[containerID]; exists {
			for _, backup := range backups {
				files = append(files, &models.FileInfo{
					Name: backup, Path: "/data/backups/" + backup, IsDir: false, Size: 1048576, Modified: "2024-01-01 12:00:00",
				})
			}
		}
	}
	
	return files, nil
}

func (m *MockDockerManager) ReadFile(containerID string, path string) ([]byte, error) {
	if m.shouldFail["read_file"] {
		return nil, &DockerError{Op: "read_file", Msg: "mock read file error"}
	}
	if containerFiles, exists := m.files[containerID]; exists {
		if content, exists := containerFiles[path]; exists {
			return content, nil
		}
	}
	return []byte("mock file content"), nil
}

func (m *MockDockerManager) WriteFile(containerID string, path string, content []byte) error {
	if m.shouldFail["write_file"] {
		return &DockerError{Op: "write_file", Msg: "mock write file error"}
	}
	if m.files[containerID] == nil {
		m.files[containerID] = make(map[string][]byte)
	}
	m.files[containerID][path] = content
	return nil
}

func (m *MockDockerManager) CreateDirectory(containerID string, path string) error {
	if m.shouldFail["create_directory"] {
		return &DockerError{Op: "create_directory", Msg: "mock create directory error"}
	}
	// Mark directory as created by adding empty entry
	if m.files[containerID] == nil {
		m.files[containerID] = make(map[string][]byte)
	}
	m.files[containerID][path+"/"] = []byte{}
	return nil
}

func (m *MockDockerManager) DeletePath(containerID string, path string) error {
	if m.shouldFail["delete_path"] {
		return &DockerError{Op: "delete_path", Msg: "mock delete path error"}
	}
	if containerFiles, exists := m.files[containerID]; exists {
		delete(containerFiles, path)
	}
	return nil
}

func (m *MockDockerManager) DownloadFile(containerID string, path string) (io.ReadCloser, error) {
	if m.shouldFail["download_file"] {
		return nil, &DockerError{Op: "download_file", Msg: "mock download file error"}
	}
	content := "mock file content for download"
	if containerFiles, exists := m.files[containerID]; exists {
		if fileContent, exists := containerFiles[path]; exists {
			content = string(fileContent)
		}
	}
	return io.NopCloser(strings.NewReader(content)), nil
}

func (m *MockDockerManager) RenameFile(containerID string, oldPath string, newPath string) error {
	if m.shouldFail["rename_file"] {
		return &DockerError{Op: "rename_file", Msg: "mock rename file error"}
	}
	if containerFiles, exists := m.files[containerID]; exists {
		if content, exists := containerFiles[oldPath]; exists {
			containerFiles[newPath] = content
			delete(containerFiles, oldPath)
		}
	}
	return nil
}

func (m *MockDockerManager) UploadFile(containerID string, destPath string, reader io.Reader) error {
	if m.shouldFail["upload_file"] {
		return &DockerError{Op: "upload_file", Msg: "mock upload file error"}
	}
	content, _ := io.ReadAll(reader)
	if m.files[containerID] == nil {
		m.files[containerID] = make(map[string][]byte)
	}
	m.files[containerID][destPath] = content
	return nil
}

// Command operations
func (m *MockDockerManager) SendCommand(containerID string, command string) error {
	if m.shouldFail["send_command"] {
		return &DockerError{Op: "send_command", Msg: "mock send command error"}
	}
	return nil
}

func (m *MockDockerManager) ExecCommand(containerID string, cmd []string) (string, error) {
	if m.shouldFail["exec_command"] {
		return "", &DockerError{Op: "exec_command", Msg: "mock exec command error"}
	}
	return "mock command output", nil
}

// Helper methods for tests
func (m *MockDockerManager) GetContainer(containerID string) *models.Gameserver {
	return m.containers[containerID]
}

func (m *MockDockerManager) HasVolume(volumeName string) bool {
	_, exists := m.volumes[volumeName]
	return exists
}

func (m *MockDockerManager) GetBackups(containerID string) []string {
	return m.backups[containerID]
}
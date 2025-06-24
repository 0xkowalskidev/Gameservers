package database

import (
	"io"
	"strings"

	"0xkowalskidev/gameservers/models"
)

// MockDockerManager provides a mock implementation for testing database components
type MockDockerManager struct {
	shouldFail map[string]bool
}

// NewMockDockerManager creates a new mock Docker manager
func NewMockDockerManager() *MockDockerManager {
	return &MockDockerManager{
		shouldFail: make(map[string]bool),
	}
}

// SetShouldFail configures the mock to fail for specific operations
func (m *MockDockerManager) SetShouldFail(operation string, shouldFail bool) {
	if m.shouldFail == nil {
		m.shouldFail = make(map[string]bool)
	}
	m.shouldFail[operation] = shouldFail
}

// Container operations
func (m *MockDockerManager) CreateContainer(server *models.Gameserver) error {
	if m.shouldFail["create"] {
		return &mockDockerError{Op: "create", Msg: "mock create error"}
	}
	server.ContainerID = "mock-container-" + server.ID
	server.Status = models.StatusStopped
	return nil
}

func (m *MockDockerManager) StartContainer(containerID string) error {
	if m.shouldFail["start"] {
		return &mockDockerError{Op: "start", Msg: "mock start error"}
	}
	return nil
}

func (m *MockDockerManager) StopContainer(containerID string) error {
	if m.shouldFail["stop"] {
		return &mockDockerError{Op: "stop", Msg: "mock stop error"}
	}
	return nil
}

func (m *MockDockerManager) RemoveContainer(containerID string) error {
	if m.shouldFail["remove"] {
		return &mockDockerError{Op: "remove", Msg: "mock remove error"}
	}
	return nil
}

func (m *MockDockerManager) GetContainerStatus(containerID string) (models.GameserverStatus, error) {
	if m.shouldFail["status"] {
		return models.StatusError, &mockDockerError{Op: "status", Msg: "mock status error"}
	}
	return models.StatusStopped, nil
}

func (m *MockDockerManager) ListContainers() ([]string, error) {
	if m.shouldFail["list"] {
		return nil, &mockDockerError{Op: "list", Msg: "mock list error"}
	}
	return []string{}, nil
}

// Logging operations
func (m *MockDockerManager) StreamContainerLogs(containerID string) (io.ReadCloser, error) {
	if m.shouldFail["stream_logs"] {
		return nil, &mockDockerError{Op: "stream_logs", Msg: "mock stream logs error"}
	}
	return io.NopCloser(strings.NewReader("Mock log stream")), nil
}

func (m *MockDockerManager) StreamContainerStats(containerID string) (io.ReadCloser, error) {
	if m.shouldFail["stream_stats"] {
		return nil, &mockDockerError{Op: "stream_stats", Msg: "mock stream stats error"}
	}
	return io.NopCloser(strings.NewReader(`{"cpu_stats":{"cpu_usage":{"total_usage":100},"system_cpu_usage":200},"memory_stats":{"usage":536870912,"limit":1073741824}}`)), nil
}

// Volume operations
func (m *MockDockerManager) CreateVolume(volumeName string) error {
	if m.shouldFail["create_volume"] {
		return &mockDockerError{Op: "create_volume", Msg: "mock create volume error"}
	}
	return nil
}

func (m *MockDockerManager) RemoveVolume(volumeName string) error {
	if m.shouldFail["remove_volume"] {
		return &mockDockerError{Op: "remove_volume", Msg: "mock remove volume error"}
	}
	return nil
}

func (m *MockDockerManager) GetVolumeInfo(volumeName string) (*models.VolumeInfo, error) {
	if m.shouldFail["get_volume_info"] {
		return nil, &mockDockerError{Op: "get_volume_info", Msg: "mock get volume info error"}
	}
	return &models.VolumeInfo{Name: volumeName}, nil
}

// Backup operations
func (m *MockDockerManager) CreateBackup(containerID, gameserverName string) error {
	if m.shouldFail["create_backup"] {
		return &mockDockerError{Op: "create_backup", Msg: "mock create backup error"}
	}
	return nil
}

func (m *MockDockerManager) CleanupOldBackups(containerID string, maxBackups int) error {
	if m.shouldFail["cleanup_backups"] {
		return &mockDockerError{Op: "cleanup_backups", Msg: "mock cleanup backups error"}
	}
	return nil
}

func (m *MockDockerManager) RestoreBackup(containerID, backupFilename string) error {
	if m.shouldFail["restore_backup"] {
		return &mockDockerError{Op: "restore_backup", Msg: "mock restore backup error"}
	}
	return nil
}

// File operations
func (m *MockDockerManager) ListFiles(containerID string, path string) ([]*models.FileInfo, error) {
	if m.shouldFail["list_files"] {
		return nil, &mockDockerError{Op: "list_files", Msg: "mock list files error"}
	}
	return []*models.FileInfo{}, nil
}

func (m *MockDockerManager) ReadFile(containerID string, path string) ([]byte, error) {
	if m.shouldFail["read_file"] {
		return nil, &mockDockerError{Op: "read_file", Msg: "mock read file error"}
	}
	return []byte("mock file content"), nil
}

func (m *MockDockerManager) WriteFile(containerID string, path string, content []byte) error {
	if m.shouldFail["write_file"] {
		return &mockDockerError{Op: "write_file", Msg: "mock write file error"}
	}
	return nil
}

func (m *MockDockerManager) CreateDirectory(containerID string, path string) error {
	if m.shouldFail["create_directory"] {
		return &mockDockerError{Op: "create_directory", Msg: "mock create directory error"}
	}
	return nil
}

func (m *MockDockerManager) DeletePath(containerID string, path string) error {
	if m.shouldFail["delete_path"] {
		return &mockDockerError{Op: "delete_path", Msg: "mock delete path error"}
	}
	return nil
}

func (m *MockDockerManager) DownloadFile(containerID string, path string) (io.ReadCloser, error) {
	if m.shouldFail["download_file"] {
		return nil, &mockDockerError{Op: "download_file", Msg: "mock download file error"}
	}
	return io.NopCloser(strings.NewReader("mock file content")), nil
}

func (m *MockDockerManager) RenameFile(containerID string, oldPath string, newPath string) error {
	if m.shouldFail["rename_file"] {
		return &mockDockerError{Op: "rename_file", Msg: "mock rename file error"}
	}
	return nil
}

func (m *MockDockerManager) UploadFile(containerID string, destPath string, reader io.Reader) error {
	if m.shouldFail["upload_file"] {
		return &mockDockerError{Op: "upload_file", Msg: "mock upload file error"}
	}
	return nil
}

// Command operations
func (m *MockDockerManager) SendCommand(containerID string, command string) error {
	if m.shouldFail["send_command"] {
		return &mockDockerError{Op: "send_command", Msg: "mock send command error"}
	}
	return nil
}

func (m *MockDockerManager) ExecCommand(containerID string, cmd []string) (string, error) {
	if m.shouldFail["exec_command"] {
		return "", &mockDockerError{Op: "exec_command", Msg: "mock exec command error"}
	}
	return "mock command output", nil
}

// Mock error type for database tests
type mockDockerError struct {
	Op  string
	Msg string
}

func (e *mockDockerError) Error() string {
	return e.Op + ": " + e.Msg
}
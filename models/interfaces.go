package models

import (
	"io"
)

type DockerManagerInterface interface {
	CreateContainer(server *Gameserver) error
	StartContainer(containerID string) error
	StopContainer(containerID string) error
	RemoveContainer(containerID string) error
	SendCommand(containerID string, command string) error
	GetContainerStatus(containerID string) (GameserverStatus, error)
	StreamContainerLogs(containerID string) (io.ReadCloser, error)
	StreamContainerStats(containerID string) (io.ReadCloser, error)
	ListContainers() ([]string, error)
	CreateVolume(volumeName string) error
	RemoveVolume(volumeName string) error
	GetVolumeInfo(volumeName string) (*VolumeInfo, error)
	CreateBackup(gameserverID, backupPath string) error
	RestoreBackup(gameserverID, backupPath string) error
	CleanupOldBackups(containerID string, maxBackups int) error
	// File operations
	ListFiles(containerID string, path string) ([]*FileInfo, error)
	ReadFile(containerID string, path string) ([]byte, error)
	WriteFile(containerID string, path string, content []byte) error
	CreateDirectory(containerID string, path string) error
	DeletePath(containerID string, path string) error
	DownloadFile(containerID string, path string) (io.ReadCloser, error)
	UploadFile(containerID string, destPath string, reader io.Reader) error
	RenameFile(containerID string, oldPath string, newPath string) error
}

package models

import (
	"io"
	
	"github.com/0xkowalskidev/gameserverquery/protocol"
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

// Domain-specific service interfaces

type GameserverServiceInterface interface {
	CreateGameserver(server *Gameserver) error
	GetGameserver(id string) (*Gameserver, error)
	UpdateGameserver(server *Gameserver) error
	ListGameservers() ([]*Gameserver, error)
	StartGameserver(id string) error
	StopGameserver(id string) error
	RestartGameserver(id string) error
	DeleteGameserver(id string) error
	SendGameserverCommand(id string, command string) error
	StreamGameserverLogs(id string) (io.ReadCloser, error)
	StreamGameserverStats(id string) (io.ReadCloser, error)
	GetGameserverStatus(id string) (GameserverStatus, error)
	GetGameserverQuery(id string) (*protocol.ServerInfo, error)
}

type GameServiceInterface interface {
	ListGames() ([]*Game, error)
	GetGame(id string) (*Game, error)
	CreateGame(game *Game) error
	UpdateGame(game *Game) error
	DeleteGame(id string) error
}

type TaskServiceInterface interface {
	CreateScheduledTask(task *ScheduledTask) error
	GetScheduledTask(id string) (*ScheduledTask, error)
	UpdateScheduledTask(task *ScheduledTask) error
	DeleteScheduledTask(id string) error
	ListScheduledTasksForGameserver(gameserverID string) ([]*ScheduledTask, error)
}

type BackupServiceInterface interface {
	CreateGameserverBackup(gameserverID string) error
	RestoreGameserverBackup(gameserverID, backupFilename string) error
	ListGameserverBackups(gameserverID string) ([]*FileInfo, error)
}

type FileServiceInterface interface {
	ListFiles(containerID string, path string) ([]*FileInfo, error)
	ReadFile(containerID string, path string) ([]byte, error)
	WriteFile(containerID string, path string, content []byte) error
	CreateDirectory(containerID string, path string) error
	DeletePath(containerID string, path string) error
	DownloadFile(containerID string, path string) (io.ReadCloser, error)
	RenameFile(containerID string, oldPath string, newPath string) error
	UploadFile(containerID string, destPath string, reader io.Reader) error
}

// DatabaseInterface defines operations for database access
type DatabaseInterface interface {
	// Game operations
	ListGames() ([]*Game, error)
	GetGame(id string) (*Game, error)
	CreateGame(game *Game) error
	UpdateGame(game *Game) error
	DeleteGame(id string) error

	// Gameserver operations
	ListGameservers() ([]*Gameserver, error)
	GetGameserver(id string) (*Gameserver, error)
	CreateGameserver(server *Gameserver) error
	UpdateGameserver(server *Gameserver) error
	DeleteGameserver(id string) error

	// Task operations
	CreateScheduledTask(task *ScheduledTask) error
	GetScheduledTask(id string) (*ScheduledTask, error)
	UpdateScheduledTask(task *ScheduledTask) error
	DeleteScheduledTask(id string) error
	ListScheduledTasksForGameserver(gameserverID string) ([]*ScheduledTask, error)

	// Port allocation operations
	AllocatePort() (int, error)
	ReleasePort(port int) error
	IsPortAvailable(port int) bool
}

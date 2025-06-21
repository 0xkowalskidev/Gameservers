package main

import (
	"io"
	"time"
)

type GameserverStatus string

const (
	StatusStopped GameserverStatus = "stopped"
	StatusStarting GameserverStatus = "starting"
	StatusRunning GameserverStatus = "running"
	StatusStopping GameserverStatus = "stopping"
	StatusError   GameserverStatus = "error"
)

type Game struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Image       string    `json:"image"`
	DefaultPort int       `json:"default_port"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type Gameserver struct {
	ID          string            `json:"id"`
	Name        string            `json:"name"`
	GameID      string            `json:"game_id"`
	ContainerID string            `json:"container_id,omitempty"`
	Status      GameserverStatus  `json:"status"`
	Port        int               `json:"port"`
	MemoryMB    int               `json:"memory_mb"`    // Memory limit in MB
	CPUCores    float64           `json:"cpu_cores"`    // CPU cores (0 = unlimited)
	Environment []string          `json:"environment,omitempty"`
	Volumes     []string          `json:"volumes,omitempty"`
	CreatedAt   time.Time         `json:"created_at"`
	UpdatedAt   time.Time         `json:"updated_at"`
	
	// Derived fields (not stored in DB)
	GameType    string            `json:"game_type"` // From Game.Name
	Image       string            `json:"image"`     // From Game.Image
	MemoryGB    float64           `json:"memory_gb"` // MemoryMB converted to GB for display
	
	// Volume info (derived field)
	VolumeInfo  *VolumeInfo       `json:"volume_info,omitempty"`
}

type VolumeInfo struct {
	Name       string            `json:"name"`
	MountPoint string            `json:"mount_point"`
	Driver     string            `json:"driver"`
	CreatedAt  string            `json:"created_at"`
	Labels     map[string]string `json:"labels"`
}


type DockerManagerInterface interface {
	CreateContainer(server *Gameserver) error
	StartContainer(containerID string) error
	RemoveContainer(containerID string) error
	GetContainerStatus(containerID string) (GameserverStatus, error)
	StreamContainerLogs(containerID string) (io.ReadCloser, error)
	StreamContainerStats(containerID string) (io.ReadCloser, error)
	ListContainers() ([]string, error)
	CreateVolume(volumeName string) error
	RemoveVolume(volumeName string) error
	GetVolumeInfo(volumeName string) (*VolumeInfo, error)
}
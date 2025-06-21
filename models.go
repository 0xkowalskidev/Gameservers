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
	Environment []string          `json:"environment,omitempty"`
	Volumes     []string          `json:"volumes,omitempty"`
	CreatedAt   time.Time         `json:"created_at"`
	UpdatedAt   time.Time         `json:"updated_at"`
	
	// Derived fields (not stored in DB)
	GameType    string            `json:"game_type"` // From Game.Name
	Image       string            `json:"image"`     // From Game.Image
}

type ContainerStats struct {
	CPUPercent    float64 `json:"cpu_percent"`
	MemoryUsage   uint64  `json:"memory_usage"`
	MemoryLimit   uint64  `json:"memory_limit"`
	MemoryPercent float64 `json:"memory_percent"`
	NetworkRx     uint64  `json:"network_rx"`
	NetworkTx     uint64  `json:"network_tx"`
}

type DockerManagerInterface interface {
	CreateContainer(server *Gameserver) error
	StartContainer(containerID string) error
	StopContainer(containerID string) error
	RestartContainer(containerID string) error
	RemoveContainer(containerID string) error
	GetContainerStatus(containerID string) (GameserverStatus, error)
	GetContainerStats(containerID string) (*ContainerStats, error)
	GetContainerLogs(containerID string, lines int) ([]string, error)
	StreamContainerLogs(containerID string) (io.ReadCloser, error)
	ListContainers() ([]string, error)
}
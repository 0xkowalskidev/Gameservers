package main

import (
	"io"
	"time"
)

type GameServerStatus string

const (
	StatusStopped GameServerStatus = "stopped"
	StatusStarting GameServerStatus = "starting"
	StatusRunning GameServerStatus = "running"
	StatusStopping GameServerStatus = "stopping"
	StatusError   GameServerStatus = "error"
)

type GameServer struct {
	ID          string            `json:"id"`
	Name        string            `json:"name"`
	GameType    string            `json:"game_type"`
	Image       string            `json:"image"`
	ContainerID string            `json:"container_id,omitempty"`
	Status      GameServerStatus  `json:"status"`
	Port        int               `json:"port"`
	Environment []string          `json:"environment,omitempty"`
	Volumes     []string          `json:"volumes,omitempty"`
	CreatedAt   time.Time         `json:"created_at"`
	UpdatedAt   time.Time         `json:"updated_at"`
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
	CreateContainer(server *GameServer) error
	StartContainer(containerID string) error
	StopContainer(containerID string) error
	RestartContainer(containerID string) error
	RemoveContainer(containerID string) error
	GetContainerStatus(containerID string) (GameServerStatus, error)
	GetContainerStats(containerID string) (*ContainerStats, error)
	GetContainerLogs(containerID string, lines int) ([]string, error)
	StreamContainerLogs(containerID string) (io.ReadCloser, error)
	ListContainers() ([]string, error)
}
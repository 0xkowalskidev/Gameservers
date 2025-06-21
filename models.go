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


type DockerManagerInterface interface {
	CreateContainer(server *Gameserver) error
	StartContainer(containerID string) error
	StopContainer(containerID string) error
	RestartContainer(containerID string) error
	RemoveContainer(containerID string) error
	GetContainerStatus(containerID string) (GameserverStatus, error)
	StreamContainerLogs(containerID string) (io.ReadCloser, error)
	StreamContainerStats(containerID string) (io.ReadCloser, error)
	ListContainers() ([]string, error)
}
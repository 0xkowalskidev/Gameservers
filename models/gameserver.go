package models

import (
	"time"
)

type GameserverStatus string

const (
	StatusStopped  GameserverStatus = "stopped"
	StatusStarting GameserverStatus = "starting"
	StatusRunning  GameserverStatus = "running"
	StatusStopping GameserverStatus = "stopping"
	StatusError    GameserverStatus = "error"
)

type Gameserver struct {
	ID           string           `json:"id"`
	Name         string           `json:"name"`
	GameID       string           `json:"game_id"`
	ContainerID  string           `json:"container_id,omitempty"`
	Status       GameserverStatus `json:"status"`
	PortMappings []PortMapping    `json:"port_mappings"`
	MemoryMB     int              `json:"memory_mb"`   // Memory limit in MB
	CPUCores     float64          `json:"cpu_cores"`   // CPU cores (0 = unlimited)
	MaxBackups   int              `json:"max_backups"` // Maximum number of backups to keep (0 = unlimited)
	Environment  []string         `json:"environment,omitempty"`
	Volumes      []string         `json:"volumes,omitempty"`
	CreatedAt    time.Time        `json:"created_at"`
	UpdatedAt    time.Time        `json:"updated_at"`

	// Derived fields (not stored in DB)
	GameType string  `json:"game_type"` // From Game.Name
	Image    string  `json:"image"`     // From Game.Image
	MemoryGB float64 `json:"memory_gb"` // MemoryMB converted to GB for display

	// Volume info (derived field)
	VolumeInfo *VolumeInfo `json:"volume_info,omitempty"`
}

// GetGamePort returns the primary game connection port
func (g *Gameserver) GetGamePort() *PortMapping {
	for i := range g.PortMappings {
		if g.PortMappings[i].Name == "game" {
			return &g.PortMappings[i]
		}
	}
	// Fallback to first port if no "game" port found
	if len(g.PortMappings) > 0 {
		return &g.PortMappings[0]
	}
	return nil
}

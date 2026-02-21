package models

import (
	"time"

	"gorm.io/gorm"
)

type GameserverStatus string

const (
	StatusStopped           GameserverStatus = "stopped"
	StatusPullingImage      GameserverStatus = "pulling_image"
	StatusCreatingContainer GameserverStatus = "creating_container"
	StatusStartingContainer GameserverStatus = "starting_container"
	StatusWaitingReady      GameserverStatus = "waiting_ready"
	StatusRunning           GameserverStatus = "running"
	StatusStopping          GameserverStatus = "stopping"
	StatusDeleting          GameserverStatus = "deleting"
	StatusError             GameserverStatus = "error"
)

// IsTransitional returns true if the status represents an in-progress state
func (s GameserverStatus) IsTransitional() bool {
	switch s {
	case StatusPullingImage, StatusCreatingContainer, StatusStartingContainer, StatusWaitingReady, StatusStopping, StatusDeleting:
		return true
	}
	return false
}

type Gameserver struct {
	ID           string           `json:"id" gorm:"primaryKey;type:varchar(50)"`
	Name         string           `json:"name" gorm:"type:varchar(200);not null"`
	GameID       string           `json:"game_id" gorm:"type:varchar(50);not null;index"`
	ContainerID  string           `json:"container_id,omitempty" gorm:"type:varchar(100)"`
	Status       GameserverStatus `json:"status" gorm:"type:varchar(20);not null;default:'stopped'"`
	PortMappings []PortMapping    `json:"port_mappings" gorm:"serializer:json"`
	MemoryMB     int              `json:"memory_mb" gorm:"not null;default:1024"`   // Memory limit in MB
	CPUCores     float64          `json:"cpu_cores" gorm:"not null;default:0"`      // CPU cores (0 = unlimited)
	MaxBackups   int              `json:"max_backups" gorm:"not null;default:10"`   // Maximum number of backups to keep (0 = unlimited)
	Environment  []string         `json:"environment,omitempty" gorm:"serializer:json"`
	EnabledMods  []string         `json:"enabled_mods,omitempty" gorm:"serializer:json"`
	Volumes      []string         `json:"volumes,omitempty" gorm:"serializer:json"`
	CreatedAt    time.Time        `json:"created_at"`
	UpdatedAt    time.Time        `json:"updated_at"`
	DeletedAt    gorm.DeletedAt   `json:"deleted_at,omitempty" gorm:"index"`

	// Relations (removed foreign key constraint to avoid migration issues)
	Game *Game `json:"game,omitempty" gorm:"-"`

	// Derived fields (not stored in DB)
	GameType string  `json:"game_type" gorm:"-"` // From Game.Name
	Image    string  `json:"image" gorm:"-"`     // From Game.Image
	IconPath string  `json:"icon_path" gorm:"-"` // From Game.IconPath
	MemoryGB float64 `json:"memory_gb" gorm:"-"` // MemoryMB converted to GB for display

	// Volume info (derived field)
	VolumeInfo *VolumeInfo `json:"volume_info,omitempty" gorm:"-"`
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

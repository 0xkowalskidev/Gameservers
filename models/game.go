package models

import (
	"strings"
	"time"

	"gorm.io/gorm"
)

type ConfigVar struct {
	Name        string `json:"name" gorm:"type:varchar(100);not null"`         // Environment variable name
	DisplayName string `json:"display_name" gorm:"type:varchar(200);not null"` // Human-readable name
	Type        string `json:"type" gorm:"type:varchar(50);default:'text'"`    // Input type: text, number, boolean, password, select
	Options     string `json:"options" gorm:"type:text"`                       // For select type: "value1=Label 1,value2=Label 2"
	Required    bool   `json:"required" gorm:"not null;default:false"`         // Whether this config is required
	Default     string `json:"default" gorm:"type:text"`                       // Default value (empty if no default)
	Description string `json:"description" gorm:"type:text"`                   // Help text for users
}

type Game struct {
	ID            string        `json:"id" gorm:"primaryKey;type:varchar(50)"`
	Name          string        `json:"name" gorm:"type:varchar(100);not null"`
	Slug          string        `json:"slug" gorm:"type:varchar(100);not null"` // Query slug for gameserver query library
	Image         string        `json:"image" gorm:"type:varchar(500);not null"`
	IconPath      string        `json:"icon_path" gorm:"type:varchar(500)"`     // Path to the game icon (.ico)
	GridImagePath string        `json:"grid_image_path" gorm:"type:varchar(500)"` // Path to the grid image (.png)
	PortMappings  []PortMapping `json:"port_mappings" gorm:"serializer:json"`
	ConfigVars    []ConfigVar   `json:"config_vars" gorm:"serializer:json"`   // Required and optional configs
	MinMemoryMB   int           `json:"min_memory_mb" gorm:"not null;default:512"` // Minimum memory to run
	RecMemoryMB   int           `json:"rec_memory_mb" gorm:"not null;default:1024"` // Recommended memory
	CreatedAt     time.Time     `json:"created_at"`
	UpdatedAt     time.Time     `json:"updated_at"`
	DeletedAt     gorm.DeletedAt `json:"deleted_at,omitempty" gorm:"index"`
}

// ValidateEnvironment checks if all required config vars are provided in environment
func (g *Game) ValidateEnvironment(env []string) []string {
	var missing []string

	// Convert environment slice to map for easy lookup
	envMap := make(map[string]string)
	for _, envVar := range env {
		parts := strings.SplitN(envVar, "=", 2)
		if len(parts) == 2 {
			envMap[parts[0]] = parts[1]
		}
	}

	// Check each required config var
	for _, configVar := range g.ConfigVars {
		if configVar.Required {
			if value, exists := envMap[configVar.Name]; !exists || value == "" {
				missing = append(missing, configVar.Name)
			}
		}
	}

	return missing
}

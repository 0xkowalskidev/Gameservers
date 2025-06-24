package models

import (
	"strings"
	"time"
)

type ConfigVar struct {
	Name        string `json:"name"`         // Environment variable name
	DisplayName string `json:"display_name"` // Human-readable name
	Required    bool   `json:"required"`     // Whether this config is required
	Default     string `json:"default"`      // Default value (empty if no default)
	Description string `json:"description"`  // Help text for users
}

type Game struct {
	ID           string        `json:"id"`
	Name         string        `json:"name"`
	Image        string        `json:"image"`
	PortMappings []PortMapping `json:"port_mappings"`
	ConfigVars   []ConfigVar   `json:"config_vars"`   // Required and optional configs
	MinMemoryMB  int           `json:"min_memory_mb"` // Minimum memory to run
	RecMemoryMB  int           `json:"rec_memory_mb"` // Recommended memory
	CreatedAt    time.Time     `json:"created_at"`
	UpdatedAt    time.Time     `json:"updated_at"`
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

// GetRequiredConfigs returns a list of required configuration variables
func (g *Game) GetRequiredConfigs() []ConfigVar {
	var required []ConfigVar
	for _, configVar := range g.ConfigVars {
		if configVar.Required {
			required = append(required, configVar)
		}
	}
	return required
}
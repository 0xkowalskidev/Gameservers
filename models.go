package main

import (
	"io"
	"time"
)

type PortMapping struct {
	Name         string `json:"name"`         // "game", "rcon", "query", etc.
	Protocol     string `json:"protocol"`     // "tcp" or "udp"
	ContainerPort int    `json:"container_port"`
	HostPort     int    `json:"host_port"`    // 0 = auto-assign
}

type GameserverStatus string

const (
	StatusStopped GameserverStatus = "stopped"
	StatusStarting GameserverStatus = "starting"
	StatusRunning GameserverStatus = "running"
	StatusStopping GameserverStatus = "stopping"
	StatusError   GameserverStatus = "error"
)

type Game struct {
	ID           string         `json:"id"`
	Name         string         `json:"name"`
	Image        string         `json:"image"`
	PortMappings []PortMapping  `json:"port_mappings"`
	CreatedAt    time.Time      `json:"created_at"`
	UpdatedAt    time.Time      `json:"updated_at"`
}

type Gameserver struct {
	ID           string            `json:"id"`
	Name         string            `json:"name"`
	GameID       string            `json:"game_id"`
	ContainerID  string            `json:"container_id,omitempty"`
	Status       GameserverStatus  `json:"status"`
	PortMappings []PortMapping     `json:"port_mappings"`
	MemoryMB     int               `json:"memory_mb"`    // Memory limit in MB
	CPUCores     float64           `json:"cpu_cores"`    // CPU cores (0 = unlimited)
	MaxBackups   int               `json:"max_backups"`  // Maximum number of backups to keep (0 = unlimited)
	Environment  []string          `json:"environment,omitempty"`
	Volumes      []string          `json:"volumes,omitempty"`
	CreatedAt    time.Time         `json:"created_at"`
	UpdatedAt    time.Time         `json:"updated_at"`
	
	// Derived fields (not stored in DB)
	GameType     string            `json:"game_type"` // From Game.Name
	Image        string            `json:"image"`     // From Game.Image
	MemoryGB     float64           `json:"memory_gb"` // MemoryMB converted to GB for display
	
	// Volume info (derived field)
	VolumeInfo   *VolumeInfo       `json:"volume_info,omitempty"`
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

// PortAllocator manages port assignments for gameservers
type PortAllocator struct {
	startPort int
	endPort   int
}

func NewPortAllocator() *PortAllocator {
	return &PortAllocator{
		startPort: 25565, // Start from Minecraft's default port
		endPort:   35565, // Allow up to 10,000 ports
	}
}

// AllocatePortsForServer assigns available ports to all zero-valued port mappings
// Port mappings with the same name will get the same host port (for TCP+UDP on same port)
func (pa *PortAllocator) AllocatePortsForServer(server *Gameserver, usedPorts map[int]bool) error {
	// Group port mappings by name to assign same port to same-named mappings
	portGroups := make(map[string]int) // name -> assigned port
	
	for i := range server.PortMappings {
		if server.PortMappings[i].HostPort == 0 {
			portName := server.PortMappings[i].Name
			
			// Check if we already assigned a port for this name
			if assignedPort, exists := portGroups[portName]; exists {
				server.PortMappings[i].HostPort = assignedPort
			} else {
				// Find a new available port for this name
				port, err := pa.findAvailablePort(usedPorts)
				if err != nil {
					return err
				}
				server.PortMappings[i].HostPort = port
				portGroups[portName] = port
				usedPorts[port] = true
			}
		}
	}
	return nil
}

func (pa *PortAllocator) findAvailablePort(usedPorts map[int]bool) (int, error) {
	for port := pa.startPort; port <= pa.endPort; port++ {
		if !usedPorts[port] {
			return port, nil
		}
	}
	return 0, &DatabaseError{
		Op:  "allocate_port",
		Msg: "no available ports in range",
		Err: nil,
	}
}

type VolumeInfo struct {
	Name       string            `json:"name"`
	MountPoint string            `json:"mount_point"`
	Driver     string            `json:"driver"`
	CreatedAt  string            `json:"created_at"`
	Labels     map[string]string `json:"labels"`
}

type TaskType string

const (
	TaskTypeRestart TaskType = "restart"
	TaskTypeBackup  TaskType = "backup"
)

type TaskStatus string

const (
	TaskStatusActive   TaskStatus = "active"
	TaskStatusDisabled TaskStatus = "disabled"
)

type ScheduledTask struct {
	ID           string     `json:"id"`
	GameserverID string     `json:"gameserver_id"`
	Name         string     `json:"name"`
	Type         TaskType   `json:"type"`
	Status       TaskStatus `json:"status"`
	CronSchedule string     `json:"cron_schedule"`
	CreatedAt    time.Time  `json:"created_at"`
	UpdatedAt    time.Time  `json:"updated_at"`
	LastRun      *time.Time `json:"last_run,omitempty"`
	NextRun      *time.Time `json:"next_run,omitempty"`
}


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
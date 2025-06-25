package database

import (
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/rs/zerolog/log"

	"0xkowalskidev/gameservers/models"
	"0xkowalskidev/gameservers/services"
)

// GameserverService wraps DatabaseManager with Docker operations and business logic
type GameserverService struct {
	db            *DatabaseManager
	docker        models.DockerManagerInterface
	portAllocator *models.PortAllocator
}

// NewGameserverService creates a new gameserver service instance
func NewGameserverService(db *DatabaseManager, docker models.DockerManagerInterface) *GameserverService {
	return &GameserverService{
		db:            db,
		docker:        docker,
		portAllocator: models.NewPortAllocator(),
	}
}

// CreateGameserver creates a new gameserver with Docker container integration
func (gss *GameserverService) CreateGameserver(server *models.Gameserver) error {
	now := time.Now()
	server.CreatedAt, server.UpdatedAt, server.Status = now, now, models.StatusStopped
	server.ContainerID = "" // No container created yet

	// Populate derived fields from game
	if err := gss.populateGameFields(server); err != nil {
		return err
	}

	// Get game info for port mappings and validation
	game, err := gss.db.GetGame(server.GameID)
	if err != nil {
		return err
	}

	// Validate required configuration variables
	missingConfigs := game.ValidateEnvironment(server.Environment)
	if len(missingConfigs) > 0 {
		return &models.DatabaseError{
			Op:  "validate_config",
			Msg: fmt.Sprintf("missing required configuration: %v", missingConfigs),
			Err: nil,
		}
	}

	// Basic memory validation - ensure minimum requirements
	if server.MemoryMB < game.MinMemoryMB {
		return &models.DatabaseError{
			Op:  "validate_memory",
			Msg: fmt.Sprintf("memory (%d MB) is below game minimum (%d MB)", server.MemoryMB, game.MinMemoryMB),
			Err: nil,
		}
	}

	// Validate against system memory (only for creation, not updates)
	if err := gss.validateSystemMemory(server); err != nil {
		return err
	}

	// Initialize port mappings from game template if not already set
	if len(server.PortMappings) == 0 {
		server.PortMappings = make([]models.PortMapping, len(game.PortMappings))
		copy(server.PortMappings, game.PortMappings)
	}

	// Allocate ports for the server
	if err := gss.allocatePortsForServer(server); err != nil {
		return err
	}

	// Create the gameserver in database
	if err := gss.db.CreateGameserver(server); err != nil {
		return err
	}

	// Create automatic daily backup task
	backupTask := &models.ScheduledTask{
		GameserverID: server.ID,
		Name:         "Daily Backup",
		Type:         models.TaskTypeBackup,
		Status:       models.TaskStatusActive,
		CronSchedule: "0 2 * * *", // Daily at 2 AM
	}

	if err := gss.CreateScheduledTask(backupTask); err != nil {
		log.Error().Err(err).Str("gameserver_id", server.ID).Msg("Failed to create automatic backup task")
		// Don't fail gameserver creation if backup task creation fails
	} else {
		log.Info().Str("gameserver_id", server.ID).Msg("Created automatic daily backup task")
	}

	return nil
}

// UpdateGameserver updates an existing gameserver
func (gss *GameserverService) UpdateGameserver(server *models.Gameserver) error {
	// Get existing server to preserve certain fields
	existing, err := gss.db.GetGameserver(server.ID)
	if err != nil {
		return err
	}

	// Preserve fields that shouldn't be updated via form
	server.CreatedAt = existing.CreatedAt
	server.ContainerID = existing.ContainerID
	server.Status = existing.Status
	server.UpdatedAt = time.Now()

	// Populate derived fields from game
	if err := gss.populateGameFields(server); err != nil {
		return err
	}

	return gss.db.UpdateGameserver(server)
}

// populateGameFields fills in derived fields from the game configuration
func (gss *GameserverService) populateGameFields(server *models.Gameserver) error {
	game, err := gss.db.GetGame(server.GameID)
	if err != nil {
		return err
	}
	server.GameType = game.Name
	server.Image = game.Image
	server.MemoryGB = float64(server.MemoryMB) / 1024.0

	// Get volume information
	volumeName := fmt.Sprintf("gameservers-%s-data", server.Name)
	if volumeInfo, err := gss.docker.GetVolumeInfo(volumeName); err == nil {
		server.VolumeInfo = volumeInfo
	}

	return nil
}

// allocatePortsForServer finds available ports for all unassigned port mappings
func (gss *GameserverService) allocatePortsForServer(server *models.Gameserver) error {
	// Get all currently used ports from existing gameservers
	servers, err := gss.db.ListGameservers()
	if err != nil {
		return err
	}

	usedPorts := make(map[int]bool)
	for _, existingServer := range servers {
		// Skip the current server if it's being updated
		if existingServer.ID == server.ID {
			continue
		}
		for _, portMapping := range existingServer.PortMappings {
			if portMapping.HostPort > 0 {
				usedPorts[portMapping.HostPort] = true
			}
		}
	}

	// Allocate ports using our port allocator
	return gss.portAllocator.AllocatePortsForServer(server, usedPorts)
}

// StartGameserver starts a gameserver with Docker container creation
func (gss *GameserverService) StartGameserver(id string) error {
	server, err := gss.db.GetGameserver(id)
	if err != nil {
		return err
	}

	// Check if starting this server would exceed system memory
	if err := gss.validateSystemMemoryForStart(server); err != nil {
		return err
	}

	// Populate latest settings from database
	if err := gss.populateGameFields(server); err != nil {
		return err
	}

	// Create new container with latest settings
	if err := gss.docker.CreateContainer(server); err != nil {
		return err
	}

	// Start the new container
	if err := gss.docker.StartContainer(server.ContainerID); err != nil {
		return err
	}

	server.Status = models.StatusStarting
	server.UpdatedAt = time.Now()
	return gss.db.UpdateGameserver(server)
}

// StopGameserver stops a gameserver and removes its container
func (gss *GameserverService) StopGameserver(id string) error {
	server, err := gss.db.GetGameserver(id)
	if err != nil {
		return err
	}

	// Remove container entirely (this stops and removes)
	if server.ContainerID != "" {
		if err := gss.docker.RemoveContainer(server.ContainerID); err != nil {
			return err
		}
		server.ContainerID = "" // Clear container ID since it's gone
	}

	server.Status = models.StatusStopped
	server.UpdatedAt = time.Now()
	return gss.db.UpdateGameserver(server)
}

// RestartGameserver restarts a gameserver by stopping and starting it
func (gss *GameserverService) RestartGameserver(id string) error {
	// Stop first (removes container)
	if err := gss.StopGameserver(id); err != nil {
		return err
	}

	// Then start (creates new container)
	return gss.StartGameserver(id)
}

// SendGameserverCommand sends a command to a running gameserver
func (gss *GameserverService) SendGameserverCommand(id string, command string) error {
	server, err := gss.db.GetGameserver(id)
	if err != nil {
		return err
	}

	if server.ContainerID == "" {
		return &models.DatabaseError{
			Op:  "send_command",
			Msg: "gameserver has no container",
			Err: nil,
		}
	}

	if server.Status != models.StatusRunning {
		return &models.DatabaseError{
			Op:  "send_command",
			Msg: "gameserver is not running",
			Err: nil,
		}
	}

	return gss.docker.SendCommand(server.ContainerID, command)
}

// DeleteGameserver deletes a gameserver and all its data
func (gss *GameserverService) DeleteGameserver(id string) error {
	server, err := gss.db.GetGameserver(id)
	if err != nil {
		return err
	}

	// Remove container if it exists
	if server.ContainerID != "" {
		gss.docker.RemoveContainer(server.ContainerID)
	}

	// Remove the auto-managed volume (this will delete all data!)
	volumeName := fmt.Sprintf("gameservers-%s-data", server.Name)
	if err := gss.docker.RemoveVolume(volumeName); err != nil {
		log.Warn().Err(err).Str("volume", volumeName).Msg("Failed to remove volume, may not exist")
	}

	return gss.db.DeleteGameserver(id)
}

// syncStatus synchronizes the gameserver status with Docker container status
func (gss *GameserverService) syncStatus(server *models.Gameserver) {
	if server.ContainerID != "" {
		if dockerStatus, err := gss.docker.GetContainerStatus(server.ContainerID); err == nil && server.Status != dockerStatus {
			server.Status, server.UpdatedAt = dockerStatus, time.Now()
			gss.db.UpdateGameserver(server)
		}
	}
}

// GetGameserver retrieves a gameserver with populated fields and synced status
func (gss *GameserverService) GetGameserver(id string) (*models.Gameserver, error) {
	server, err := gss.db.GetGameserver(id)
	if err != nil {
		return nil, err
	}
	gss.populateGameFields(server)
	gss.syncStatus(server)
	return server, nil
}

// ListGameservers retrieves all gameservers with populated fields and synced status
func (gss *GameserverService) ListGameservers() ([]*models.Gameserver, error) {
	servers, err := gss.db.ListGameservers()
	if err != nil {
		return nil, err
	}
	for _, server := range servers {
		gss.populateGameFields(server)
		gss.syncStatus(server)
	}
	return servers, nil
}

// StreamGameserverLogs returns a stream of gameserver logs
func (gss *GameserverService) StreamGameserverLogs(id string) (io.ReadCloser, error) {
	server, err := gss.db.GetGameserver(id)
	if err != nil {
		return nil, err
	}
	if server.ContainerID == "" {
		return nil, &models.DatabaseError{Op: "stream_logs", Msg: "container not created yet", Err: nil}
	}
	return gss.docker.StreamContainerLogs(server.ContainerID)
}

// StreamGameserverStats returns a stream of gameserver statistics
func (gss *GameserverService) StreamGameserverStats(id string) (io.ReadCloser, error) {
	server, err := gss.db.GetGameserver(id)
	if err != nil {
		return nil, err
	}
	if server.ContainerID == "" {
		return nil, &models.DatabaseError{Op: "stream_stats", Msg: "container not created yet", Err: nil}
	}
	return gss.docker.StreamContainerStats(server.ContainerID)
}

// ListGames returns all available games
func (gss *GameserverService) ListGames() ([]*models.Game, error) {
	return gss.db.ListGames()
}

// GetGame returns a specific game by ID
func (gss *GameserverService) GetGame(id string) (*models.Game, error) {
	return gss.db.GetGame(id)
}

// CreateGame creates a new game configuration
func (gss *GameserverService) CreateGame(game *models.Game) error {
	now := time.Now()
	game.CreatedAt, game.UpdatedAt = now, now
	return gss.db.CreateGame(game)
}

// Scheduled Task Service Operations

// CreateScheduledTask creates a new scheduled task
func (gss *GameserverService) CreateScheduledTask(task *models.ScheduledTask) error {
	now := time.Now()
	task.CreatedAt, task.UpdatedAt = now, now
	task.ID = models.GenerateID()

	// Calculate initial next run time
	nextRun := services.CalculateNextRun(task.CronSchedule, now)
	if !nextRun.IsZero() {
		task.NextRun = &nextRun
	}

	return gss.db.CreateScheduledTask(task)
}

// GetScheduledTask retrieves a scheduled task by ID
func (gss *GameserverService) GetScheduledTask(id string) (*models.ScheduledTask, error) {
	return gss.db.GetScheduledTask(id)
}

// UpdateScheduledTask updates an existing scheduled task
func (gss *GameserverService) UpdateScheduledTask(task *models.ScheduledTask) error {
	task.UpdatedAt = time.Now()
	// Clear next run time so scheduler will recalculate it
	task.NextRun = nil
	return gss.db.UpdateScheduledTask(task)
}

// DeleteScheduledTask deletes a scheduled task
func (gss *GameserverService) DeleteScheduledTask(id string) error {
	return gss.db.DeleteScheduledTask(id)
}

// ListScheduledTasksForGameserver retrieves all scheduled tasks for a gameserver
func (gss *GameserverService) ListScheduledTasksForGameserver(gameserverID string) ([]*models.ScheduledTask, error) {
	return gss.db.ListScheduledTasksForGameserver(gameserverID)
}

// CreateGameserverBackup creates a backup of a gameserver
func (gss *GameserverService) CreateGameserverBackup(gameserverID string) error {
	gameserver, err := gss.db.GetGameserver(gameserverID)
	if err != nil {
		return err
	}

	// Create backup
	err = gss.docker.CreateBackup(gameserver.ContainerID, gameserver.Name)
	if err != nil {
		return err
	}

	// Clean up old backups if max_backups is set
	err = gss.docker.CleanupOldBackups(gameserver.ContainerID, gameserver.MaxBackups)
	if err != nil {
		log.Error().Err(err).Str("gameserver_id", gameserverID).Msg("Failed to cleanup old backups")
		// Don't return error for cleanup failure, backup creation was successful
	}

	return nil
}

// RestoreGameserverBackup restores a gameserver from a backup
func (gss *GameserverService) RestoreGameserverBackup(gameserverID, backupFilename string) error {
	gameserver, err := gss.db.GetGameserver(gameserverID)
	if err != nil {
		return err
	}
	return gss.docker.RestoreBackup(gameserver.ContainerID, backupFilename)
}

// File operation methods

// ListFiles lists files in a gameserver container
func (gss *GameserverService) ListFiles(containerID string, path string) ([]*models.FileInfo, error) {
	return gss.docker.ListFiles(containerID, path)
}

// ReadFile reads a file from a gameserver container
func (gss *GameserverService) ReadFile(containerID string, path string) ([]byte, error) {
	return gss.docker.ReadFile(containerID, path)
}

// WriteFile writes a file to a gameserver container
func (gss *GameserverService) WriteFile(containerID string, path string, content []byte) error {
	return gss.docker.WriteFile(containerID, path, content)
}

// CreateDirectory creates a directory in a gameserver container
func (gss *GameserverService) CreateDirectory(containerID string, path string) error {
	return gss.docker.CreateDirectory(containerID, path)
}

// DeletePath deletes a file or directory in a gameserver container
func (gss *GameserverService) DeletePath(containerID string, path string) error {
	return gss.docker.DeletePath(containerID, path)
}

// DownloadFile downloads a file from a gameserver container
func (gss *GameserverService) DownloadFile(containerID string, path string) (io.ReadCloser, error) {
	return gss.docker.DownloadFile(containerID, path)
}

// RenameFile renames a file in a gameserver container
func (gss *GameserverService) RenameFile(containerID string, oldPath string, newPath string) error {
	return gss.docker.RenameFile(containerID, oldPath, newPath)
}

// UploadFile uploads a file to a gameserver container
func (gss *GameserverService) UploadFile(containerID string, destPath string, reader io.Reader) error {
	return gss.docker.UploadFile(containerID, destPath, reader)
}

// ListGameserverBackups lists all backup files for a gameserver
func (gss *GameserverService) ListGameserverBackups(gameserverID string) ([]*models.FileInfo, error) {
	gameserver, err := gss.db.GetGameserver(gameserverID)
	if err != nil {
		return nil, err
	}

	// List files in /data/backups and filter for .tar.gz files
	files, err := gss.docker.ListFiles(gameserver.ContainerID, "/data/backups")
	if err != nil {
		return nil, err
	}

	// Filter for backup files
	var backups []*models.FileInfo
	for _, file := range files {
		if !file.IsDir && strings.HasSuffix(strings.ToLower(file.Name), ".tar.gz") {
			backups = append(backups, file)
		}
	}

	return backups, nil
}

// validateSystemMemory checks if the server's memory requirements fit within available system memory
func (gss *GameserverService) validateSystemMemory(server *models.Gameserver) error {
	systemInfo, err := models.GetSystemInfo()
	if err != nil {
		log.Warn().Err(err).Msg("Could not get system memory info, skipping validation")
		return nil // Don't fail if we can't get system info
	}

	if server.MemoryMB > systemInfo.TotalMemoryMB {
		return &models.DatabaseError{
			Op:  "validate_memory",
			Msg: fmt.Sprintf("server memory (%d MB) exceeds total system memory (%d MB)", 
				server.MemoryMB, systemInfo.TotalMemoryMB),
			Err: nil,
		}
	}

	return nil
}

// validateSystemMemoryForStart checks if starting this server would exceed available system memory
func (gss *GameserverService) validateSystemMemoryForStart(server *models.Gameserver) error {
	systemInfo, err := models.GetSystemInfo()
	if err != nil {
		log.Warn().Err(err).Msg("Could not get system memory info, skipping validation")
		return nil // Don't fail if we can't get system info
	}

	// Get all currently running servers
	servers, err := gss.db.ListGameservers()
	if err != nil {
		return &models.DatabaseError{
			Op:  "validate_memory",
			Msg: "failed to check existing memory usage",
			Err: err,
		}
	}

	// Calculate current memory usage from running servers only
	currentMemoryUsage := 0
	for _, existingServer := range servers {
		// Only count running servers (starting servers will become running)
		if existingServer.Status == models.StatusRunning || existingServer.Status == models.StatusStarting {
			currentMemoryUsage += existingServer.MemoryMB
		}
	}

	// Check if starting this server would exceed total system memory
	if currentMemoryUsage+server.MemoryMB > systemInfo.TotalMemoryMB {
		return &models.DatabaseError{
			Op:  "validate_memory",
			Msg: fmt.Sprintf("starting server would exceed total system memory: %d MB (running) + %d MB (new) = %d MB > %d MB total", 
				currentMemoryUsage, server.MemoryMB, currentMemoryUsage+server.MemoryMB, systemInfo.TotalMemoryMB),
			Err: nil,
		}
	}

	return nil
}


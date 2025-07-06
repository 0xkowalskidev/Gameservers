package services

import (
	"context"
	"fmt"
	"io"
	"path/filepath"
	"strings"
	"time"

	"github.com/0xkowalskidev/gameserverquery/protocol"
	"github.com/0xkowalskidev/gameserverquery/query"
	"github.com/rs/zerolog/log"

	"0xkowalskidev/gameservers/models"
)

// Local error types for services package
type serviceError struct {
	Status  int
	Message string
	Cause   error
}

func (e *serviceError) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("%s: %v", e.Message, e.Cause)
	}
	return e.Message
}

// Local error helpers
func badRequest(format string, args ...interface{}) error {
	return &serviceError{Status: 400, Message: fmt.Sprintf(format, args...)}
}

func notFound(resource string) error {
	return &serviceError{Status: 404, Message: fmt.Sprintf("%s not found", resource)}
}

func internalError(cause error, message string) error {
	return &serviceError{Status: 500, Message: message, Cause: cause}
}

// GameserverServiceInterface defines the business logic layer for gameserver operations
type GameserverServiceInterface interface {
	// CRUD operations (no context needed for simple operations)
	CreateGameserver(req CreateGameserverRequest) (*models.Gameserver, error)
	GetGameserver(id string) (*models.Gameserver, error)
	ListGameservers() ([]*models.Gameserver, error)
	UpdateGameserver(id string, req UpdateGameserverRequest) error
	DeleteGameserver(id string) error
	
	// Container operations (no context needed for simple start/stop)
	StartGameserver(id string) error
	StopGameserver(id string) error
	RestartGameserver(id string) error
	SendGameserverCommand(id string, command string) error
	
	// Status and monitoring (no context needed for simple status check)
	GetGameserverStatus(id string) (models.GameserverStatus, error)
	
	// Operations that benefit from context (queries, streaming, long operations)
	GetGameserverQuery(ctx context.Context, id string) (*protocol.ServerInfo, error)
	StreamGameserverLogs(ctx context.Context, id string) (io.ReadCloser, error)
	StreamGameserverStats(ctx context.Context, id string) (io.ReadCloser, error)
	
	// Background operations (context for cancellation)
	ExecuteScheduledTask(ctx context.Context, task *models.ScheduledTask) error
	CreateBackup(ctx context.Context, gameserverID string, name string) error
	FileOperation(ctx context.Context, gameserverID string, path string, op func(string, string) error) error
}

// CreateGameserverRequest represents a request to create a new gameserver
type CreateGameserverRequest struct {
	Name     string
	GameID   string
	Port     int
	MemoryMB int
	CPUCores float64
}

// UpdateGameserverRequest represents a request to update a gameserver
type UpdateGameserverRequest struct {
	Name     string
	Port     int
	MemoryMB int
}

// GameserverService handles business logic for gameserver operations
type GameserverService struct {
	db            models.DatabaseInterface
	gameService   models.GameServiceInterface
	docker        models.DockerManagerInterface
	basePath      string
	portAllocator *models.PortAllocator
}

// NewGameserverService creates a new service instance
func NewGameserverService(db models.DatabaseInterface, gameService models.GameServiceInterface, docker models.DockerManagerInterface, basePath string) *GameserverService {
	return &GameserverService{
		db:            db,
		gameService:   gameService,
		docker:        docker,
		basePath:      basePath,
		portAllocator: models.NewPortAllocator(),
	}
}


// populateGameFields fills in derived fields from the game configuration
func (s *GameserverService) populateGameFields(server *models.Gameserver) error {
	game, err := s.gameService.GetGame(server.GameID)
	if err != nil {
		return err
	}
	server.GameType = game.Name
	server.Image = game.Image
	server.IconPath = game.IconPath
	server.MemoryGB = float64(server.MemoryMB) / 1024.0

	// Get volume information
	volumeName := fmt.Sprintf("gameservers-%s-data", server.Name)
	if volumeInfo, err := s.docker.GetVolumeInfo(volumeName); err == nil {
		server.VolumeInfo = volumeInfo
	}

	return nil
}

// allocatePortsForServer finds available ports for all unassigned port mappings
func (s *GameserverService) allocatePortsForServer(server *models.Gameserver) error {
	// Get all currently used ports from existing gameservers
	servers, err := s.db.ListGameservers()
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
	return s.portAllocator.AllocatePortsForServer(server, usedPorts)
}

// validateSystemMemory checks if the server's memory requirements fit within available system memory
func (s *GameserverService) validateSystemMemory(server *models.Gameserver) error {
	systemInfo, err := models.GetSystemInfo()
	if err != nil {
		log.Warn().Err(err).Msg("Could not get system memory info, skipping validation")
		return nil // Don't fail if we can't get system info
	}

	if server.MemoryMB > systemInfo.TotalMemoryMB {
		return &serviceError{
			Status: 400,
			Message: fmt.Sprintf("server memory (%d MB) exceeds total system memory (%d MB)",
				server.MemoryMB, systemInfo.TotalMemoryMB),
			Cause: nil,
		}
	}

	return nil
}

// validateSystemMemoryForStart checks if starting this server would exceed available system memory
func (s *GameserverService) validateSystemMemoryForStart(server *models.Gameserver) error {
	systemInfo, err := models.GetSystemInfo()
	if err != nil {
		log.Warn().Err(err).Msg("Could not get system memory info, skipping validation")
		return nil // Don't fail if we can't get system info
	}

	// Get all currently running servers
	servers, err := s.db.ListGameservers()
	if err != nil {
		return &serviceError{
			Status: 500,
			Message: "failed to check existing memory usage",
			Cause: err,
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
		return &serviceError{
			Status: 400,
			Message: fmt.Sprintf("starting server would exceed total system memory: %d MB (running) + %d MB (new) = %d MB > %d MB total",
				currentMemoryUsage, server.MemoryMB, currentMemoryUsage+server.MemoryMB, systemInfo.TotalMemoryMB),
			Cause: nil,
		}
	}

	return nil
}

// CreateGameserver creates a new gameserver with Docker container
func (s *GameserverService) CreateGameserver(req CreateGameserverRequest) (*models.Gameserver, error) {
	// Validate input
	if req.Name == "" || req.GameID == "" {
		return nil, badRequest("Name and Game ID are required")
	}

	// Get game configuration
	game, err := s.gameService.GetGame(req.GameID)
	if err != nil {
		return nil, notFound("game")
	}

	// Create database record with port mappings from game template
	gs := &models.Gameserver{
		Name:         req.Name,
		GameID:       req.GameID,
		PortMappings: make([]models.PortMapping, len(game.PortMappings)),
		MemoryMB:     req.MemoryMB,
		CPUCores:     req.CPUCores,
		Status:       models.StatusStopped,
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}

	// Copy port mappings from game template
	copy(gs.PortMappings, game.PortMappings)

	if req.MemoryMB <= 0 {
		gs.MemoryMB = 1024 // Default 1GB
	}

	// Validate required configuration variables
	missingConfigs := game.ValidateEnvironment(gs.Environment)
	if len(missingConfigs) > 0 {
		return nil, badRequest("missing required configuration: %v", missingConfigs)
	}

	// Basic memory validation - ensure minimum requirements
	if gs.MemoryMB < game.MinMemoryMB {
		return nil, badRequest("memory (%d MB) is below game minimum (%d MB)", gs.MemoryMB, game.MinMemoryMB)
	}

	// Validate against system memory (only for creation, not updates)
	if err := s.validateSystemMemory(gs); err != nil {
		return nil, err
	}

	// Initialize port mappings from game template if not already set
	if len(gs.PortMappings) == 0 {
		gs.PortMappings = make([]models.PortMapping, len(game.PortMappings))
		copy(gs.PortMappings, game.PortMappings)
	}

	// Allocate ports for the server
	if err := s.allocatePortsForServer(gs); err != nil {
		return nil, err
	}

	// Populate derived fields from game
	if err := s.populateGameFields(gs); err != nil {
		return nil, err
	}

	// Create gameserver record
	if err = s.db.CreateGameserver(gs); err != nil {
		return nil, internalError(err, "Failed to create gameserver")
	}

	log.Info().Str("name", gs.Name).Str("game", gs.GameID).Msg("Gameserver created")
	return gs, nil
}

// GetGameserver retrieves a gameserver by ID
func (s *GameserverService) GetGameserver(id string) (*models.Gameserver, error) {
	gs, err := s.db.GetGameserver(id)
	if err != nil {
		return nil, notFound("gameserver")
	}

	// Populate derived fields and sync status
	if err := s.populateGameFields(gs); err != nil {
		return nil, err
	}

	// Sync status with Docker
	if gs.ContainerID != "" {
		if dockerStatus, err := s.docker.GetContainerStatus(gs.ContainerID); err == nil && gs.Status != dockerStatus {
			gs.Status = dockerStatus
			gs.UpdatedAt = time.Now()
			s.db.UpdateGameserver(gs)
		}
	}

	return gs, nil
}

// updateGameserverStatus updates the status of a gameserver
func (s *GameserverService) updateGameserverStatus(gs *models.Gameserver, status models.GameserverStatus, action string) error {
	gs.Status = status
	gs.UpdatedAt = time.Now()
	if err := s.db.UpdateGameserver(gs); err != nil {
		return internalError(err, "Failed to update status")
	}
	log.Info().Str("id", gs.ID).Str("name", gs.Name).Msgf("Gameserver %s", action)
	return nil
}

// StartGameserver starts a gameserver container
func (s *GameserverService) StartGameserver(id string) error {
	gs, err := s.db.GetGameserver(id)
	if err != nil {
		return notFound("gameserver")
	}

	// Check if starting this server would exceed system memory
	if err := s.validateSystemMemoryForStart(gs); err != nil {
		return err
	}

	// Populate latest settings from database
	if err := s.populateGameFields(gs); err != nil {
		return err
	}

	// Create new container with latest settings
	if err := s.docker.CreateContainer(gs); err != nil {
		return internalError(err, "Failed to create container")
	}

	// Start the new container
	if err := s.docker.StartContainer(gs.ContainerID); err != nil {
		return internalError(err, "Failed to start container")
	}

	return s.updateGameserverStatus(gs, models.StatusStarting, "started")
}

// StopGameserver stops a gameserver container
func (s *GameserverService) StopGameserver(id string) error {
	gs, err := s.db.GetGameserver(id)
	if err != nil {
		return notFound("gameserver")
	}

	// Remove container entirely (this stops and removes)
	if gs.ContainerID != "" {
		if err := s.docker.RemoveContainer(gs.ContainerID); err != nil {
			return internalError(err, "Failed to remove container")
		}
		gs.ContainerID = "" // Clear container ID since it's gone
	}

	return s.updateGameserverStatus(gs, models.StatusStopped, "stopped")
}

// RestartGameserver restarts a gameserver container
func (s *GameserverService) RestartGameserver(id string) error {
	if err := s.StopGameserver(id); err != nil {
		return err
	}
	time.Sleep(2 * time.Second) // Brief pause
	return s.StartGameserver(id)
}

// DeleteGameserver deletes a gameserver and its container
func (s *GameserverService) DeleteGameserver(id string) error {
	gs, err := s.db.GetGameserver(id)
	if err != nil {
		return notFound("gameserver")
	}

	// Remove container if it exists
	if gs.ContainerID != "" {
		s.docker.RemoveContainer(gs.ContainerID)
	}

	// Remove the auto-managed volume (this will delete all data!)
	volumeName := fmt.Sprintf("gameservers-%s-data", gs.Name)
	if err := s.docker.RemoveVolume(volumeName); err != nil {
		log.Warn().Err(err).Str("volume", volumeName).Msg("Failed to remove volume, may not exist")
	}

	// Delete from database
	if err := s.db.DeleteGameserver(id); err != nil {
		return internalError(err, "Failed to delete gameserver")
	}

	log.Info().Str("id", id).Str("name", gs.Name).Msg("Gameserver deleted")
	return nil
}

// UpdateGameserver updates gameserver configuration
func (s *GameserverService) UpdateGameserver(id string, req UpdateGameserverRequest) error {
	gs, err := s.db.GetGameserver(id)
	if err != nil {
		return notFound("gameserver")
	}

	// Get existing server to preserve certain fields
	existing, err := s.db.GetGameserver(id)
	if err != nil {
		return internalError(err, "Failed to get existing gameserver")
	}

	// Update fields
	if req.Name != "" {
		gs.Name = req.Name
	}
	if req.MemoryMB > 0 {
		gs.MemoryMB = req.MemoryMB
	}

	// Preserve fields that shouldn't be updated via form
	gs.CreatedAt = existing.CreatedAt
	gs.ContainerID = existing.ContainerID
	gs.Status = existing.Status
	gs.UpdatedAt = time.Now()

	// Populate derived fields from game
	if err := s.populateGameFields(gs); err != nil {
		return internalError(err, "Failed to populate game fields")
	}

	if err := s.db.UpdateGameserver(gs); err != nil {
		return internalError(err, "Failed to update gameserver")
	}

	return nil
}

// ExecuteScheduledTask executes a scheduled task
func (s *GameserverService) ExecuteScheduledTask(ctx context.Context, task *models.ScheduledTask) error {
	log.Info().Str("task_id", task.ID).Str("task_name", task.Name).Str("type", string(task.Type)).Msg("Executing scheduled task")

	gameserver, err := s.db.GetGameserver(task.GameserverID)
	if err != nil {
		log.Error().Err(err).Str("gameserver_id", task.GameserverID).Msg("Gameserver not found, skipping task")
		return err
	}

	switch task.Type {
	case models.TaskTypeRestart:
		// Only restart if the server is currently running
		if gameserver.Status != models.StatusRunning {
			log.Info().
				Str("gameserver_id", task.GameserverID).
				Str("status", string(gameserver.Status)).
				Msg("Skipping restart - gameserver not running")
			return nil
		}
		return s.RestartGameserver(task.GameserverID)

	case models.TaskTypeBackup:
		// Backups can happen regardless of server status
		log.Info().
			Str("gameserver_id", task.GameserverID).
			Str("status", string(gameserver.Status)).
			Msg("Executing scheduled backup")
		return s.CreateBackup(ctx, task.GameserverID, "")

	default:
		return badRequest("Unknown task type: %s", string(task.Type))
	}
}

// CreateBackup creates a backup of gameserver files
func (s *GameserverService) CreateBackup(ctx context.Context, gameserverID string, name string) error {
	gs, err := s.db.GetGameserver(gameserverID)
	if err != nil {
		return notFound("gameserver")
	}

	// Generate backup filename
	if name == "" {
		name = fmt.Sprintf("backup-%s", time.Now().Format("20060102-150405"))
	}

	// Create backup
	if err := s.docker.CreateBackup(gs.ContainerID, gs.Name); err != nil {
		return internalError(err, "Failed to create backup")
	}

	// Clean up old backups if max_backups is set
	if err := s.docker.CleanupOldBackups(gs.ContainerID, gs.MaxBackups); err != nil {
		log.Error().Err(err).Str("gameserver_id", gameserverID).Msg("Failed to cleanup old backups")
		// Don't return error for cleanup failure, backup creation was successful
	}

	log.Info().Str("gameserver", gameserverID).Str("name", name).Msg("Backup created")
	return nil
}

// ListGameservers retrieves all gameservers with populated fields and synced status
func (s *GameserverService) ListGameservers() ([]*models.Gameserver, error) {
	servers, err := s.db.ListGameservers()
	if err != nil {
		return nil, internalError(err, "Failed to list gameservers")
	}

	for _, server := range servers {
		if err := s.populateGameFields(server); err != nil {
			log.Error().Err(err).Str("gameserver_id", server.ID).Msg("Failed to populate game fields")
		}

		// Sync status with Docker
		if server.ContainerID != "" {
			if dockerStatus, err := s.docker.GetContainerStatus(server.ContainerID); err == nil && server.Status != dockerStatus {
				server.Status = dockerStatus
				server.UpdatedAt = time.Now()
				s.db.UpdateGameserver(server)
			}
		}
	}

	return servers, nil
}

// SendGameserverCommand sends a command to a running gameserver
func (s *GameserverService) SendGameserverCommand(id string, command string) error {
	gs, err := s.db.GetGameserver(id)
	if err != nil {
		return notFound("gameserver")
	}

	if gs.ContainerID == "" {
		return badRequest("gameserver has no container")
	}

	if gs.Status != models.StatusRunning {
		return badRequest("gameserver is not running")
	}

	return s.docker.SendCommand(gs.ContainerID, command)
}

// GetGameserverStatus returns the current Docker status of a gameserver
func (s *GameserverService) GetGameserverStatus(id string) (models.GameserverStatus, error) {
	gs, err := s.db.GetGameserver(id)
	if err != nil {
		return "", notFound("gameserver")
	}

	// Check actual Docker container status
	status, err := s.docker.GetContainerStatus(gs.ContainerID)
	if err != nil {
		log.Warn().Err(err).Str("gameserver_id", id).Msg("Failed to get container status, using database status")
		return gs.Status, nil
	}

	// Update database if status has changed
	if status != gs.Status {
		gs.Status = status
		gs.UpdatedAt = time.Now()
		if err := s.db.UpdateGameserver(gs); err != nil {
			log.Error().Err(err).Str("gameserver_id", id).Msg("Failed to update gameserver status")
		}
	}

	return status, nil
}

// GetGameserverQuery performs a protocol query on the gameserver using gameserverquery package
func (s *GameserverService) GetGameserverQuery(ctx context.Context, id string) (*protocol.ServerInfo, error) {
	gs, err := s.db.GetGameserver(id)
	if err != nil {
		return nil, notFound("gameserver")
	}

	// Get the game to determine protocol
	game, err := s.gameService.GetGame(gs.GameID)
	if err != nil {
		log.Error().Err(err).Str("game_id", gs.GameID).Msg("Failed to get game info for query")
		return nil, err
	}

	// Find the query port for this gameserver
	var queryPort int
	for _, mapping := range gs.PortMappings {
		if mapping.Name == "query" {
			queryPort = mapping.HostPort
			break
		}
	}

	if queryPort == 0 {
		// If no specific query port, try the game/main port
		for _, mapping := range gs.PortMappings {
			if mapping.Name == "game" || mapping.Name == "main" || mapping.Name == "server" {
				queryPort = mapping.HostPort
				break
			}
		}
	}

	if queryPort == 0 {
		// Fall back to the first available port
		if len(gs.PortMappings) > 0 {
			queryPort = gs.PortMappings[0].HostPort
		}
	}

	if queryPort == 0 {
		return nil, fmt.Errorf("no ports available for query")
	}

	// Build query address
	address := fmt.Sprintf("localhost:%d", queryPort)

	// Query the server using the gameserverquery package
	info, err := query.Query(ctx, game.Slug, address)
	if err != nil {
		log.Debug().Err(err).Str("gameserver_id", id).Str("address", address).Msg("Failed to query gameserver")
		return nil, err
	}

	return info, nil
}

// StreamGameserverLogs returns a stream of gameserver logs
func (s *GameserverService) StreamGameserverLogs(ctx context.Context, id string) (io.ReadCloser, error) {
	gs, err := s.db.GetGameserver(id)
	if err != nil {
		return nil, notFound("gameserver")
	}
	
	if gs.ContainerID == "" {
		return nil, badRequest("gameserver has no container")
	}
	
	return s.docker.StreamContainerLogs(gs.ContainerID)
}

// StreamGameserverStats returns a stream of gameserver statistics
func (s *GameserverService) StreamGameserverStats(ctx context.Context, id string) (io.ReadCloser, error) {
	gs, err := s.db.GetGameserver(id)
	if err != nil {
		return nil, notFound("gameserver")
	}
	
	if gs.ContainerID == "" {
		return nil, badRequest("gameserver has no container")
	}
	
	return s.docker.StreamContainerStats(gs.ContainerID)
}

// FileOperation handles file operations with path validation
func (s *GameserverService) FileOperation(ctx context.Context, gameserverID string, path string, op func(string, string) error) error {
	gs, err := s.db.GetGameserver(gameserverID)
	if err != nil {
		return notFound("gameserver")
	}

	// Validate and clean path
	cleanPath := filepath.Clean(path)
	if strings.Contains(cleanPath, "..") {
		return badRequest("Invalid path")
	}

	// Execute operation
	if err := op(gs.ContainerID, cleanPath); err != nil {
		return internalError(err, "File operation failed")
	}

	return nil
}

package services

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/rs/zerolog/log"

	"0xkowalskidev/gameservers/models"
)

// HTTPError represents an HTTP error with status code
type HTTPError struct {
	Status  int
	Message string
	Cause   error
}

func (e *HTTPError) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("%s: %v", e.Message, e.Cause)
	}
	return e.Message
}

// Error helpers
func BadRequest(format string, args ...interface{}) error {
	return &HTTPError{Status: 400, Message: fmt.Sprintf(format, args...)}
}

func NotFound(resource string) error {
	return &HTTPError{Status: 404, Message: fmt.Sprintf("%s not found", resource)}
}

func InternalError(cause error, message string) error {
	return &HTTPError{Status: 500, Message: message, Cause: cause}
}

// GameserverServiceInterface defines the business logic layer for gameserver operations
type GameserverServiceInterface interface {
	CreateGameserver(ctx context.Context, req CreateGameserverRequest) (*models.Gameserver, error)
	GetGameserver(ctx context.Context, id string) (*models.Gameserver, error)
	UpdateGameserver(ctx context.Context, id string, req UpdateGameserverRequest) error
	DeleteGameserver(ctx context.Context, id string) error
	StartGameserver(ctx context.Context, id string) error
	StopGameserver(ctx context.Context, id string) error
	RestartGameserver(ctx context.Context, id string) error
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
	db       models.GameserverServiceInterface
	docker   models.DockerManagerInterface
	basePath string
}

// NewGameserverService creates a new service instance
func NewGameserverService(db models.GameserverServiceInterface, docker models.DockerManagerInterface, basePath string) *GameserverService {
	return &GameserverService{db: db, docker: docker, basePath: basePath}
}

// getGameserverOrError is a helper to reduce repetitive error handling
func (s *GameserverService) getGameserverOrError(id string) (*models.Gameserver, error) {
	gs, err := s.db.GetGameserver(id)
	if err != nil {
		return nil, NotFound("gameserver")
	}
	return gs, nil
}

// CreateGameserver creates a new gameserver with Docker container
func (s *GameserverService) CreateGameserver(ctx context.Context, req CreateGameserverRequest) (*models.Gameserver, error) {
	// Validate input
	if req.Name == "" || req.GameID == "" {
		return nil, BadRequest("Name and Game ID are required")
	}

	// Get game configuration
	game, err := s.db.GetGame(req.GameID)
	if err != nil {
		return nil, NotFound("game")
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

	// Create gameserver record
	if err = s.db.CreateGameserver(gs); err != nil {
		return nil, InternalError(err, "Failed to create gameserver")
	}

	// Create Docker container
	if err = s.docker.CreateContainer(gs); err != nil {
		// Rollback database changes
		s.db.DeleteGameserver(gs.ID)
		return nil, InternalError(err, "Failed to create container")
	}

	log.Info().Str("name", gs.Name).Str("game", gs.GameID).Msg("Gameserver created")
	return gs, nil
}

// GetGameserver retrieves a gameserver by ID
func (s *GameserverService) GetGameserver(ctx context.Context, id string) (*models.Gameserver, error) {
	return s.getGameserverOrError(id)
}

// updateGameserverStatus updates the status of a gameserver
func (s *GameserverService) updateGameserverStatus(gs *models.Gameserver, status models.GameserverStatus, action string) error {
	gs.Status = status
	gs.UpdatedAt = time.Now()
	if err := s.db.UpdateGameserver(gs); err != nil {
		return InternalError(err, "Failed to update status")
	}
	log.Info().Str("id", gs.ID).Str("name", gs.Name).Msgf("Gameserver %s", action)
	return nil
}

// StartGameserver starts a gameserver container
func (s *GameserverService) StartGameserver(ctx context.Context, id string) error {
	gs, err := s.getGameserverOrError(id)
	if err != nil {
		return err
	}

	if err := s.docker.StartContainer(gs.ContainerID); err != nil {
		return InternalError(err, "Failed to start container")
	}

	return s.updateGameserverStatus(gs, models.StatusRunning, "started")
}

// StopGameserver stops a gameserver container
func (s *GameserverService) StopGameserver(ctx context.Context, id string) error {
	gs, err := s.getGameserverOrError(id)
	if err != nil {
		return err
	}

	if err := s.docker.StopContainer(gs.ContainerID); err != nil {
		return InternalError(err, "Failed to stop container")
	}

	return s.updateGameserverStatus(gs, models.StatusStopped, "stopped")
}

// RestartGameserver restarts a gameserver container
func (s *GameserverService) RestartGameserver(ctx context.Context, id string) error {
	if err := s.StopGameserver(ctx, id); err != nil {
		return err
	}
	time.Sleep(2 * time.Second) // Brief pause
	return s.StartGameserver(ctx, id)
}

// DeleteGameserver deletes a gameserver and its container
func (s *GameserverService) DeleteGameserver(ctx context.Context, id string) error {
	gs, err := s.getGameserverOrError(id)
	if err != nil {
		return err
	}

	// Stop container if running
	if gs.Status == models.StatusRunning {
		s.docker.StopContainer(gs.ContainerID)
	}

	// Remove container
	if err := s.docker.RemoveContainer(gs.ContainerID); err != nil {
		log.Warn().Err(err).Msg("Failed to remove container")
	}

	// Delete from database
	if err := s.db.DeleteGameserver(id); err != nil {
		return InternalError(err, "Failed to delete gameserver")
	}

	log.Info().Str("id", id).Str("name", gs.Name).Msg("Gameserver deleted")
	return nil
}

// UpdateGameserver updates gameserver configuration
func (s *GameserverService) UpdateGameserver(ctx context.Context, id string, req UpdateGameserverRequest) error {
	gs, err := s.getGameserverOrError(id)
	if err != nil {
		return err
	}

	// Update fields
	if req.Name != "" {
		gs.Name = req.Name
	}
	if req.MemoryMB > 0 {
		gs.MemoryMB = req.MemoryMB
	}

	gs.UpdatedAt = time.Now()

	if err := s.db.UpdateGameserver(gs); err != nil {
		return InternalError(err, "Failed to update gameserver")
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
		return s.RestartGameserver(ctx, task.GameserverID)
		
	case models.TaskTypeBackup:
		// Backups can happen regardless of server status
		log.Info().
			Str("gameserver_id", task.GameserverID).
			Str("status", string(gameserver.Status)).
			Msg("Executing scheduled backup")
		return s.CreateBackup(ctx, task.GameserverID, "")
		
	default:
		return BadRequest("Unknown task type: %s", string(task.Type))
	}
}

// CreateBackup creates a backup of gameserver files
func (s *GameserverService) CreateBackup(ctx context.Context, gameserverID string, name string) error {
	if _, err := s.getGameserverOrError(gameserverID); err != nil {
		return err
	}

	// Generate backup filename
	if name == "" {
		name = fmt.Sprintf("backup-%s", time.Now().Format("20060102-150405"))
	}

	backupPath := filepath.Join(s.basePath, "backups", fmt.Sprintf("gameserver-%s", gameserverID))

	// Create backup
	if err := s.docker.CreateBackup(gameserverID, backupPath); err != nil {
		return InternalError(err, "Failed to create backup")
	}

	log.Info().Str("gameserver", gameserverID).Str("name", name).Msg("Backup created")
	return nil
}

// FileOperation handles file operations with path validation
func (s *GameserverService) FileOperation(ctx context.Context, gameserverID string, path string, op func(string, string) error) error {
	gs, err := s.getGameserverOrError(gameserverID)
	if err != nil {
		return err
	}

	// Validate and clean path
	cleanPath := filepath.Clean(path)
	if strings.Contains(cleanPath, "..") {
		return BadRequest("Invalid path")
	}

	// Execute operation
	if err := op(gs.ContainerID, cleanPath); err != nil {
		return InternalError(err, "File operation failed")
	}

	return nil
}
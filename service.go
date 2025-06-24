package main

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/rs/zerolog/log"

	"0xkowalskidev/gameservers/models"
)

// ServiceInterface defines the business logic layer for gameserver operations
type ServiceInterface interface {
	CreateGameserver(ctx context.Context, req CreateGameserverRequest) (*models.Gameserver, error)
	GetGameserver(ctx context.Context, id string) (*models.Gameserver, error)
	UpdateGameserver(ctx context.Context, id string, req UpdateGameserverRequest) error
	DeleteGameserver(ctx context.Context, id string) error
	StartGameserver(ctx context.Context, id string) error
	StopGameserver(ctx context.Context, id string) error
	RestartGameserver(ctx context.Context, id string) error
	ExecuteScheduledTask(ctx context.Context, task *models.ScheduledTask) error
}

// Service handles business logic for gameserver operations
type Service struct {
	db       models.GameserverServiceInterface
	docker   models.DockerManagerInterface
	basePath string
}

// NewService creates a new service instance
func NewService(db models.GameserverServiceInterface, docker models.DockerManagerInterface, basePath string) *Service {
	return &Service{
		db:       db,
		docker:   docker,
		basePath: basePath,
	}
}

// CreateGameserver creates a new gameserver with Docker container
func (s *Service) CreateGameserver(ctx context.Context, req CreateGameserverRequest) (*models.Gameserver, error) {
	// Validate input
	if req.Name == "" {
		return nil, BadRequest("Name is required")
	}
	if req.GameID == "" {
		return nil, BadRequest("Game ID is required")
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
	err = s.db.CreateGameserver(gs)
	if err != nil {
		return nil, InternalError(err, "Failed to create gameserver")
	}

	// Create Docker container
	err = s.docker.CreateContainer(gs)
	if err != nil {
		// Rollback database changes
		s.db.DeleteGameserver(gs.ID)
		return nil, InternalError(err, "Failed to create container")
	}

	log.Info().Str("name", gs.Name).Str("game", gs.GameID).Msg("Gameserver created")
	return gs, nil
}

// GetGameserver retrieves a gameserver by ID
func (s *Service) GetGameserver(ctx context.Context, id string) (*models.Gameserver, error) {
	gs, err := s.db.GetGameserver(id)
	if err != nil {
		return nil, NotFound("gameserver")
	}
	return gs, nil
}

// StartGameserver starts a gameserver container
func (s *Service) StartGameserver(ctx context.Context, id string) error {
	gs, err := s.db.GetGameserver(id)
	if err != nil {
		return NotFound("gameserver")
	}

	if err := s.docker.StartContainer(gs.ContainerID); err != nil {
		return InternalError(err, "Failed to start container")
	}

	gs.Status = models.StatusRunning
	gs.UpdatedAt = time.Now()
	if err := s.db.UpdateGameserver(gs); err != nil {
		return InternalError(err, "Failed to update status")
	}

	log.Info().Str("id", id).Str("name", gs.Name).Msg("Gameserver started")
	return nil
}

// StopGameserver stops a gameserver container
func (s *Service) StopGameserver(ctx context.Context, id string) error {
	gs, err := s.db.GetGameserver(id)
	if err != nil {
		return NotFound("gameserver")
	}

	if err := s.docker.StopContainer(gs.ContainerID); err != nil {
		return InternalError(err, "Failed to stop container")
	}

	gs.Status = models.StatusStopped
	gs.UpdatedAt = time.Now()
	if err := s.db.UpdateGameserver(gs); err != nil {
		return InternalError(err, "Failed to update status")
	}

	log.Info().Str("id", id).Str("name", gs.Name).Msg("Gameserver stopped")
	return nil
}

// RestartGameserver restarts a gameserver container
func (s *Service) RestartGameserver(ctx context.Context, id string) error {
	if err := s.StopGameserver(ctx, id); err != nil {
		return err
	}
	time.Sleep(2 * time.Second) // Brief pause
	return s.StartGameserver(ctx, id)
}

// DeleteGameserver deletes a gameserver and its container
func (s *Service) DeleteGameserver(ctx context.Context, id string) error {
	gs, err := s.db.GetGameserver(id)
	if err != nil {
		return NotFound("gameserver")
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
func (s *Service) UpdateGameserver(ctx context.Context, id string, req UpdateGameserverRequest) error {
	gs, err := s.db.GetGameserver(id)
	if err != nil {
		return NotFound("gameserver")
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
func (s *Service) ExecuteScheduledTask(ctx context.Context, task *models.ScheduledTask) error {
	log.Info().Str("id", task.ID).Str("type", string(task.Type)).Msg("Executing scheduled task")

	switch task.Type {
	case models.TaskTypeRestart:
		return s.RestartGameserver(ctx, task.GameserverID)
	case models.TaskTypeBackup:
		return s.CreateBackup(ctx, task.GameserverID, "")
	default:
		return BadRequest("Unknown task type: %s", string(task.Type))
	}
}

// CreateBackup creates a backup of gameserver files
func (s *Service) CreateBackup(ctx context.Context, gameserverID string, name string) error {
	_, err := s.db.GetGameserver(gameserverID)
	if err != nil {
		return NotFound("gameserver")
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
func (s *Service) FileOperation(ctx context.Context, gameserverID string, path string, op func(string, string) error) error {
	gs, err := s.db.GetGameserver(gameserverID)
	if err != nil {
		return NotFound("gameserver")
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

// Request types
type CreateGameserverRequest struct {
	Name     string
	GameID   string
	Port     int
	MemoryMB int
	CPUCores float64
}

type UpdateGameserverRequest struct {
	Name     string
	Port     int
	MemoryMB int
}
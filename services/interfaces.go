package services

import (
	"context"

	"0xkowalskidev/gameservers/models"
)

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
package services

import (
	"time"

	"github.com/google/uuid"

	"0xkowalskidev/gameservers/models"
)

// GameService handles game-related operations
type GameService struct {
	db models.DatabaseInterface
}

// NewGameService creates a new game service
func NewGameService(db models.DatabaseInterface) models.GameServiceInterface {
	return &GameService{
		db: db,
	}
}

// ListGames returns all games
func (s *GameService) ListGames() ([]*models.Game, error) {
	return s.db.ListGames()
}

// GetGame returns a specific game
func (s *GameService) GetGame(id string) (*models.Game, error) {
	return s.db.GetGame(id)
}

// CreateGame creates a new game
func (s *GameService) CreateGame(game *models.Game) error {
	if game.ID == "" {
		game.ID = uuid.New().String()
	}
	now := time.Now()
	game.CreatedAt, game.UpdatedAt = now, now
	return s.db.CreateGame(game)
}

// UpdateGame updates an existing game
func (s *GameService) UpdateGame(game *models.Game) error {
	game.UpdatedAt = time.Now()
	return s.db.UpdateGame(game)
}

// DeleteGame deletes a game
func (s *GameService) DeleteGame(id string) error {
	return s.db.DeleteGame(id)
}
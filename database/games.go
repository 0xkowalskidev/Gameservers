package database

import (
	"fmt"

	"0xkowalskidev/gameservers/models"
)

// CreateGame inserts a new game into the database
func (dm *DatabaseManager) CreateGame(game *models.Game) error {
	if err := dm.db.Create(game).Error; err != nil {
		return &models.DatabaseError{Op: "create_game", Msg: fmt.Sprintf("failed to insert game %s", game.Name), Err: err}
	}
	return nil
}

// GetGame retrieves a game by ID
func (dm *DatabaseManager) GetGame(id string) (*models.Game, error) {
	var game models.Game
	if err := dm.db.First(&game, "id = ?", id).Error; err != nil {
		return nil, &models.DatabaseError{Op: "get_game", Msg: fmt.Sprintf("failed to get game %s", id), Err: err}
	}
	return &game, nil
}

// ListGames retrieves all games
func (dm *DatabaseManager) ListGames() ([]*models.Game, error) {
	var games []*models.Game
	if err := dm.db.Order("name").Find(&games).Error; err != nil {
		return nil, &models.DatabaseError{Op: "list_games", Msg: "failed to query games", Err: err}
	}
	return games, nil
}

// UpdateGame updates an existing game
func (dm *DatabaseManager) UpdateGame(game *models.Game) error {
	if err := dm.db.Save(game).Error; err != nil {
		return &models.DatabaseError{Op: "update_game", Msg: fmt.Sprintf("failed to update game %s", game.ID), Err: err}
	}
	return nil
}

// DeleteGame deletes a game by ID
func (dm *DatabaseManager) DeleteGame(id string) error {
	if err := dm.db.Unscoped().Delete(&models.Game{}, "id = ?", id).Error; err != nil {
		return &models.DatabaseError{Op: "delete_game", Msg: fmt.Sprintf("failed to delete game %s", id), Err: err}
	}
	return nil
}

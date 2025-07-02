package database

import (
	"encoding/json"
	"fmt"

	"0xkowalskidev/gameservers/models"
)

// CreateGame inserts a new game into the database
func (dm *DatabaseManager) CreateGame(game *models.Game) error {
	portMappingsJSON, err := json.Marshal(game.PortMappings)
	if err != nil {
		return &models.DatabaseError{Op: "db", Msg: "failed to marshal port mappings", Err: err}
	}

	configVarsJSON, err := json.Marshal(game.ConfigVars)
	if err != nil {
		return &models.DatabaseError{Op: "db", Msg: "failed to marshal config vars", Err: err}
	}

	_, err = dm.db.Exec(`INSERT INTO games (id, name, slug, image, icon_path, grid_image_path, port_mappings, config_vars, min_memory_mb, rec_memory_mb, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		game.ID, game.Name, game.Slug, game.Image, game.IconPath, game.GridImagePath, string(portMappingsJSON), string(configVarsJSON), game.MinMemoryMB, game.RecMemoryMB, game.CreatedAt, game.UpdatedAt)

	if err != nil {
		return &models.DatabaseError{Op: "create_game", Msg: fmt.Sprintf("failed to insert game %s", game.Name), Err: err}
	}
	return nil
}

// GetGame retrieves a game by ID
func (dm *DatabaseManager) GetGame(id string) (*models.Game, error) {
	row := dm.db.QueryRow(`SELECT id, name, slug, image, icon_path, grid_image_path, port_mappings, config_vars, min_memory_mb, rec_memory_mb, created_at, updated_at FROM games WHERE id = ?`, id)
	return dm.scanGame(row)
}

// ListGames retrieves all games
func (dm *DatabaseManager) ListGames() ([]*models.Game, error) {
	rows, err := dm.db.Query(`SELECT id, name, slug, image, icon_path, grid_image_path, port_mappings, config_vars, min_memory_mb, rec_memory_mb, created_at, updated_at FROM games ORDER BY name`)
	if err != nil {
		return nil, &models.DatabaseError{Op: "list_games", Msg: "failed to query games", Err: err}
	}
	defer rows.Close()

	var games []*models.Game
	for rows.Next() {
		game, err := dm.scanGame(rows)
		if err != nil {
			return nil, &models.DatabaseError{Op: "list_games", Msg: "failed to scan game", Err: err}
		}
		games = append(games, game)
	}
	return games, nil
}

// UpdateGame updates an existing game
func (dm *DatabaseManager) UpdateGame(game *models.Game) error {
	portMappingsJSON, err := json.Marshal(game.PortMappings)
	if err != nil {
		return &models.DatabaseError{Op: "update_game", Msg: "failed to marshal port mappings", Err: err}
	}

	configVarsJSON, err := json.Marshal(game.ConfigVars)
	if err != nil {
		return &models.DatabaseError{Op: "update_game", Msg: "failed to marshal config vars", Err: err}
	}

	_, err = dm.db.Exec(`UPDATE games SET name = ?, slug = ?, image = ?, icon_path = ?, grid_image_path = ?, port_mappings = ?, config_vars = ?, updated_at = ? WHERE id = ?`,
		game.Name, game.Slug, game.Image, game.IconPath, game.GridImagePath, string(portMappingsJSON), string(configVarsJSON), game.UpdatedAt, game.ID)

	if err != nil {
		return &models.DatabaseError{Op: "update_game", Msg: fmt.Sprintf("failed to update game %s", game.ID), Err: err}
	}
	return nil
}

// DeleteGame deletes a game by ID
func (dm *DatabaseManager) DeleteGame(id string) error {
	_, err := dm.db.Exec(`DELETE FROM games WHERE id = ?`, id)
	if err != nil {
		return &models.DatabaseError{Op: "delete_game", Msg: fmt.Sprintf("failed to delete game %s", id), Err: err}
	}
	return nil
}

// scanGame scans a database row into a Game struct
func (dm *DatabaseManager) scanGame(row interface{ Scan(...interface{}) error }) (*models.Game, error) {
	var game models.Game
	var portMappingsJSON, configVarsJSON string
	err := row.Scan(&game.ID, &game.Name, &game.Slug, &game.Image, &game.IconPath, &game.GridImagePath, &portMappingsJSON, &configVarsJSON, &game.MinMemoryMB, &game.RecMemoryMB, &game.CreatedAt, &game.UpdatedAt)
	if err != nil {
		return nil, err
	}

	if err := json.Unmarshal([]byte(portMappingsJSON), &game.PortMappings); err != nil {
		return nil, &models.DatabaseError{Op: "scan_game", Msg: "failed to unmarshal port mappings", Err: err}
	}

	if err := json.Unmarshal([]byte(configVarsJSON), &game.ConfigVars); err != nil {
		return nil, &models.DatabaseError{Op: "scan_game", Msg: "failed to unmarshal config vars", Err: err}
	}

	return &game, nil
}

package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"time"

	_ "github.com/mattn/go-sqlite3"
	"github.com/rs/zerolog/log"
)

type DatabaseError struct {
	Op  string
	Msg string
	Err error
}

func (e *DatabaseError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("db %s: %s: %v", e.Op, e.Msg, e.Err)
	}
	return fmt.Sprintf("db %s: %s", e.Op, e.Msg)
}

type DatabaseManager struct {
	db *sql.DB
}

func NewDatabaseManager(dbPath string) (*DatabaseManager, error) {
	log.Info().Str("db_path", dbPath).Msg("Connecting to database")

	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		log.Error().Err(err).Str("db_path", dbPath).Msg("Failed to open database")
		return nil, &DatabaseError{Op: "db", Msg: "failed to open database", Err: err}
	}

	if err := db.Ping(); err != nil {
		log.Error().Err(err).Msg("Failed to ping database")
		return nil, &DatabaseError{Op: "db", Msg: "failed to ping database", Err: err}
	}

	dm := &DatabaseManager{db: db}
	if err := dm.migrate(); err != nil {
		log.Error().Err(err).Msg("Database migration failed")
		return nil, err
	}

	if err := dm.seedGames(); err != nil {
		log.Error().Err(err).Msg("Failed to seed games")
		return nil, err
	}

	log.Info().Msg("Database connected and migrated successfully")
	return dm, nil
}

func (dm *DatabaseManager) Close() error { return dm.db.Close() }

func (dm *DatabaseManager) migrate() error {
	schema := `
	CREATE TABLE IF NOT EXISTS games (
		id TEXT PRIMARY KEY, name TEXT NOT NULL UNIQUE, image TEXT NOT NULL,
		default_port INTEGER NOT NULL, created_at DATETIME NOT NULL, updated_at DATETIME NOT NULL
	);
	CREATE TABLE IF NOT EXISTS gameservers (
		id TEXT PRIMARY KEY, name TEXT NOT NULL UNIQUE, game_id TEXT NOT NULL,
		container_id TEXT, status TEXT NOT NULL DEFAULT 'stopped', port INTEGER NOT NULL,
		memory_mb INTEGER NOT NULL DEFAULT 1024, cpu_cores REAL NOT NULL DEFAULT 0,
		environment TEXT, volumes TEXT, created_at DATETIME NOT NULL, updated_at DATETIME NOT NULL,
		FOREIGN KEY (game_id) REFERENCES games(id)
	);
	CREATE INDEX IF NOT EXISTS idx_gameservers_status ON gameservers(status);
	CREATE INDEX IF NOT EXISTS idx_gameservers_game_id ON gameservers(game_id);`

	_, err := dm.db.Exec(schema)
	if err != nil {
		return &DatabaseError{Op: "db", Msg: "failed to create schema", Err: err}
	}

	// Add new columns if they don't exist (for existing databases)
	alterQueries := []string{
		`ALTER TABLE gameservers ADD COLUMN memory_mb INTEGER DEFAULT 1024`,
		`ALTER TABLE gameservers ADD COLUMN cpu_cores REAL DEFAULT 0`,
	}
	
	for _, query := range alterQueries {
		_, err := dm.db.Exec(query)
		// Ignore errors for columns that already exist
		if err != nil && !strings.Contains(err.Error(), "duplicate column name") {
			log.Warn().Err(err).Str("query", query).Msg("Failed to add column, may already exist")
		}
	}
	
	return nil
}

func (dm *DatabaseManager) seedGames() error {
	// Check if games already exist
	count := 0
	row := dm.db.QueryRow("SELECT COUNT(*) FROM games")
	row.Scan(&count)
	if count > 0 {
		return nil // Games already seeded
	}

	games := []*Game{
		{ID: "minecraft", Name: "Minecraft", Image: "ghcr.io/0xkowalskidev/gameservers/minecraft:latest", DefaultPort: 25565, CreatedAt: time.Now(), UpdatedAt: time.Now()},
		{ID: "cs2", Name: "Counter-Strike 2", Image: "ghcr.io/0xkowalskidev/gameservers/cs2:latest", DefaultPort: 27015, CreatedAt: time.Now(), UpdatedAt: time.Now()},
		{ID: "valheim", Name: "Valheim", Image: "ghcr.io/0xkowalskidev/gameservers/valheim:latest", DefaultPort: 2456, CreatedAt: time.Now(), UpdatedAt: time.Now()},
		{ID: "terraria", Name: "Terraria", Image: "ghcr.io/0xkowalskidev/gameservers/terraria:latest", DefaultPort: 7777, CreatedAt: time.Now(), UpdatedAt: time.Now()},
	}

	for _, game := range games {
		if err := dm.CreateGame(game); err != nil {
			return err
		}
	}

	log.Info().Int("count", len(games)).Msg("Seeded games")
	return nil
}

func (dm *DatabaseManager) CreateGame(game *Game) error {
	_, err := dm.db.Exec(`INSERT INTO games (id, name, image, default_port, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?)`,
		game.ID, game.Name, game.Image, game.DefaultPort, game.CreatedAt, game.UpdatedAt)

	if err != nil {
		return &DatabaseError{Op: fmt.Sprintf("failed to insert game %s", game.Name),Err: err}
	}
	return nil
}

func (dm *DatabaseManager) GetGame(id string) (*Game, error) {
	row := dm.db.QueryRow(`SELECT id, name, image, default_port, created_at, updated_at FROM games WHERE id = ?`, id)
	return dm.scanGame(row)
}

func (dm *DatabaseManager) ListGames() ([]*Game, error) {
	rows, err := dm.db.Query(`SELECT id, name, image, default_port, created_at, updated_at FROM games ORDER BY name`)
	if err != nil {
		return nil, &DatabaseError{Op: "db", Msg: "failed to query games", Err: err}
	}
	defer rows.Close()

	var games []*Game
	for rows.Next() {
		game, err := dm.scanGame(rows)
		if err != nil {
			return nil, &DatabaseError{Op: "db", Msg: "failed to scan game", Err: err}
		}
		games = append(games, game)
	}
	return games, nil
}

func (dm *DatabaseManager) UpdateGame(game *Game) error {
	_, err := dm.db.Exec(`UPDATE games SET name = ?, image = ?, default_port = ?, updated_at = ? WHERE id = ?`,
		game.Name, game.Image, game.DefaultPort, game.UpdatedAt, game.ID)

	if err != nil {
		return &DatabaseError{Op: fmt.Sprintf("failed to update game %s", game.ID),Err: err}
	}
	return nil
}

func (dm *DatabaseManager) DeleteGame(id string) error {
	_, err := dm.db.Exec(`DELETE FROM games WHERE id = ?`, id)
	if err != nil {
		return &DatabaseError{Op: fmt.Sprintf("failed to delete game %s", id),Err: err}
	}
	return nil
}

func (dm *DatabaseManager) CreateGameserver(server *Gameserver) error {
	envJSON, _ := json.Marshal(server.Environment)
	volumesJSON, _ := json.Marshal(server.Volumes)

	_, err := dm.db.Exec(`INSERT INTO gameservers (id, name, game_id, container_id, status, port, memory_mb, cpu_cores, environment, volumes, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		server.ID, server.Name, server.GameID, server.ContainerID, server.Status, server.Port, server.MemoryMB, server.CPUCores, string(envJSON), string(volumesJSON), server.CreatedAt, server.UpdatedAt)

	if err != nil {
		log.Error().Err(err).Str("gameserver_id", server.ID).Str("name", server.Name).Msg("Failed to create gameserver in database")
		return &DatabaseError{Op: fmt.Sprintf("failed to insert gameserver %s", server.Name),Err: err}
	}

	return nil
}

func (dm *DatabaseManager) scanGame(row interface{ Scan(...interface{}) error }) (*Game, error) {
	var game Game
	err := row.Scan(&game.ID, &game.Name, &game.Image, &game.DefaultPort, &game.CreatedAt, &game.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return &game, nil
}

func (dm *DatabaseManager) scanGameserver(row interface{ Scan(...interface{}) error }) (*Gameserver, error) {
	var server Gameserver
	var envJSON, volumesJSON string

	err := row.Scan(&server.ID, &server.Name, &server.GameID, &server.ContainerID, &server.Status, &server.Port, &server.MemoryMB, &server.CPUCores, &envJSON, &volumesJSON, &server.CreatedAt, &server.UpdatedAt)
	if err != nil {
		return nil, err
	}

	json.Unmarshal([]byte(envJSON), &server.Environment)
	json.Unmarshal([]byte(volumesJSON), &server.Volumes)
	return &server, nil
}

func (dm *DatabaseManager) GetGameserver(id string) (*Gameserver, error) {
	row := dm.db.QueryRow(`SELECT id, name, game_id, container_id, status, port, memory_mb, cpu_cores, environment, volumes, created_at, updated_at FROM gameservers WHERE id = ?`, id)
	server, err := dm.scanGameserver(row)
	if err == sql.ErrNoRows {
		return nil, &DatabaseError{Op: "error", Msg: fmt.Sprintf("gameserver %s not found", id), Err: nil}
	}
	if err != nil {
		return nil, &DatabaseError{Op: fmt.Sprintf("failed to query gameserver %s", id),Err: err}
	}
	return server, nil
}

func (dm *DatabaseManager) UpdateGameserver(server *Gameserver) error {
	envJSON, _ := json.Marshal(server.Environment)
	volumesJSON, _ := json.Marshal(server.Volumes)
	server.UpdatedAt = time.Now()

	result, err := dm.db.Exec(`UPDATE gameservers SET name = ?, game_id = ?, container_id = ?, status = ?, port = ?, memory_mb = ?, cpu_cores = ?, environment = ?, volumes = ?, updated_at = ? WHERE id = ?`,
		server.Name, server.GameID, server.ContainerID, server.Status, server.Port, server.MemoryMB, server.CPUCores, string(envJSON), string(volumesJSON), server.UpdatedAt, server.ID)

	if err != nil {
		return &DatabaseError{Op: fmt.Sprintf("failed to update gameserver %s", server.ID),Err: err}
	}

	if rowsAffected, _ := result.RowsAffected(); rowsAffected == 0 {
		return &DatabaseError{Op: "error", Msg: fmt.Sprintf("gameserver %s not found", server.ID), Err: nil}
	}
	return nil
}

func (dm *DatabaseManager) DeleteGameserver(id string) error {
	result, err := dm.db.Exec(`DELETE FROM gameservers WHERE id = ?`, id)
	if err != nil {
		return &DatabaseError{Op: fmt.Sprintf("failed to delete gameserver %s", id),Err: err}
	}
	if rowsAffected, _ := result.RowsAffected(); rowsAffected == 0 {
		return &DatabaseError{Op: "error", Msg: fmt.Sprintf("gameserver %s not found", id), Err: nil}
	}
	return nil
}

func (dm *DatabaseManager) ListGameservers() ([]*Gameserver, error) {
	rows, err := dm.db.Query(`SELECT id, name, game_id, container_id, status, port, memory_mb, cpu_cores, environment, volumes, created_at, updated_at FROM gameservers ORDER BY created_at DESC`)
	if err != nil {
		return nil, &DatabaseError{Op: "db", Msg: "failed to query gameservers", Err: err}
	}
	defer rows.Close()

	var servers []*Gameserver
	for rows.Next() {
		server, err := dm.scanGameserver(rows)
		if err != nil {
			return nil, &DatabaseError{Op: "db", Msg: "failed to scan gameserver row", Err: err}
		}
		servers = append(servers, server)
	}
	return servers, rows.Err()
}

func (dm *DatabaseManager) GetGameserverByContainerID(containerID string) (*Gameserver, error) {
	row := dm.db.QueryRow(`SELECT id, name, game_id, container_id, status, port, memory_mb, cpu_cores, environment, volumes, created_at, updated_at FROM gameservers WHERE container_id = ?`, containerID)
	server, err := dm.scanGameserver(row)
	if err == sql.ErrNoRows {
		return nil, &DatabaseError{Op: "error", Msg: fmt.Sprintf("gameserver with container %s not found", containerID), Err: nil}
	}
	if err != nil {
		return nil, &DatabaseError{Op: fmt.Sprintf("failed to query gameserver by container %s", containerID),Err: err}
	}
	return server, nil
}

type GameserverService struct {
	db     *DatabaseManager
	docker DockerManagerInterface
}

func NewGameserverService(db *DatabaseManager, docker DockerManagerInterface) *GameserverService {
	return &GameserverService{db: db, docker: docker}
}

func (gss *GameserverService) CreateGameserver(server *Gameserver) error {
	now := time.Now()
	server.CreatedAt, server.UpdatedAt, server.Status = now, now, StatusStopped
	server.ContainerID = "" // No container created yet

	// Populate derived fields from game
	if err := gss.populateGameFields(server); err != nil {
		return err
	}

	return gss.db.CreateGameserver(server)
}

func (gss *GameserverService) UpdateGameserver(server *Gameserver) error {
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

func (gss *GameserverService) populateGameFields(server *Gameserver) error {
	game, err := gss.db.GetGame(server.GameID)
	if err != nil {
		return err
	}
	server.GameType = game.Name
	server.Image = game.Image
	server.MemoryGB = float64(server.MemoryMB) / 1024.0
	return nil
}


func (gss *GameserverService) StartGameserver(id string) error {
	server, err := gss.db.GetGameserver(id)
	if err != nil {
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
	
	server.Status = StatusStarting
	server.UpdatedAt = time.Now()
	return gss.db.UpdateGameserver(server)
}

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
	
	server.Status = StatusStopped
	server.UpdatedAt = time.Now()
	return gss.db.UpdateGameserver(server)
}

func (gss *GameserverService) RestartGameserver(id string) error {
	// Stop first (removes container)
	if err := gss.StopGameserver(id); err != nil {
		return err
	}
	
	// Then start (creates new container)
	return gss.StartGameserver(id)
}

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

func (gss *GameserverService) syncStatus(server *Gameserver) {
	if server.ContainerID != "" {
		if dockerStatus, err := gss.docker.GetContainerStatus(server.ContainerID); err == nil && server.Status != dockerStatus {
			server.Status, server.UpdatedAt = dockerStatus, time.Now()
			gss.db.UpdateGameserver(server)
		}
	}
}

func (gss *GameserverService) GetGameserver(id string) (*Gameserver, error) {
	server, err := gss.db.GetGameserver(id)
	if err != nil {
		return nil, err
	}
	gss.populateGameFields(server)
	gss.syncStatus(server)
	return server, nil
}

func (gss *GameserverService) ListGameservers() ([]*Gameserver, error) {
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


func (gss *GameserverService) StreamGameserverLogs(id string) (io.ReadCloser, error) {
	server, err := gss.db.GetGameserver(id)
	if err != nil {
		return nil, err
	}
	if server.ContainerID == "" {
		return nil, &DatabaseError{Op: "error", Msg: "container not created yet", Err: nil}
	}
	return gss.docker.StreamContainerLogs(server.ContainerID)
}

func (gss *GameserverService) StreamGameserverStats(id string) (io.ReadCloser, error) {
	server, err := gss.db.GetGameserver(id)
	if err != nil {
		return nil, err
	}
	if server.ContainerID == "" {
		return nil, &DatabaseError{Op: "error", Msg: "container not created yet", Err: nil}
	}
	return gss.docker.StreamContainerStats(server.ContainerID)
}

func (gss *GameserverService) ListGames() ([]*Game, error) {
	return gss.db.ListGames()
}

func (gss *GameserverService) GetGame(id string) (*Game, error) {
	return gss.db.GetGame(id)
}

func (gss *GameserverService) CreateGame(game *Game) error {
	now := time.Now()
	game.CreatedAt, game.UpdatedAt = now, now
	return gss.db.CreateGame(game)
}


package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"strconv"
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

	// Enable foreign key constraints
	if _, err := db.Exec("PRAGMA foreign_keys = ON"); err != nil {
		log.Error().Err(err).Msg("Failed to enable foreign key constraints")
		return nil, &DatabaseError{Op: "db", Msg: "failed to enable foreign key constraints", Err: err}
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
	CREATE TABLE IF NOT EXISTS scheduled_tasks (
		id TEXT PRIMARY KEY, gameserver_id TEXT NOT NULL, name TEXT NOT NULL,
		type TEXT NOT NULL, status TEXT NOT NULL DEFAULT 'active', cron_schedule TEXT NOT NULL,
		created_at DATETIME NOT NULL, updated_at DATETIME NOT NULL,
		last_run DATETIME, next_run DATETIME,
		FOREIGN KEY (gameserver_id) REFERENCES gameservers(id) ON DELETE CASCADE
	);
	CREATE INDEX IF NOT EXISTS idx_gameservers_status ON gameservers(status);
	CREATE INDEX IF NOT EXISTS idx_gameservers_game_id ON gameservers(game_id);
	CREATE INDEX IF NOT EXISTS idx_scheduled_tasks_gameserver_id ON scheduled_tasks(gameserver_id);
	CREATE INDEX IF NOT EXISTS idx_scheduled_tasks_status ON scheduled_tasks(status);`

	_, err := dm.db.Exec(schema)
	if err != nil {
		return &DatabaseError{Op: "db", Msg: "failed to create schema", Err: err}
	}

	// Add new columns if they don't exist (for existing databases)
	alterQueries := []string{
		`ALTER TABLE gameservers ADD COLUMN memory_mb INTEGER DEFAULT 1024`,
		`ALTER TABLE gameservers ADD COLUMN cpu_cores REAL DEFAULT 0`,
		`ALTER TABLE gameservers ADD COLUMN max_backups INTEGER DEFAULT 7`,
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

	_, err := dm.db.Exec(`INSERT INTO gameservers (id, name, game_id, container_id, status, port, memory_mb, cpu_cores, max_backups, environment, volumes, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		server.ID, server.Name, server.GameID, server.ContainerID, server.Status, server.Port, server.MemoryMB, server.CPUCores, server.MaxBackups, string(envJSON), string(volumesJSON), server.CreatedAt, server.UpdatedAt)

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

	err := row.Scan(&server.ID, &server.Name, &server.GameID, &server.ContainerID, &server.Status, &server.Port, &server.MemoryMB, &server.CPUCores, &server.MaxBackups, &envJSON, &volumesJSON, &server.CreatedAt, &server.UpdatedAt)
	if err != nil {
		return nil, err
	}

	json.Unmarshal([]byte(envJSON), &server.Environment)
	json.Unmarshal([]byte(volumesJSON), &server.Volumes)
	return &server, nil
}

func (dm *DatabaseManager) GetGameserver(id string) (*Gameserver, error) {
	row := dm.db.QueryRow(`SELECT id, name, game_id, container_id, status, port, memory_mb, cpu_cores, max_backups, environment, volumes, created_at, updated_at FROM gameservers WHERE id = ?`, id)
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

	result, err := dm.db.Exec(`UPDATE gameservers SET name = ?, game_id = ?, container_id = ?, status = ?, port = ?, memory_mb = ?, cpu_cores = ?, max_backups = ?, environment = ?, volumes = ?, updated_at = ? WHERE id = ?`,
		server.Name, server.GameID, server.ContainerID, server.Status, server.Port, server.MemoryMB, server.CPUCores, server.MaxBackups, string(envJSON), string(volumesJSON), server.UpdatedAt, server.ID)

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
	rows, err := dm.db.Query(`SELECT id, name, game_id, container_id, status, port, memory_mb, cpu_cores, max_backups, environment, volumes, created_at, updated_at FROM gameservers ORDER BY created_at DESC`)
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
	row := dm.db.QueryRow(`SELECT id, name, game_id, container_id, status, port, memory_mb, cpu_cores, max_backups, environment, volumes, created_at, updated_at FROM gameservers WHERE container_id = ?`, containerID)
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

	// Create the gameserver in database
	if err := gss.db.CreateGameserver(server); err != nil {
		return err
	}

	// Create automatic daily backup task
	backupTask := &ScheduledTask{
		GameserverID: server.ID,
		Name:         "Daily Backup",
		Type:         TaskTypeBackup,
		Status:       TaskStatusActive,
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
	
	// Get volume information
	volumeName := fmt.Sprintf("gameservers-%s-data", server.Name)
	if volumeInfo, err := gss.docker.GetVolumeInfo(volumeName); err == nil {
		server.VolumeInfo = volumeInfo
	}
	
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

func (gss *GameserverService) SendGameserverCommand(id string, command string) error {
	server, err := gss.db.GetGameserver(id)
	if err != nil {
		return err
	}
	
	if server.ContainerID == "" {
		return &DatabaseError{
			Op:  "send_command",
			Msg: "gameserver has no container",
			Err: nil,
		}
	}
	
	if server.Status != StatusRunning {
		return &DatabaseError{
			Op:  "send_command", 
			Msg: "gameserver is not running",
			Err: nil,
		}
	}
	
	return gss.docker.SendCommand(server.ContainerID, command)
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

// =============================================================================
// Scheduled Task Service Operations
// =============================================================================

func (gss *GameserverService) CreateScheduledTask(task *ScheduledTask) error {
	now := time.Now()
	task.CreatedAt, task.UpdatedAt = now, now
	task.ID = generateID()
	
	// Calculate initial next run time
	if nextRun := gss.calculateNextRun(task.CronSchedule, now); nextRun != nil {
		task.NextRun = nextRun
	}
	
	return gss.db.CreateScheduledTask(task)
}

func (gss *GameserverService) GetScheduledTask(id string) (*ScheduledTask, error) {
	return gss.db.GetScheduledTask(id)
}

func (gss *GameserverService) UpdateScheduledTask(task *ScheduledTask) error {
	task.UpdatedAt = time.Now()
	// Clear next run time so scheduler will recalculate it
	task.NextRun = nil
	return gss.db.UpdateScheduledTask(task)
}

func (gss *GameserverService) DeleteScheduledTask(id string) error {
	return gss.db.DeleteScheduledTask(id)
}

func (gss *GameserverService) ListScheduledTasksForGameserver(gameserverID string) ([]*ScheduledTask, error) {
	return gss.db.ListScheduledTasksForGameserver(gameserverID)
}

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

func (gss *GameserverService) RestoreGameserverBackup(gameserverID, backupFilename string) error {
	gameserver, err := gss.db.GetGameserver(gameserverID)
	if err != nil {
		return err
	}
	return gss.docker.RestoreBackup(gameserver.ContainerID, backupFilename)
}

// File operation methods

func (gss *GameserverService) ListFiles(containerID string, path string) ([]*FileInfo, error) {
	return gss.docker.ListFiles(containerID, path)
}

func (gss *GameserverService) ReadFile(containerID string, path string) ([]byte, error) {
	return gss.docker.ReadFile(containerID, path)
}

func (gss *GameserverService) WriteFile(containerID string, path string, content []byte) error {
	return gss.docker.WriteFile(containerID, path, content)
}

func (gss *GameserverService) CreateDirectory(containerID string, path string) error {
	return gss.docker.CreateDirectory(containerID, path)
}

func (gss *GameserverService) DeletePath(containerID string, path string) error {
	return gss.docker.DeletePath(containerID, path)
}

func (gss *GameserverService) DownloadFile(containerID string, path string) (io.ReadCloser, error) {
	return gss.docker.DownloadFile(containerID, path)
}

func (gss *GameserverService) RenameFile(containerID string, oldPath string, newPath string) error {
	return gss.docker.RenameFile(containerID, oldPath, newPath)
}

// Simple cron parser for calculating next run times
func (gss *GameserverService) calculateNextRun(cronSchedule string, from time.Time) *time.Time {
	parts := strings.Fields(cronSchedule)
	if len(parts) != 5 {
		return nil
	}

	minute := parts[0]
	hour := parts[1]
	day := parts[2]
	month := parts[3]
	weekday := parts[4]

	// Start from the next minute
	next := from.Truncate(time.Minute).Add(time.Minute)
	
	// Simple implementation - find next matching time within next 7 days
	for attempts := 0; attempts < 7*24*60; attempts++ {
		if gss.cronMatches(next, minute, hour, day, month, weekday) {
			return &next
		}
		next = next.Add(time.Minute)
	}

	return nil
}

func (gss *GameserverService) cronMatches(t time.Time, minute, hour, day, month, weekday string) bool {
	return gss.fieldMatches(t.Minute(), minute) &&
		gss.fieldMatches(t.Hour(), hour) &&
		gss.fieldMatches(t.Day(), day) &&
		gss.fieldMatches(int(t.Month()), month) &&
		gss.fieldMatches(int(t.Weekday()), weekday)
}

func (gss *GameserverService) fieldMatches(value int, pattern string) bool {
	if pattern == "*" {
		return true
	}
	
	// Handle step values like */5
	if strings.HasPrefix(pattern, "*/") {
		stepStr := pattern[2:]
		if step, err := strconv.Atoi(stepStr); err == nil {
			return value%step == 0
		}
		return false
	}
	
	// Handle exact matches
	if patternValue, err := strconv.Atoi(pattern); err == nil {
		return value == patternValue
	}
	
	return false
}

// =============================================================================
// Scheduled Task Database Operations
// =============================================================================

func (dm *DatabaseManager) CreateScheduledTask(task *ScheduledTask) error {
	query := `INSERT INTO scheduled_tasks (id, gameserver_id, name, type, status, cron_schedule, created_at, updated_at, last_run, next_run) 
			  VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`
	
	_, err := dm.db.Exec(query, task.ID, task.GameserverID, task.Name, string(task.Type), string(task.Status), 
		task.CronSchedule, task.CreatedAt, task.UpdatedAt, task.LastRun, task.NextRun)
	
	if err != nil {
		return &DatabaseError{Op: "create_task", Msg: "failed to create scheduled task", Err: err}
	}
	return nil
}

func (dm *DatabaseManager) GetScheduledTask(id string) (*ScheduledTask, error) {
	query := `SELECT id, gameserver_id, name, type, status, cron_schedule, created_at, updated_at, last_run, next_run 
			  FROM scheduled_tasks WHERE id = ?`
	
	row := dm.db.QueryRow(query, id)
	task, err := dm.scanScheduledTask(row)
	if err == sql.ErrNoRows {
		return nil, &DatabaseError{Op: "get_task", Msg: fmt.Sprintf("scheduled task %s not found", id), Err: nil}
	}
	if err != nil {
		return nil, &DatabaseError{Op: "get_task", Msg: fmt.Sprintf("failed to query scheduled task %s", id), Err: err}
	}
	return task, nil
}

func (dm *DatabaseManager) UpdateScheduledTask(task *ScheduledTask) error {
	query := `UPDATE scheduled_tasks SET name = ?, type = ?, status = ?, cron_schedule = ?, updated_at = ?, last_run = ?, next_run = ? 
			  WHERE id = ?`
	
	result, err := dm.db.Exec(query, task.Name, string(task.Type), string(task.Status), task.CronSchedule, 
		task.UpdatedAt, task.LastRun, task.NextRun, task.ID)
	
	if err != nil {
		return &DatabaseError{Op: "update_task", Msg: "failed to update scheduled task", Err: err}
	}
	if rowsAffected, _ := result.RowsAffected(); rowsAffected == 0 {
		return &DatabaseError{Op: "update_task", Msg: fmt.Sprintf("scheduled task %s not found", task.ID), Err: nil}
	}
	return nil
}

func (dm *DatabaseManager) DeleteScheduledTask(id string) error {
	query := `DELETE FROM scheduled_tasks WHERE id = ?`
	
	result, err := dm.db.Exec(query, id)
	if err != nil {
		return &DatabaseError{Op: "delete_task", Msg: "failed to delete scheduled task", Err: err}
	}
	if rowsAffected, _ := result.RowsAffected(); rowsAffected == 0 {
		return &DatabaseError{Op: "delete_task", Msg: fmt.Sprintf("scheduled task %s not found", id), Err: nil}
	}
	return nil
}

func (dm *DatabaseManager) ListScheduledTasksForGameserver(gameserverID string) ([]*ScheduledTask, error) {
	query := `SELECT id, gameserver_id, name, type, status, cron_schedule, created_at, updated_at, last_run, next_run 
			  FROM scheduled_tasks WHERE gameserver_id = ? ORDER BY created_at DESC`
	
	rows, err := dm.db.Query(query, gameserverID)
	if err != nil {
		return nil, &DatabaseError{Op: "list_tasks", Msg: "failed to query scheduled tasks", Err: err}
	}
	defer rows.Close()

	var tasks []*ScheduledTask
	for rows.Next() {
		task, err := dm.scanScheduledTask(rows)
		if err != nil {
			return nil, &DatabaseError{Op: "list_tasks", Msg: "failed to scan scheduled task row", Err: err}
		}
		tasks = append(tasks, task)
	}
	return tasks, rows.Err()
}

func (dm *DatabaseManager) ListActiveScheduledTasks() ([]*ScheduledTask, error) {
	query := `SELECT id, gameserver_id, name, type, status, cron_schedule, created_at, updated_at, last_run, next_run 
			  FROM scheduled_tasks WHERE status = ? ORDER BY next_run ASC`
	
	rows, err := dm.db.Query(query, string(TaskStatusActive))
	if err != nil {
		return nil, &DatabaseError{Op: "list_active_tasks", Msg: "failed to query active scheduled tasks", Err: err}
	}
	defer rows.Close()

	var tasks []*ScheduledTask
	for rows.Next() {
		task, err := dm.scanScheduledTask(rows)
		if err != nil {
			return nil, &DatabaseError{Op: "list_active_tasks", Msg: "failed to scan scheduled task row", Err: err}
		}
		tasks = append(tasks, task)
	}
	return tasks, rows.Err()
}

type ScheduledTaskScanner interface {
	Scan(dest ...interface{}) error
}

func (dm *DatabaseManager) scanScheduledTask(row ScheduledTaskScanner) (*ScheduledTask, error) {
	var task ScheduledTask
	var taskType, status string
	var lastRun, nextRun sql.NullTime
	
	err := row.Scan(&task.ID, &task.GameserverID, &task.Name, &taskType, &status, 
		&task.CronSchedule, &task.CreatedAt, &task.UpdatedAt, &lastRun, &nextRun)
	
	if err != nil {
		return nil, err
	}
	
	task.Type = TaskType(taskType)
	task.Status = TaskStatus(status)
	
	if lastRun.Valid {
		task.LastRun = &lastRun.Time
	}
	if nextRun.Valid {
		task.NextRun = &nextRun.Time
	}
	
	return &task, nil
}



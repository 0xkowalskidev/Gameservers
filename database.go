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

	"0xkowalskidev/gameservers/models"
	"0xkowalskidev/gameservers/services"
)

// DatabaseError is now in models package

type DatabaseManager struct {
	db *sql.DB
}

func NewDatabaseManager(dbPath string) (*DatabaseManager, error) {
	log.Info().Str("db_path", dbPath).Msg("Connecting to database")

	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		log.Error().Err(err).Str("db_path", dbPath).Msg("Failed to open database")
		return nil, &models.DatabaseError{Op: "db", Msg: "failed to open database", Err: err}
	}

	if err := db.Ping(); err != nil {
		log.Error().Err(err).Msg("Failed to ping database")
		return nil, &models.DatabaseError{Op: "db", Msg: "failed to ping database", Err: err}
	}

	// Enable foreign key constraints
	if _, err := db.Exec("PRAGMA foreign_keys = ON"); err != nil {
		log.Error().Err(err).Msg("Failed to enable foreign key constraints")
		return nil, &models.DatabaseError{Op: "db", Msg: "failed to enable foreign key constraints", Err: err}
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
		port_mappings TEXT NOT NULL, config_vars TEXT NOT NULL DEFAULT '[]', 
		min_memory_mb INTEGER NOT NULL DEFAULT 512, rec_memory_mb INTEGER NOT NULL DEFAULT 2048,
		created_at DATETIME NOT NULL, updated_at DATETIME NOT NULL
	);
	CREATE TABLE IF NOT EXISTS gameservers (
		id TEXT PRIMARY KEY, name TEXT NOT NULL UNIQUE, game_id TEXT NOT NULL,
		container_id TEXT, status TEXT NOT NULL DEFAULT 'stopped',
		port_mappings TEXT NOT NULL,
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
		return &models.DatabaseError{Op: "db", Msg: "failed to create schema", Err: err}
	}

	// Add new columns if they don't exist (for existing databases)
	alterQueries := []string{
		`ALTER TABLE gameservers ADD COLUMN memory_mb INTEGER DEFAULT 1024`,
		`ALTER TABLE gameservers ADD COLUMN cpu_cores REAL DEFAULT 0`,
		`ALTER TABLE gameservers ADD COLUMN max_backups INTEGER DEFAULT 7`,
		`ALTER TABLE games ADD COLUMN port_mappings TEXT DEFAULT '[]'`,
		`ALTER TABLE gameservers ADD COLUMN port_mappings TEXT DEFAULT '[]'`,
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

	games := []*models.Game{
		{ID: "minecraft", Name: "Minecraft", Image: "ghcr.io/0xkowalskidev/gameservers/minecraft:latest", 
			PortMappings: []models.PortMapping{
				{Name: "game", Protocol: "tcp", ContainerPort: 25565, HostPort: 0},
			},
			ConfigVars: []models.ConfigVar{
				{Name: "EULA", DisplayName: "Accept Minecraft EULA", Required: true, Default: "true", Description: "You must accept the Minecraft End User License Agreement to run a server"},
				{Name: "SERVER_NAME", DisplayName: "Server Name", Required: false, Default: "A Minecraft Server", Description: "The name shown in server lists"},
				{Name: "MOTD", DisplayName: "Message of the Day", Required: false, Default: "Welcome to our server!", Description: "Message shown to players when joining"},
				{Name: "DIFFICULTY", DisplayName: "Difficulty", Required: false, Default: "normal", Description: "models.Game difficulty (peaceful, easy, normal, hard)"},
				{Name: "GAMEMODE", DisplayName: "models.Game Mode", Required: false, Default: "survival", Description: "Default game mode (survival, creative, adventure, spectator)"},
			}, MinMemoryMB: 1024, RecMemoryMB: 3072, CreatedAt: time.Now(), UpdatedAt: time.Now()},
		{ID: "cs2", Name: "Counter-Strike 2", Image: "ghcr.io/0xkowalskidev/gameservers/cs2:latest", 
			PortMappings: []models.PortMapping{
				{Name: "game", Protocol: "tcp", ContainerPort: 27015, HostPort: 0}, 
				{Name: "game", Protocol: "udp", ContainerPort: 27015, HostPort: 0},
			},
			ConfigVars: []models.ConfigVar{
				{Name: "HOSTNAME", DisplayName: "Server Name", Required: false, Default: "CS2 Server", Description: "Server hostname shown in browser"},
				{Name: "RCON_PASSWORD", DisplayName: "RCON Password", Required: true, Default: "", Description: "Password for remote console access (required)"},
				{Name: "SERVER_PASSWORD", DisplayName: "Server Password", Required: false, Default: "", Description: "Password to join server (leave empty for public)"},
				{Name: "MAXPLAYERS", DisplayName: "Max Players", Required: false, Default: "10", Description: "Maximum number of players"},
			}, MinMemoryMB: 2048, RecMemoryMB: 4096, CreatedAt: time.Now(), UpdatedAt: time.Now()},
		{ID: "valheim", Name: "Valheim", Image: "ghcr.io/0xkowalskidev/gameservers/valheim:latest", 
			PortMappings: []models.PortMapping{
				{Name: "game", Protocol: "udp", ContainerPort: 2456, HostPort: 0}, 
				{Name: "query", Protocol: "udp", ContainerPort: 2457, HostPort: 0},
			},
			ConfigVars: []models.ConfigVar{
				{Name: "SERVER_NAME", DisplayName: "Server Name", Required: false, Default: "Valheim Server", Description: "Name shown in server browser"},
				{Name: "WORLD_NAME", DisplayName: "World Name", Required: false, Default: "Dedicated", Description: "Name of the world save file"},
				{Name: "SERVER_PASSWORD", DisplayName: "Password", Required: true, Default: "", Description: "Server password (required for Valheim)"},
				{Name: "SERVER_PUBLIC", DisplayName: "Public Server", Required: false, Default: "1", Description: "Show in server browser (1=yes, 0=no)"},
			}, MinMemoryMB: 1024, RecMemoryMB: 2048, CreatedAt: time.Now(), UpdatedAt: time.Now()},
		{ID: "terraria", Name: "Terraria", Image: "ghcr.io/0xkowalskidev/gameservers/terraria:latest", 
			PortMappings: []models.PortMapping{
				{Name: "game", Protocol: "tcp", ContainerPort: 7777, HostPort: 0},
			},
			ConfigVars: []models.ConfigVar{
				{Name: "WORLD", DisplayName: "World Name", Required: false, Default: "world", Description: "Name of the world file"},
				{Name: "DIFFICULTY", DisplayName: "Difficulty", Required: false, Default: "1", Description: "World difficulty (0=Classic, 1=Expert, 2=Master)"},
				{Name: "MAXPLAYERS", DisplayName: "Max Players", Required: false, Default: "8", Description: "Maximum number of players"},
				{Name: "PASSWORD", DisplayName: "Server Password", Required: false, Default: "", Description: "Password to join server (leave empty for public)"},
			}, MinMemoryMB: 512, RecMemoryMB: 1024, CreatedAt: time.Now(), UpdatedAt: time.Now()},
		{ID: "garrysmod", Name: "Garry's Mod", Image: "ghcr.io/0xkowalskidev/gameservers/garrysmod:latest", 
			PortMappings: []models.PortMapping{
				{Name: "game", Protocol: "tcp", ContainerPort: 27015, HostPort: 0}, 
				{Name: "game", Protocol: "udp", ContainerPort: 27015, HostPort: 0},
			},
			ConfigVars: []models.ConfigVar{
				{Name: "NAME", DisplayName: "Server Name", Required: false, Default: "Garry's Mod Server", Description: "Server name shown in browser"},
				{Name: "RCON_PASSWORD", DisplayName: "RCON Password", Required: true, Default: "", Description: "Password for remote console access (required)"},
				{Name: "PASSWORD", DisplayName: "Server Password", Required: false, Default: "", Description: "Password to join server (leave empty for public)"},
				{Name: "MAXPLAYERS", DisplayName: "Max Players", Required: false, Default: "16", Description: "Maximum number of players"},
				{Name: "MAP", DisplayName: "Default Map", Required: false, Default: "gm_construct", Description: "Starting map"},
				{Name: "GAMEMODE", DisplayName: "models.Game Mode", Required: false, Default: "sandbox", Description: "Default game mode"},
				{Name: "WORKSHOP_ID", DisplayName: "Workshop Collection", Required: false, Default: "", Description: "Steam Workshop collection ID (optional)"},
				{Name: "STEAM_AUTHKEY", DisplayName: "Steam Auth Key", Required: false, Default: "", Description: "Steam Web API key for Workshop content"},
			}, MinMemoryMB: 1024, RecMemoryMB: 2048, CreatedAt: time.Now(), UpdatedAt: time.Now()},
	}

	for _, game := range games {
		if err := dm.CreateGame(game); err != nil {
			return err
		}
	}

	log.Info().Int("count", len(games)).Msg("Seeded games")
	return nil
}

func (dm *DatabaseManager) CreateGame(game *models.Game) error {
	portMappingsJSON, err := json.Marshal(game.PortMappings)
	if err != nil {
		return &models.DatabaseError{Op: "db", Msg: "failed to marshal port mappings", Err: err}
	}

	configVarsJSON, err := json.Marshal(game.ConfigVars)
	if err != nil {
		return &models.DatabaseError{Op: "db", Msg: "failed to marshal config vars", Err: err}
	}

	_, err = dm.db.Exec(`INSERT INTO games (id, name, image, port_mappings, config_vars, min_memory_mb, rec_memory_mb, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		game.ID, game.Name, game.Image, string(portMappingsJSON), string(configVarsJSON), game.MinMemoryMB, game.RecMemoryMB, game.CreatedAt, game.UpdatedAt)

	if err != nil {
		return &models.DatabaseError{Op: fmt.Sprintf("failed to insert game %s", game.Name), Err: err}
	}
	return nil
}

func (dm *DatabaseManager) GetGame(id string) (*models.Game, error) {
	row := dm.db.QueryRow(`SELECT id, name, image, port_mappings, config_vars, min_memory_mb, rec_memory_mb, created_at, updated_at FROM games WHERE id = ?`, id)
	return dm.scanGame(row)
}

func (dm *DatabaseManager) ListGames() ([]*models.Game, error) {
	rows, err := dm.db.Query(`SELECT id, name, image, port_mappings, config_vars, min_memory_mb, rec_memory_mb, created_at, updated_at FROM games ORDER BY name`)
	if err != nil {
		return nil, &models.DatabaseError{Op: "db", Msg: "failed to query games", Err: err}
	}
	defer rows.Close()

	var games []*models.Game
	for rows.Next() {
		game, err := dm.scanGame(rows)
		if err != nil {
			return nil, &models.DatabaseError{Op: "db", Msg: "failed to scan game", Err: err}
		}
		games = append(games, game)
	}
	return games, nil
}

func (dm *DatabaseManager) UpdateGame(game *models.Game) error {
	portMappingsJSON, err := json.Marshal(game.PortMappings)
	if err != nil {
		return &models.DatabaseError{Op: "db", Msg: "failed to marshal port mappings", Err: err}
	}

	_, err = dm.db.Exec(`UPDATE games SET name = ?, image = ?, port_mappings = ?, updated_at = ? WHERE id = ?`,
		game.Name, game.Image, string(portMappingsJSON), game.UpdatedAt, game.ID)

	if err != nil {
		return &models.DatabaseError{Op: fmt.Sprintf("failed to update game %s", game.ID), Err: err}
	}
	return nil
}

func (dm *DatabaseManager) DeleteGame(id string) error {
	_, err := dm.db.Exec(`DELETE FROM games WHERE id = ?`, id)
	if err != nil {
		return &models.DatabaseError{Op: fmt.Sprintf("failed to delete game %s", id),Err: err}
	}
	return nil
}

func (dm *DatabaseManager) CreateGameserver(server *models.Gameserver) error {
	envJSON, _ := json.Marshal(server.Environment)
	volumesJSON, _ := json.Marshal(server.Volumes)
	portMappingsJSON, _ := json.Marshal(server.PortMappings)

	_, err := dm.db.Exec(`INSERT INTO gameservers (id, name, game_id, container_id, status, port_mappings, memory_mb, cpu_cores, max_backups, environment, volumes, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		server.ID, server.Name, server.GameID, server.ContainerID, server.Status, string(portMappingsJSON), server.MemoryMB, server.CPUCores, server.MaxBackups, string(envJSON), string(volumesJSON), server.CreatedAt, server.UpdatedAt)

	if err != nil {
		log.Error().Err(err).Str("gameserver_id", server.ID).Str("name", server.Name).Msg("Failed to create gameserver in database")
		return &models.DatabaseError{Op: fmt.Sprintf("failed to insert gameserver %s", server.Name), Err: err}
	}

	return nil
}

func (dm *DatabaseManager) scanGame(row interface{ Scan(...interface{}) error }) (*models.Game, error) {
	var game models.Game
	var portMappingsJSON, configVarsJSON string
	err := row.Scan(&game.ID, &game.Name, &game.Image, &portMappingsJSON, &configVarsJSON, &game.MinMemoryMB, &game.RecMemoryMB, &game.CreatedAt, &game.UpdatedAt)
	if err != nil {
		return nil, err
	}
	
	if err := json.Unmarshal([]byte(portMappingsJSON), &game.PortMappings); err != nil {
		return nil, &models.DatabaseError{Op: "db", Msg: "failed to unmarshal port mappings", Err: err}
	}

	if err := json.Unmarshal([]byte(configVarsJSON), &game.ConfigVars); err != nil {
		return nil, &models.DatabaseError{Op: "db", Msg: "failed to unmarshal config vars", Err: err}
	}
	
	return &game, nil
}

func (dm *DatabaseManager) scanGameserver(row interface{ Scan(...interface{}) error }) (*models.Gameserver, error) {
	var server models.Gameserver
	var envJSON, volumesJSON, portMappingsJSON string

	err := row.Scan(&server.ID, &server.Name, &server.GameID, &server.ContainerID, &server.Status, &portMappingsJSON, &server.MemoryMB, &server.CPUCores, &server.MaxBackups, &envJSON, &volumesJSON, &server.CreatedAt, &server.UpdatedAt)
	if err != nil {
		return nil, err
	}

	json.Unmarshal([]byte(envJSON), &server.Environment)
	json.Unmarshal([]byte(volumesJSON), &server.Volumes)
	json.Unmarshal([]byte(portMappingsJSON), &server.PortMappings)
	return &server, nil
}

func (dm *DatabaseManager) GetGameserver(id string) (*models.Gameserver, error) {
	row := dm.db.QueryRow(`SELECT id, name, game_id, container_id, status, port_mappings, memory_mb, cpu_cores, max_backups, environment, volumes, created_at, updated_at FROM gameservers WHERE id = ?`, id)
	server, err := dm.scanGameserver(row)
	if err == sql.ErrNoRows {
		return nil, &models.DatabaseError{Op: "error", Msg: fmt.Sprintf("gameserver %s not found", id), Err: nil}
	}
	if err != nil {
		return nil, &models.DatabaseError{Op: fmt.Sprintf("failed to query gameserver %s", id),Err: err}
	}
	return server, nil
}

func (dm *DatabaseManager) UpdateGameserver(server *models.Gameserver) error {
	envJSON, _ := json.Marshal(server.Environment)
	volumesJSON, _ := json.Marshal(server.Volumes)
	portMappingsJSON, _ := json.Marshal(server.PortMappings)
	server.UpdatedAt = time.Now()

	result, err := dm.db.Exec(`UPDATE gameservers SET name = ?, game_id = ?, container_id = ?, status = ?, port_mappings = ?, memory_mb = ?, cpu_cores = ?, max_backups = ?, environment = ?, volumes = ?, updated_at = ? WHERE id = ?`,
		server.Name, server.GameID, server.ContainerID, server.Status, string(portMappingsJSON), server.MemoryMB, server.CPUCores, server.MaxBackups, string(envJSON), string(volumesJSON), server.UpdatedAt, server.ID)

	if err != nil {
		return &models.DatabaseError{Op: fmt.Sprintf("failed to update gameserver %s", server.ID),Err: err}
	}

	if rowsAffected, _ := result.RowsAffected(); rowsAffected == 0 {
		return &models.DatabaseError{Op: "error", Msg: fmt.Sprintf("gameserver %s not found", server.ID), Err: nil}
	}
	return nil
}

func (dm *DatabaseManager) DeleteGameserver(id string) error {
	result, err := dm.db.Exec(`DELETE FROM gameservers WHERE id = ?`, id)
	if err != nil {
		return &models.DatabaseError{Op: fmt.Sprintf("failed to delete gameserver %s", id),Err: err}
	}
	if rowsAffected, _ := result.RowsAffected(); rowsAffected == 0 {
		return &models.DatabaseError{Op: "error", Msg: fmt.Sprintf("gameserver %s not found", id), Err: nil}
	}
	return nil
}

func (dm *DatabaseManager) ListGameservers() ([]*models.Gameserver, error) {
	rows, err := dm.db.Query(`SELECT id, name, game_id, container_id, status, port_mappings, memory_mb, cpu_cores, max_backups, environment, volumes, created_at, updated_at FROM gameservers ORDER BY created_at DESC`)
	if err != nil {
		return nil, &models.DatabaseError{Op: "db", Msg: "failed to query gameservers", Err: err}
	}
	defer rows.Close()

	var servers []*models.Gameserver
	for rows.Next() {
		server, err := dm.scanGameserver(rows)
		if err != nil {
			return nil, &models.DatabaseError{Op: "db", Msg: "failed to scan gameserver row", Err: err}
		}
		servers = append(servers, server)
	}
	return servers, rows.Err()
}

func (dm *DatabaseManager) GetGameserverByContainerID(containerID string) (*models.Gameserver, error) {
	row := dm.db.QueryRow(`SELECT id, name, game_id, container_id, status, port, host_port, memory_mb, cpu_cores, max_backups, environment, volumes, created_at, updated_at FROM gameservers WHERE container_id = ?`, containerID)
	server, err := dm.scanGameserver(row)
	if err == sql.ErrNoRows {
		return nil, &models.DatabaseError{Op: "error", Msg: fmt.Sprintf("gameserver with container %s not found", containerID), Err: nil}
	}
	if err != nil {
		return nil, &models.DatabaseError{Op: fmt.Sprintf("failed to query gameserver by container %s", containerID),Err: err}
	}
	return server, nil
}

type GameserverService struct {
	db           *DatabaseManager
	docker       models.DockerManagerInterface
	portAllocator *models.PortAllocator
}

func NewGameserverService(db *DatabaseManager, docker models.DockerManagerInterface) *GameserverService {
	return &GameserverService{
		db:            db,
		docker:        docker,
		portAllocator: models.NewPortAllocator(),
	}
}

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
	
	server.Status = models.StatusStarting
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
	
	server.Status = models.StatusStopped
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

func (gss *GameserverService) syncStatus(server *models.Gameserver) {
	if server.ContainerID != "" {
		if dockerStatus, err := gss.docker.GetContainerStatus(server.ContainerID); err == nil && server.Status != dockerStatus {
			server.Status, server.UpdatedAt = dockerStatus, time.Now()
			gss.db.UpdateGameserver(server)
		}
	}
}

func (gss *GameserverService) GetGameserver(id string) (*models.Gameserver, error) {
	server, err := gss.db.GetGameserver(id)
	if err != nil {
		return nil, err
	}
	gss.populateGameFields(server)
	gss.syncStatus(server)
	return server, nil
}

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


func (gss *GameserverService) StreamGameserverLogs(id string) (io.ReadCloser, error) {
	server, err := gss.db.GetGameserver(id)
	if err != nil {
		return nil, err
	}
	if server.ContainerID == "" {
		return nil, &models.DatabaseError{Op: "error", Msg: "container not created yet", Err: nil}
	}
	return gss.docker.StreamContainerLogs(server.ContainerID)
}

func (gss *GameserverService) StreamGameserverStats(id string) (io.ReadCloser, error) {
	server, err := gss.db.GetGameserver(id)
	if err != nil {
		return nil, err
	}
	if server.ContainerID == "" {
		return nil, &models.DatabaseError{Op: "error", Msg: "container not created yet", Err: nil}
	}
	return gss.docker.StreamContainerStats(server.ContainerID)
}

func (gss *GameserverService) ListGames() ([]*models.Game, error) {
	return gss.db.ListGames()
}

func (gss *GameserverService) GetGame(id string) (*models.Game, error) {
	return gss.db.GetGame(id)
}

func (gss *GameserverService) CreateGame(game *models.Game) error {
	now := time.Now()
	game.CreatedAt, game.UpdatedAt = now, now
	return gss.db.CreateGame(game)
}

// =============================================================================
// Scheduled Task Service Operations
// =============================================================================

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

func (gss *GameserverService) GetScheduledTask(id string) (*models.ScheduledTask, error) {
	return gss.db.GetScheduledTask(id)
}

func (gss *GameserverService) UpdateScheduledTask(task *models.ScheduledTask) error {
	task.UpdatedAt = time.Now()
	// Clear next run time so scheduler will recalculate it
	task.NextRun = nil
	return gss.db.UpdateScheduledTask(task)
}

func (gss *GameserverService) DeleteScheduledTask(id string) error {
	return gss.db.DeleteScheduledTask(id)
}

func (gss *GameserverService) ListScheduledTasksForGameserver(gameserverID string) ([]*models.ScheduledTask, error) {
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

func (gss *GameserverService) ListFiles(containerID string, path string) ([]*models.FileInfo, error) {
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

func (gss *GameserverService) UploadFile(containerID string, destPath string, reader io.Reader) error {
	return gss.docker.UploadFile(containerID, destPath, reader)
}

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


// =============================================================================
// Scheduled Task Database Operations
// =============================================================================

func (dm *DatabaseManager) CreateScheduledTask(task *models.ScheduledTask) error {
	query := `INSERT INTO scheduled_tasks (id, gameserver_id, name, type, status, cron_schedule, created_at, updated_at, last_run, next_run) 
			  VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`
	
	_, err := dm.db.Exec(query, task.ID, task.GameserverID, task.Name, string(task.Type), string(task.Status), 
		task.CronSchedule, task.CreatedAt, task.UpdatedAt, task.LastRun, task.NextRun)
	
	if err != nil {
		return &models.DatabaseError{Op: "create_task", Msg: "failed to create scheduled task", Err: err}
	}
	return nil
}

func (dm *DatabaseManager) GetScheduledTask(id string) (*models.ScheduledTask, error) {
	query := `SELECT id, gameserver_id, name, type, status, cron_schedule, created_at, updated_at, last_run, next_run 
			  FROM scheduled_tasks WHERE id = ?`
	
	row := dm.db.QueryRow(query, id)
	task, err := dm.scanScheduledTask(row)
	if err == sql.ErrNoRows {
		return nil, &models.DatabaseError{Op: "get_task", Msg: fmt.Sprintf("scheduled task %s not found", id), Err: nil}
	}
	if err != nil {
		return nil, &models.DatabaseError{Op: "get_task", Msg: fmt.Sprintf("failed to query scheduled task %s", id), Err: err}
	}
	return task, nil
}

func (dm *DatabaseManager) UpdateScheduledTask(task *models.ScheduledTask) error {
	query := `UPDATE scheduled_tasks SET name = ?, type = ?, status = ?, cron_schedule = ?, updated_at = ?, last_run = ?, next_run = ? 
			  WHERE id = ?`
	
	result, err := dm.db.Exec(query, task.Name, string(task.Type), string(task.Status), task.CronSchedule, 
		task.UpdatedAt, task.LastRun, task.NextRun, task.ID)
	
	if err != nil {
		return &models.DatabaseError{Op: "update_task", Msg: "failed to update scheduled task", Err: err}
	}
	if rowsAffected, _ := result.RowsAffected(); rowsAffected == 0 {
		return &models.DatabaseError{Op: "update_task", Msg: fmt.Sprintf("scheduled task %s not found", task.ID), Err: nil}
	}
	return nil
}

func (dm *DatabaseManager) DeleteScheduledTask(id string) error {
	query := `DELETE FROM scheduled_tasks WHERE id = ?`
	
	result, err := dm.db.Exec(query, id)
	if err != nil {
		return &models.DatabaseError{Op: "delete_task", Msg: "failed to delete scheduled task", Err: err}
	}
	if rowsAffected, _ := result.RowsAffected(); rowsAffected == 0 {
		return &models.DatabaseError{Op: "delete_task", Msg: fmt.Sprintf("scheduled task %s not found", id), Err: nil}
	}
	return nil
}

func (dm *DatabaseManager) ListScheduledTasksForGameserver(gameserverID string) ([]*models.ScheduledTask, error) {
	query := `SELECT id, gameserver_id, name, type, status, cron_schedule, created_at, updated_at, last_run, next_run 
			  FROM scheduled_tasks WHERE gameserver_id = ? ORDER BY created_at DESC`
	
	rows, err := dm.db.Query(query, gameserverID)
	if err != nil {
		return nil, &models.DatabaseError{Op: "list_tasks", Msg: "failed to query scheduled tasks", Err: err}
	}
	defer rows.Close()

	var tasks []*models.ScheduledTask
	for rows.Next() {
		task, err := dm.scanScheduledTask(rows)
		if err != nil {
			return nil, &models.DatabaseError{Op: "list_tasks", Msg: "failed to scan scheduled task row", Err: err}
		}
		tasks = append(tasks, task)
	}
	return tasks, rows.Err()
}

func (dm *DatabaseManager) ListActiveScheduledTasks() ([]*models.ScheduledTask, error) {
	query := `SELECT id, gameserver_id, name, type, status, cron_schedule, created_at, updated_at, last_run, next_run 
			  FROM scheduled_tasks WHERE status = ? ORDER BY next_run ASC`
	
	rows, err := dm.db.Query(query, string(models.TaskStatusActive))
	if err != nil {
		return nil, &models.DatabaseError{Op: "list_active_tasks", Msg: "failed to query active scheduled tasks", Err: err}
	}
	defer rows.Close()

	var tasks []*models.ScheduledTask
	for rows.Next() {
		task, err := dm.scanScheduledTask(rows)
		if err != nil {
			return nil, &models.DatabaseError{Op: "list_active_tasks", Msg: "failed to scan scheduled task row", Err: err}
		}
		tasks = append(tasks, task)
	}
	return tasks, rows.Err()
}

type ScheduledTaskScanner interface {
	Scan(dest ...interface{}) error
}

func (dm *DatabaseManager) scanScheduledTask(row ScheduledTaskScanner) (*models.ScheduledTask, error) {
	var task models.ScheduledTask
	var taskType, status string
	var lastRun, nextRun sql.NullTime
	
	err := row.Scan(&task.ID, &task.GameserverID, &task.Name, &taskType, &status, 
		&task.CronSchedule, &task.CreatedAt, &task.UpdatedAt, &lastRun, &nextRun)
	
	if err != nil {
		return nil, err
	}
	
	task.Type = models.TaskType(taskType)
	task.Status = models.TaskStatus(status)
	
	if lastRun.Valid {
		task.LastRun = &lastRun.Time
	}
	if nextRun.Valid {
		task.NextRun = &nextRun.Time
	}
	
	return &task, nil
}



package database

import (
	"database/sql"
	"strings"
	"time"

	_ "github.com/mattn/go-sqlite3"
	"github.com/rs/zerolog/log"

	"0xkowalskidev/gameservers/models"
)

// DatabaseManager manages SQLite database operations
type DatabaseManager struct {
	db *sql.DB
}

// NewDatabaseManager creates a new database manager and performs migrations
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

// Close closes the database connection
func (dm *DatabaseManager) Close() error {
	return dm.db.Close()
}

// migrate creates the database schema
func (dm *DatabaseManager) migrate() error {
	schema := `
	CREATE TABLE IF NOT EXISTS games (
		id TEXT PRIMARY KEY, name TEXT NOT NULL UNIQUE, slug TEXT NOT NULL DEFAULT '', image TEXT NOT NULL,
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
		`ALTER TABLE games ADD COLUMN slug TEXT DEFAULT ''`,
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

// seedGames adds default game configurations to the database
func (dm *DatabaseManager) seedGames() error {
	// Check if games already exist
	count := 0
	row := dm.db.QueryRow("SELECT COUNT(*) FROM games")
	row.Scan(&count)
	if count > 0 {
		return nil // Games already seeded
	}

	games := []*models.Game{
		{ID: "minecraft", Name: "Minecraft", Slug: "minecraft", Image: "ghcr.io/0xkowalskidev/gameservers/minecraft:latest",
			PortMappings: []models.PortMapping{
				{Name: "game", Protocol: "tcp", ContainerPort: 25565, HostPort: 0},
			},
			ConfigVars: []models.ConfigVar{
				{Name: "MINECRAFT_VERSION", DisplayName: "Minecraft Version", Required: false, Default: "latest", Description: "Server version (latest recommended, or specific version like 1.21.6 for mod compatibility)"},
				{Name: "EULA", DisplayName: "Accept Minecraft EULA", Required: true, Default: "true", Description: "You must accept the Minecraft End User License Agreement to run a server"},
				{Name: "SERVER_NAME", DisplayName: "Server Name", Required: false, Default: "A Minecraft Server", Description: "The name shown in server lists"},
				{Name: "MOTD", DisplayName: "Message of the Day", Required: false, Default: "Welcome to our server!", Description: "Message shown to players when joining"},
				{Name: "DIFFICULTY", DisplayName: "Difficulty", Required: false, Default: "normal", Description: "Game difficulty (peaceful, easy, normal, hard)"},
				{Name: "GAMEMODE", DisplayName: "Game Mode", Required: false, Default: "survival", Description: "Default game mode (survival, creative, adventure, spectator)"},
			}, MinMemoryMB: 1024, RecMemoryMB: 3072, CreatedAt: time.Now(), UpdatedAt: time.Now()},
		{ID: "cs2", Name: "Counter-Strike 2", Slug: "counter-strike-2", Image: "ghcr.io/0xkowalskidev/gameservers/cs2:latest",
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
		{ID: "valheim", Name: "Valheim", Slug: "valheim", Image: "ghcr.io/0xkowalskidev/gameservers/valheim:latest",
			PortMappings: []models.PortMapping{
				{Name: "game", Protocol: "udp", ContainerPort: 2456, HostPort: 0},
				{Name: "query", Protocol: "udp", ContainerPort: 2457, HostPort: 0},
			},
			ConfigVars: []models.ConfigVar{
				{Name: "SERVER_NAME", DisplayName: "Server Name", Required: false, Default: "My Valheim Server", Description: "The name of your Valheim server"},
				{Name: "PASSWORD", DisplayName: "Server Password", Required: true, Default: "valheim123", Description: "Password to join server (minimum 5 characters required)"},
				{Name: "PUBLIC", DisplayName: "Public Server", Required: false, Default: "1", Description: "Whether to list server publicly (1 for yes, 0 for no)"},
				{Name: "CROSSPLAY", DisplayName: "Enable Crossplay", Required: false, Default: "1", Description: "Enable crossplay between Steam and Xbox (1 for yes, 0 for no)"},
			}, MinMemoryMB: 2048, RecMemoryMB: 4096, CreatedAt: time.Now(), UpdatedAt: time.Now()},
		{ID: "terraria", Name: "Terraria", Slug: "terraria", Image: "ghcr.io/0xkowalskidev/gameservers/terraria:latest",
			PortMappings: []models.PortMapping{
				{Name: "game", Protocol: "tcp", ContainerPort: 7777, HostPort: 0},
			},
			ConfigVars: []models.ConfigVar{
				{Name: "WORLD_NAME", DisplayName: "World Name", Required: false, Default: "World", Description: "The name of the Terraria world"},
				{Name: "MAX_PLAYERS", DisplayName: "Max Players", Required: false, Default: "8", Description: "Maximum number of players"},
				{Name: "SERVER_PASSWORD", DisplayName: "Server Password", Required: false, Default: "", Description: "Password to join server (leave empty for public)"},
				{Name: "DIFFICULTY", DisplayName: "Difficulty", Required: false, Default: "1", Description: "World difficulty (0=Classic, 1=Expert, 2=Master)"},
			}, MinMemoryMB: 1024, RecMemoryMB: 2048, CreatedAt: time.Now(), UpdatedAt: time.Now()},
		{ID: "garrysmod", Name: "Garry's Mod", Slug: "garrys-mod", Image: "ghcr.io/0xkowalskidev/gameservers/garrysmod:latest",
			PortMappings: []models.PortMapping{
				{Name: "game", Protocol: "tcp", ContainerPort: 27015, HostPort: 0},
				{Name: "game", Protocol: "udp", ContainerPort: 27015, HostPort: 0},
			},
			ConfigVars: []models.ConfigVar{
				{Name: "HOSTNAME", DisplayName: "Server Name", Required: false, Default: "Garry's Mod Server", Description: "Server hostname shown in browser"},
				{Name: "GAMEMODE", DisplayName: "Game Mode", Required: false, Default: "sandbox", Description: "Game mode to run (sandbox, darkrp, etc.)"},
				{Name: "MAP", DisplayName: "Starting Map", Required: false, Default: "gm_flatgrass", Description: "The map to load on server start"},
				{Name: "MAXPLAYERS", DisplayName: "Max Players", Required: false, Default: "16", Description: "Maximum number of players"},
				{Name: "SERVER_PASSWORD", DisplayName: "Server Password", Required: false, Default: "", Description: "Password to join server (leave empty for public)"},
			}, MinMemoryMB: 2048, RecMemoryMB: 4096, CreatedAt: time.Now(), UpdatedAt: time.Now()},
		{ID: "palworld", Name: "Palworld", Slug: "palworld", Image: "ghcr.io/0xkowalskidev/gameservers/palworld:latest",
			PortMappings: []models.PortMapping{
				{Name: "game", Protocol: "udp", ContainerPort: 8211, HostPort: 0},
			},
			ConfigVars: []models.ConfigVar{
				{Name: "SERVER_NAME", DisplayName: "Server Name", Required: false, Default: "Palworld Server", Description: "The name of your Palworld server"},
				{Name: "MAX_PLAYERS", DisplayName: "Max Players", Required: false, Default: "32", Description: "Maximum number of players"},
				{Name: "SERVER_PASSWORD", DisplayName: "Server Password", Required: false, Default: "", Description: "Password to join server (leave empty for public)"},
				{Name: "ADMIN_PASSWORD", DisplayName: "Admin Password", Required: false, Default: "", Description: "Password for admin access"},
			}, MinMemoryMB: 8192, RecMemoryMB: 16384, CreatedAt: time.Now(), UpdatedAt: time.Now()},
		{ID: "rust", Name: "Rust", Slug: "rust", Image: "ghcr.io/0xkowalskidev/gameservers/rust:latest",
			PortMappings: []models.PortMapping{
				{Name: "game", Protocol: "udp", ContainerPort: 28015, HostPort: 0},
				{Name: "rcon", Protocol: "tcp", ContainerPort: 28016, HostPort: 0},
				{Name: "rcon", Protocol: "udp", ContainerPort: 28016, HostPort: 0},
				{Name: "query", Protocol: "udp", ContainerPort: 28017, HostPort: 0},
			},
			ConfigVars: []models.ConfigVar{
				{Name: "NAME", DisplayName: "Server Name", Required: false, Default: "Rust Server", Description: "The name of your Rust server"},
				{Name: "MAXPLAYERS", DisplayName: "Max Players", Required: false, Default: "50", Description: "Maximum number of players"},
				{Name: "WORLDSIZE", DisplayName: "World Size", Required: false, Default: "3000", Description: "Size of the world map (1000-4000)"},
				{Name: "SEED", DisplayName: "World Seed", Required: false, Default: "12345", Description: "Seed for world generation (numeric value)"},
				{Name: "PASSWORD", DisplayName: "Server Password", Required: false, Default: "", Description: "Password to join server (leave empty for public)"},
				{Name: "RCON_PASSWORD", DisplayName: "RCON Password", Required: false, Default: "", Description: "Password for remote console access"},
				{Name: "TICKRATE", DisplayName: "Tick Rate", Required: false, Default: "30", Description: "Server tick rate (10-30, higher = better performance)"},
				{Name: "SAVEINTERVAL", DisplayName: "Save Interval", Required: false, Default: "300", Description: "How often to save the world (in seconds)"},
				{Name: "UPDATE_ON_START", DisplayName: "Update on Start", Required: false, Default: "false", Description: "Update server files on container start"},
			}, MinMemoryMB: 4096, RecMemoryMB: 8192, CreatedAt: time.Now(), UpdatedAt: time.Now()},
	}

	for _, game := range games {
		if err := dm.CreateGame(game); err != nil {
			log.Error().Err(err).Str("game_id", game.ID).Msg("Failed to seed game")
			return err
		}
	}

	log.Info().Int("count", len(games)).Msg("Games seeded successfully")
	return nil
}

package database

import (
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"github.com/rs/zerolog/log"

	"0xkowalskidev/gameservers/models"
)

// DatabaseManager manages GORM database operations
type DatabaseManager struct {
	db *gorm.DB
}

// NewDatabaseManager creates a new database manager and performs migrations
func NewDatabaseManager(dbPath string) (*DatabaseManager, error) {
	log.Info().Str("db_path", dbPath).Msg("Connecting to database")

	// Configure GORM with SQLite driver
	db, err := gorm.Open(sqlite.Open(dbPath), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent), // Use silent mode to avoid double logging
	})
	if err != nil {
		log.Error().Err(err).Str("db_path", dbPath).Msg("Failed to open database")
		return nil, &models.DatabaseError{Op: "db", Msg: "failed to open database", Err: err}
	}

	// Get underlying SQL DB for configuration
	sqlDB, err := db.DB()
	if err != nil {
		log.Error().Err(err).Msg("Failed to get underlying SQL DB")
		return nil, &models.DatabaseError{Op: "db", Msg: "failed to get underlying SQL DB", Err: err}
	}

	// Configure connection pool
	sqlDB.SetMaxIdleConns(10)
	sqlDB.SetMaxOpenConns(100)

	// Enable foreign key constraints
	if err := db.Exec("PRAGMA foreign_keys = ON").Error; err != nil {
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
	sqlDB, err := dm.db.DB()
	if err != nil {
		return err
	}
	return sqlDB.Close()
}

// DB returns the GORM DB instance
func (dm *DatabaseManager) DB() *gorm.DB {
	return dm.db
}

// migrate performs auto-migration of models
func (dm *DatabaseManager) migrate() error {
	err := dm.db.AutoMigrate(
		&models.Game{},
		&models.Gameserver{},
		&models.ScheduledTask{},
	)
	if err != nil {
		return &models.DatabaseError{Op: "db", Msg: "failed to auto-migrate", Err: err}
	}

	return nil
}

// seedGames adds default game configurations to the database
func (dm *DatabaseManager) seedGames() error {
	// Check if games already exist
	var count int64
	if err := dm.db.Model(&models.Game{}).Count(&count).Error; err != nil {
		return &models.DatabaseError{Op: "db", Msg: "failed to count games", Err: err}
	}
	if count > 0 {
		return nil // Games already seeded
	}

	games := []*models.Game{
		{ID: "minecraft", Name: "Minecraft", Slug: "minecraft", Image: "ghcr.io/0xkowalskidev/gameservers/minecraft:latest",
			IconPath: "/static/games/minecraft/minecraft-icon.ico", GridImagePath: "/static/games/minecraft/minecraft-grid.png",
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
				{Name: "MAX_PLAYERS", DisplayName: "Max Players", Required: false, Default: "20", Description: "Maximum number of players that can join"},
				{Name: "VIEW_DISTANCE", DisplayName: "View Distance", Required: false, Default: "10", Description: "Chunk render distance (3-32, lower = better performance)"},
				{Name: "PVP", DisplayName: "PvP Combat", Required: false, Default: "true", Description: "Allow players to damage each other"},
				{Name: "WHITELIST", DisplayName: "Whitelist", Required: false, Default: "false", Description: "Only allow approved players to join"},
			}, MinMemoryMB: 1024, RecMemoryMB: 3072},
		{ID: "valheim", Name: "Valheim", Slug: "valheim", Image: "ghcr.io/0xkowalskidev/gameservers/valheim:latest",
			IconPath: "/static/games/valheim/valheim-icon.ico", GridImagePath: "/static/games/valheim/valheim-grid.png",
			PortMappings: []models.PortMapping{
				{Name: "game", Protocol: "udp", ContainerPort: 2456, HostPort: 0},
				{Name: "query", Protocol: "udp", ContainerPort: 2457, HostPort: 0},
			},
			ConfigVars: []models.ConfigVar{
				{Name: "SERVER_NAME", DisplayName: "Server Name", Required: false, Default: "My Valheim Server", Description: "The name of your Valheim server"},
				{Name: "PASSWORD", DisplayName: "Server Password", Required: true, Default: "valheim123", Description: "Password to join server (minimum 5 characters required)"},
				{Name: "PUBLIC", DisplayName: "Public Server", Required: false, Default: "1", Description: "Whether to list server publicly (1 for yes, 0 for no)"},
				{Name: "CROSSPLAY", DisplayName: "Enable Crossplay", Required: false, Default: "1", Description: "Enable crossplay between Steam and Xbox (1 for yes, 0 for no)"},
			}, MinMemoryMB: 2048, RecMemoryMB: 4096},
		{ID: "terraria", Name: "Terraria", Slug: "terraria", Image: "ghcr.io/0xkowalskidev/gameservers/terraria:latest",
			IconPath: "/static/games/terraria/terraria-icon.ico", GridImagePath: "/static/games/terraria/terraria-grid.png",
			PortMappings: []models.PortMapping{
				{Name: "game", Protocol: "tcp", ContainerPort: 7777, HostPort: 0},
			},
			ConfigVars: []models.ConfigVar{
				{Name: "WORLD_NAME", DisplayName: "World Name", Required: false, Default: "World", Description: "The name of the Terraria world"},
				{Name: "MAX_PLAYERS", DisplayName: "Max Players", Required: false, Default: "8", Description: "Maximum number of players"},
				{Name: "SERVER_PASSWORD", DisplayName: "Server Password", Required: false, Default: "", Description: "Password to join server (leave empty for public)"},
				{Name: "DIFFICULTY", DisplayName: "Difficulty", Required: false, Default: "1", Description: "World difficulty (0=Classic, 1=Expert, 2=Master)"},
			}, MinMemoryMB: 1024, RecMemoryMB: 2048},
		{ID: "garrysmod", Name: "Garry's Mod", Slug: "garrys-mod", Image: "ghcr.io/0xkowalskidev/gameservers/garrysmod:latest",
			IconPath: "/static/games/garrysmod/garrys-mod-icon.ico", GridImagePath: "/static/games/garrysmod/garrys-mod-grid.png",
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
			}, MinMemoryMB: 2048, RecMemoryMB: 4096},
		{ID: "palworld", Name: "Palworld", Slug: "palworld", Image: "ghcr.io/0xkowalskidev/gameservers/palworld:latest",
			IconPath: "/static/games/palworld/palworld-icon.ico", GridImagePath: "/static/games/palworld/palworld-grid.png",
			PortMappings: []models.PortMapping{
				{Name: "game", Protocol: "udp", ContainerPort: 8211, HostPort: 0},
				{Name: "rest_api", Protocol: "tcp", ContainerPort: 8212, HostPort: 0},
			},
			ConfigVars: []models.ConfigVar{
				{Name: "SERVER_NAME", DisplayName: "Server Name", Required: false, Default: "Palworld Server", Description: "The name of your Palworld server"},
				{Name: "MAX_PLAYERS", DisplayName: "Max Players", Required: false, Default: "32", Description: "Maximum number of players"},
				{Name: "SERVER_PASSWORD", DisplayName: "Server Password", Required: false, Default: "", Description: "Password to join server (leave empty for public)"},
				{Name: "ADMIN_PASSWORD", DisplayName: "Admin Password", Required: false, Default: "", Description: "Password for admin access"},
			}, MinMemoryMB: 8192, RecMemoryMB: 16384},
		{ID: "rust", Name: "Rust", Slug: "rust", Image: "ghcr.io/0xkowalskidev/gameservers/rust:latest",
			IconPath: "/static/games/rust/rust-icon.ico", GridImagePath: "/static/games/rust/rust-grid.png",
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
			}, MinMemoryMB: 4096, RecMemoryMB: 8192},
	}

	for _, game := range games {
		if err := dm.db.Create(game).Error; err != nil {
			log.Error().Err(err).Str("game_id", game.ID).Msg("Failed to seed game")
			return &models.DatabaseError{Op: "db", Msg: "failed to create game", Err: err}
		}
	}

	log.Info().Int("count", len(games)).Msg("Games seeded successfully")
	return nil
}

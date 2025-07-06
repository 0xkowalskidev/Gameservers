package main

import (
	"context"
	"embed"
	"fmt"
	"html/template"
	"io/fs"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	"0xkowalskidev/gameservers/database"
	"0xkowalskidev/gameservers/docker"
	"0xkowalskidev/gameservers/handlers"
	"0xkowalskidev/gameservers/services"
)

//go:embed templates/*.html
var templateFiles embed.FS

//go:embed static/*
var staticFiles embed.FS

// Config holds all configuration for the application
type Config struct {
	// Server Configuration
	Host            string
	Port            int
	ShutdownTimeout time.Duration

	// Database Configuration
	DatabasePath string

	// Docker Configuration
	DockerSocket         string
	ContainerNamespace   string
	ContainerStopTimeout time.Duration

	// File System Limits
	MaxFileEditSize int64
	MaxUploadSize   int64
}

func main() {
	// Load configuration
	config := loadConfig()
	log.Info().Interface("config", config).Msg("Configuration loaded")

	// Setup logging
	log.Logger = log.Output(zerolog.ConsoleWriter{
		Out:        os.Stderr,
		TimeFormat: time.RFC3339,
	})
	zerolog.SetGlobalLevel(zerolog.InfoLevel)
	if os.Getenv("DEBUG") != "" {
		zerolog.SetGlobalLevel(zerolog.DebugLevel)
	}
	log.Info().Msg("Logger initialized")

	// Initialize database
	db, err := database.NewDatabaseManager(config.DatabasePath)
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to initialize database")
	}
	defer db.Close()
	log.Info().Msg("Database initialized successfully")

	// Initialize Docker manager
	dockerManager, err := docker.NewDockerManager(config.DockerSocket, config.ContainerNamespace, config.ContainerStopTimeout)
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to initialize Docker manager")
	}
	log.Info().Msg("Docker manager initialized successfully")

	// Initialize domain-specific services
	gameService := services.NewGameService(db)
	gameserverService := services.NewGameserverService(db, gameService, dockerManager, "/data")
	log.Info().Msg("Domain services initialized")

	// Initialize and start task scheduler
	taskScheduler := services.NewTaskScheduler(db, gameserverService)
	taskScheduler.Start()
	log.Info().Msg("Task scheduler started")

	// Ensure scheduler is stopped when application exits
	defer taskScheduler.Stop()

	// Parse html templates without custom functions
	tmpl, err := template.New("").ParseFS(templateFiles, "templates/*.html")
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to parse templates")
	}
	log.Info().Msg("Templates parsed successfully")

	// Setup static fs
	staticFS, err := fs.Sub(staticFiles, "static")
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to setup static files")
	}

	// Initialize domain-specific services
	taskService := services.NewTaskService(db)
	fileService := services.NewFileService(dockerManager)
	backupService := services.NewBackupService(dockerManager, db)
	log.Info().Msg("Domain-specific services initialized")

	// Initialize base handlers
	baseHandlers := handlers.NewBaseHandlers(tmpl)

	// Initialize domain-specific handlers
	dashboardHandlers := &handlers.DashboardHandlers{BaseHandlers: baseHandlers}
	gameHandlers := handlers.NewGameHandlers(baseHandlers, gameService, gameserverService)
	gameserverHandlers := handlers.NewGameserverHandlers(baseHandlers, gameserverService, gameService)
	consoleHandlers := handlers.NewConsoleHandlers(baseHandlers, gameserverService)
	fileHandlers := handlers.NewFileHandlers(baseHandlers, gameserverService, fileService, config.MaxFileEditSize, config.MaxUploadSize)
	taskHandlers := handlers.NewTaskHandlers(baseHandlers, gameserverService, taskService)
	backupHandlers := handlers.NewBackupHandlers(baseHandlers, gameserverService, backupService, fileService)

	log.Info().Msg("Handlers initialized")

	// Chi HTTP Server
	r := chi.NewRouter()

	// Chi middleware
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Recoverer)
	r.Use(middleware.StripSlashes)
	r.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			ww := middleware.NewWrapResponseWriter(w, r.ProtoMajor)
			next.ServeHTTP(ww, r)

			log.Info().
				Str("method", r.Method).
				Str("path", r.URL.Path).
				Int("status", ww.Status()).
				Dur("duration", time.Since(start)).
				Str("remote_addr", r.RemoteAddr).
				Str("user_agent", r.UserAgent()).
				Msg("HTTP request")
		})
	})

	// Static
	r.Handle("/static/*", http.StripPrefix("/static", http.FileServer(http.FS(staticFS))))

	// Games management routes
	r.Get("/games", gameHandlers.IndexGames)
	r.Get("/games/new", gameHandlers.NewGame)
	r.Post("/games", gameHandlers.CreateGame)
	r.Get("/games/{id}", gameHandlers.ShowGame)
	r.Get("/games/{id}/edit", gameHandlers.EditGame)
	r.Put("/games/{id}", gameHandlers.UpdateGame)
	r.Delete("/games/{id}", gameHandlers.DestroyGame)

	// Dashboard route
	r.Get("/", dashboardHandlers.IndexDashboard)

	// Gameservers routes
	r.Get("/gameservers", gameserverHandlers.IndexGameservers)
	r.Post("/gameservers", gameserverHandlers.CreateGameserver)
	r.Get("/gameservers/new", gameserverHandlers.NewGameserver)
	r.Get("/gameservers/{id}", gameserverHandlers.ShowGameserver)
	r.Get("/gameservers/{id}/edit", gameserverHandlers.EditGameserver)
	r.Put("/gameservers/{id}", gameserverHandlers.UpdateGameserver)
	r.Post("/gameservers/{id}/start", gameserverHandlers.StartGameserver)
	r.Post("/gameservers/{id}/stop", gameserverHandlers.StopGameserver)
	r.Post("/gameservers/{id}/restart", gameserverHandlers.RestartGameserver)
	r.Post("/gameservers/{id}/console", consoleHandlers.SendGameserverCommand)
	r.Delete("/gameservers/{id}", gameserverHandlers.DestroyGameserver)
	r.Get("/gameservers/{id}/console", consoleHandlers.GameserverConsole)
	r.Get("/gameservers/{id}/logs", consoleHandlers.GameserverLogs)
	r.Get("/gameservers/{id}/stats", gameserverHandlers.GameserverStats)
	r.Get("/gameservers/{id}/query", gameserverHandlers.GetGameserverQuery)
	r.Get("/gameservers/{id}/status", gameserverHandlers.GetGameserverStatus)
	r.Get("/gameservers/{id}/tasks", taskHandlers.ListGameserverTasks)
	r.Get("/gameservers/{id}/tasks/new", taskHandlers.NewGameserverTask)
	r.Post("/gameservers/{id}/tasks", taskHandlers.CreateGameserverTask)
	r.Get("/gameservers/{id}/tasks/{taskId}/edit", taskHandlers.EditGameserverTask)
	r.Put("/gameservers/{id}/tasks/{taskId}", taskHandlers.UpdateGameserverTask)
	r.Delete("/gameservers/{id}/tasks/{taskId}", taskHandlers.DeleteGameserverTask)
	r.Post("/gameservers/{id}/restore", backupHandlers.RestoreGameserverBackup)
	r.Post("/gameservers/{id}/backups", backupHandlers.CreateGameserverBackup)
	r.Get("/gameservers/{id}/backups", backupHandlers.ListGameserverBackups)
	r.Delete("/gameservers/{id}/backups/delete", backupHandlers.DeleteGameserverBackup)

	// File manager routes
	r.Get("/gameservers/{id}/files", fileHandlers.GameserverFiles)
	r.Get("/gameservers/{id}/files/browse", fileHandlers.BrowseGameserverFiles)
	r.Get("/gameservers/{id}/files/content", fileHandlers.GameserverFileContent)
	r.Post("/gameservers/{id}/files/save", fileHandlers.SaveGameserverFile)
	r.Get("/gameservers/{id}/files/download", fileHandlers.DownloadGameserverFile)
	r.Post("/gameservers/{id}/files/create", fileHandlers.CreateGameserverFile)
	r.Delete("/gameservers/{id}/files/delete", fileHandlers.DeleteGameserverFile)
	r.Post("/gameservers/{id}/files/rename", fileHandlers.RenameGameserverFile)
	r.Post("/gameservers/{id}/files/upload", fileHandlers.UploadGameserverFile)

	// Setup HTTP server with graceful shutdown
	srv := &http.Server{
		Addr:    fmt.Sprintf("%s:%d", config.Host, config.Port),
		Handler: r,
	}

	// Start server in a goroutine
	go func() {
		log.Info().Str("addr", srv.Addr).Msg("Starting HTTP server")
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatal().Err(err).Msg("Failed to start HTTP server")
		}
	}()

	// Wait for interrupt signal to gracefully shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	log.Info().Msg("Shutting down server...")

	// Create context with timeout for shutdown
	ctx, cancel := context.WithTimeout(context.Background(), config.ShutdownTimeout)
	defer cancel()

	// Shutdown server
	if err := srv.Shutdown(ctx); err != nil {
		log.Fatal().Err(err).Msg("Server forced to shutdown")
	}
	log.Info().Msg("Server exited")
}

// loadConfig loads configuration from environment variables with sensible defaults
func loadConfig() Config {
	config := Config{
		// Server defaults
		Host:            getenv("GAMESERVER_HOST", "localhost"),
		Port:            getenvInt("GAMESERVER_PORT", 3000),
		ShutdownTimeout: getenvDuration("GAMESERVER_SHUTDOWN_TIMEOUT", 30*time.Second),

		// Database defaults
		DatabasePath: getenv("GAMESERVER_DATABASE_PATH", "gameservers.db"),

		// Docker defaults
		DockerSocket:         getenv("GAMESERVER_DOCKER_SOCKET", ""),
		ContainerNamespace:   getenv("GAMESERVER_CONTAINER_NAMESPACE", "gameservers"),
		ContainerStopTimeout: getenvDuration("GAMESERVER_CONTAINER_STOP_TIMEOUT", 30*time.Second),

		// File system defaults (10MB edit, 100MB upload)
		MaxFileEditSize: getenvInt64("GAMESERVER_MAX_FILE_EDIT_SIZE", 10*1024*1024),
		MaxUploadSize:   getenvInt64("GAMESERVER_MAX_UPLOAD_SIZE", 100*1024*1024),
	}

	return config
}

// getenv gets environment variable with default value
func getenv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

// getenvInt gets environment variable as int with default value
func getenvInt(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		if i, err := strconv.Atoi(value); err == nil {
			return i
		}
		log.Warn().Str("key", key).Str("value", value).Msg("Invalid integer value, using default")
	}
	return defaultValue
}

// getenvInt64 gets environment variable as int64 with default value
func getenvInt64(key string, defaultValue int64) int64 {
	if value := os.Getenv(key); value != "" {
		if i, err := strconv.ParseInt(value, 10, 64); err == nil {
			return i
		}
		log.Warn().Str("key", key).Str("value", value).Msg("Invalid int64 value, using default")
	}
	return defaultValue
}

// getenvDuration gets environment variable as duration with default value
func getenvDuration(key string, defaultValue time.Duration) time.Duration {
	if value := os.Getenv(key); value != "" {
		if d, err := time.ParseDuration(value); err == nil {
			return d
		}
		log.Warn().Str("key", key).Str("value", value).Msg("Invalid duration value, using default")
	}
	return defaultValue
}

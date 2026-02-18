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
	"reflect"
	"strconv"
	"strings"
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
	Host          string
	Port          int
	PublicAddress string // Public IP/domain for gameserver connection details
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

	// Initialize gameserver repository
	gameserverRepo := database.NewGameserverRepository(db, dockerManager)
	log.Info().Msg("Gameserver repository initialized")

	// Initialize and start task scheduler
	taskScheduler := services.NewTaskScheduler(db, gameserverRepo)
	taskScheduler.Start()
	log.Info().Msg("Task scheduler started")

	// Ensure scheduler is stopped when application exits
	defer taskScheduler.Stop()

	// Parse html templates with custom functions
	tmpl, err := template.New("").Funcs(template.FuncMap{
		"formatFileSize": formatFileSize,
		"cronToHuman":    cronToHuman,
		"publicAddress":  func() string { return config.PublicAddress },
		"sub":            func(a, b int) int { return a - b },
		"mul": func(a, b interface{}) float64 {
			aVal, bVal := toFloat64(a), toFloat64(b)
			return aVal * bVal
		},
		"div": func(a, b interface{}) float64 {
			aVal, bVal := toFloat64(a), toFloat64(b)
			if bVal == 0 {
				return 0
			}
			return aVal / bVal
		},
		"gt": func(a, b interface{}) bool {
			return toFloat64(a) > toFloat64(b)
		},
		"floor": func(val interface{}) int {
			return int(toFloat64(val))
		},
		"printf": fmt.Sprintf,
		"len": func(v interface{}) int {
			if v == nil {
				return 0
			}
			val := reflect.ValueOf(v)
			switch val.Kind() {
			case reflect.Slice, reflect.Array, reflect.Map:
				return val.Len()
			default:
				return 0
			}
		},
	}).ParseFS(templateFiles, "templates/*.html")
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to parse templates")
	}
	log.Info().Msg("Templates parsed successfully")

	// Setup static fs
	staticFS, err := fs.Sub(staticFiles, "static")
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to setup static files")
	}

	// Set up error handling functions for handlers
	handlers.HandleError = HandleError
	handlers.NotFound = NotFound
	handlers.BadRequest = BadRequest
	handlers.InternalError = InternalError
	handlers.ParseForm = ParseForm
	handlers.RequireMethod = RequireMethod

	// Initialize query service
	queryService := services.NewQueryService()
	log.Info().Msg("Query service initialized")

	// Initialize handlers
	handlerInstance := handlers.New(gameserverRepo, dockerManager, tmpl, config.MaxFileEditSize, config.MaxUploadSize, queryService)

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

	// Routes
	r.Get("/", handlerInstance.IndexGameservers)

	// Gameserver routes
	r.Route("/gameservers", func(r chi.Router) {
		r.Get("/", handlerInstance.ListGameservers)
		r.Post("/", handlerInstance.CreateGameserver)
		r.Get("/new", handlerInstance.NewGameserver)
		r.Get("/{id}", handlerInstance.ShowGameserver)
		r.Get("/{id}/edit", handlerInstance.EditGameserver)
		r.Put("/{id}", handlerInstance.UpdateGameserver)
		r.Post("/{id}/start", handlerInstance.StartGameserver)
		r.Post("/{id}/stop", handlerInstance.StopGameserver)
		r.Post("/{id}/restart", handlerInstance.RestartGameserver)
		r.Post("/{id}/console", handlerInstance.SendGameserverCommand)
		r.Delete("/{id}", handlerInstance.DestroyGameserver)
		r.Get("/{id}/console", handlerInstance.GameserverConsole)
		r.Get("/{id}/logs", handlerInstance.GameserverLogs)
		r.Get("/{id}/stats", handlerInstance.GameserverStats)
		r.Get("/{id}/query", handlerInstance.QueryGameserver)
		r.Get("/{id}/query-partial", handlerInstance.QueryGameserverPartial)
		r.Get("/{id}/tasks", handlerInstance.ListGameserverTasks)
		r.Get("/{id}/tasks/new", handlerInstance.NewGameserverTask)
		r.Post("/{id}/tasks", handlerInstance.CreateGameserverTask)
		r.Get("/{id}/tasks/{taskId}/edit", handlerInstance.EditGameserverTask)
		r.Put("/{id}/tasks/{taskId}", handlerInstance.UpdateGameserverTask)
		r.Delete("/{id}/tasks/{taskId}", handlerInstance.DeleteGameserverTask)
		r.Post("/{id}/restore", handlerInstance.RestoreGameserverBackup)
		r.Post("/{id}/backup", handlerInstance.CreateGameserverBackup)
		r.Get("/{id}/backups", handlerInstance.ListGameserverBackups)
		r.Delete("/{id}/backups/delete", handlerInstance.DeleteGameserverBackup)

		// File manager routes
		r.Get("/{id}/files", handlerInstance.GameserverFiles)
		r.Get("/{id}/files/browse", handlerInstance.BrowseGameserverFiles)
		r.Get("/{id}/files/content", handlerInstance.GameserverFileContent)
		r.Post("/{id}/files/save", handlerInstance.SaveGameserverFile)
		r.Get("/{id}/files/download", handlerInstance.DownloadGameserverFile)
		r.Post("/{id}/files/create", handlerInstance.CreateGameserverFile)
		r.Delete("/{id}/files/delete", handlerInstance.DeleteGameserverFile)
		r.Post("/{id}/files/rename", handlerInstance.RenameGameserverFile)
		r.Post("/{id}/files/upload", handlerInstance.UploadGameserverFile)
	})

	// Game routes
	r.Route("/games", func(r chi.Router) {
		r.Get("/", handlerInstance.ListGames)
		r.Post("/", handlerInstance.CreateGame)
		r.Get("/new", handlerInstance.NewGame)
		r.Get("/{id}", handlerInstance.ShowGame)
		r.Get("/{id}/edit", handlerInstance.EditGame)
		r.Put("/{id}", handlerInstance.UpdateGame)
		r.Delete("/{id}", handlerInstance.DeleteGame)
	})

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


func formatFileSize(size int64) string {
	const unit = 1024
	if size < unit {
		return fmt.Sprintf("%d B", size)
	}
	div, exp := int64(unit), 0
	for n := size / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(size)/float64(div), "KMGTPE"[exp])
}

// toFloat64 converts interface{} to float64 for template math functions
func toFloat64(v interface{}) float64 {
	switch val := v.(type) {
	case int:
		return float64(val)
	case int64:
		return float64(val)
	case float64:
		return val
	case float32:
		return float64(val)
	default:
		return 0
	}
}

// cronToHuman converts a cron schedule to human-readable description
func cronToHuman(cron string) string {
	parts := strings.Fields(cron)
	if len(parts) != 5 {
		return cron
	}

	minute, hour, day, month, weekday := parts[0], parts[1], parts[2], parts[3], parts[4]
	weekdays := []string{"Sunday", "Monday", "Tuesday", "Wednesday", "Thursday", "Friday", "Saturday"}

	// Helper to format time in 12-hour format
	formatTime := func(h, m int) string {
		ampm := "AM"
		if h >= 12 {
			ampm = "PM"
		}
		displayHour := h
		if h == 0 {
			displayHour = 12
		} else if h > 12 {
			displayHour = h - 12
		}
		return fmt.Sprintf("%d:%02d %s", displayHour, m, ampm)
	}

	// Check for interval patterns in hour (e.g., */6)
	if strings.HasPrefix(hour, "*/") {
		interval, _ := strconv.Atoi(strings.TrimPrefix(hour, "*/"))
		if interval == 1 {
			return "Every hour"
		}
		return fmt.Sprintf("Every %d hours", interval)
	}

	// Check for interval patterns in minute (e.g., */30)
	if strings.HasPrefix(minute, "*/") {
		interval, _ := strconv.Atoi(strings.TrimPrefix(minute, "*/"))
		if interval == 1 {
			return "Every minute"
		}
		return fmt.Sprintf("Every %d minutes", interval)
	}

	// Parse hour and minute for specific time patterns
	h, hErr := strconv.Atoi(hour)
	m, mErr := strconv.Atoi(minute)
	if hErr != nil || mErr != nil {
		return cron
	}

	// Weekly pattern (specific weekday)
	if weekday != "*" {
		wd, err := strconv.Atoi(weekday)
		if err == nil && wd >= 0 && wd < 7 {
			return fmt.Sprintf("Weekly on %s at %s", weekdays[wd], formatTime(h, m))
		}
	}

	// Daily pattern (day, month, weekday all wildcards)
	if day == "*" && month == "*" && weekday == "*" {
		return fmt.Sprintf("Daily at %s", formatTime(h, m))
	}

	return cron
}

// loadConfig loads configuration from environment variables with sensible defaults
func loadConfig() Config {
	// Helper to get string env var
	getStr := func(key, def string) string {
		if v := os.Getenv(key); v != "" {
			return v
		}
		return def
	}

	// Helper to get int env var
	getInt := func(key string, def int) int {
		if v := os.Getenv(key); v != "" {
			if i, err := strconv.Atoi(v); err == nil {
				return i
			}
			log.Warn().Str("key", key).Str("value", v).Msg("Invalid integer, using default")
		}
		return def
	}

	// Helper to get int64 env var
	getInt64 := func(key string, def int64) int64 {
		if v := os.Getenv(key); v != "" {
			if i, err := strconv.ParseInt(v, 10, 64); err == nil {
				return i
			}
			log.Warn().Str("key", key).Str("value", v).Msg("Invalid int64, using default")
		}
		return def
	}

	// Helper to get duration env var
	getDuration := func(key string, def time.Duration) time.Duration {
		if v := os.Getenv(key); v != "" {
			if d, err := time.ParseDuration(v); err == nil {
				return d
			}
			log.Warn().Str("key", key).Str("value", v).Msg("Invalid duration, using default")
		}
		return def
	}

	return Config{
		// Server defaults
		Host:            getStr("GAMESERVER_HOST", "localhost"),
		Port:            getInt("GAMESERVER_PORT", 3000),
		PublicAddress:   getStr("GAMESERVER_PUBLIC_ADDRESS", "localhost"),
		ShutdownTimeout: getDuration("GAMESERVER_SHUTDOWN_TIMEOUT", 30*time.Second),

		// Database defaults
		DatabasePath: getStr("GAMESERVER_DATABASE_PATH", "gameservers.db"),

		// Docker defaults
		DockerSocket:         getStr("GAMESERVER_DOCKER_SOCKET", ""),
		ContainerNamespace:   getStr("GAMESERVER_CONTAINER_NAMESPACE", "gameservers"),
		ContainerStopTimeout: getDuration("GAMESERVER_CONTAINER_STOP_TIMEOUT", 30*time.Second),

		// File system defaults (10MB edit, 100MB upload)
		MaxFileEditSize: getInt64("GAMESERVER_MAX_FILE_EDIT_SIZE", 10*1024*1024),
		MaxUploadSize:   getInt64("GAMESERVER_MAX_UPLOAD_SIZE", 100*1024*1024),
	}
}

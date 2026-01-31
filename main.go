package main

import (
	"bytes"
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

	// Initialize database-level Gameserver service (implements models.GameserverServiceInterface)
	dbGameserverService := database.NewGameserverService(db, dockerManager)
	log.Info().Msg("Database gameserver service initialized")

	// Initialize business logic service from services package (using dbGameserverService which implements the interface)
	businessService := services.NewGameserverService(dbGameserverService, dockerManager, "/data")
	log.Info().Msg("Business logic service initialized")

	// Initialize and start task scheduler
	taskScheduler := services.NewTaskScheduler(db, businessService)
	taskScheduler.Start()
	log.Info().Msg("Task scheduler started")

	// Ensure scheduler is stopped when application exits
	defer taskScheduler.Stop()

	// Parse html templates with custom functions
	tmpl, err := template.New("").Funcs(template.FuncMap{
		"formatFileSize": formatFileSize,
		"sub":            func(a, b int) int { return a - b },
		"mul": func(a, b interface{}) float64 {
			var aVal, bVal float64

			switch v := a.(type) {
			case int:
				aVal = float64(v)
			case float64:
				aVal = v
			default:
				return 0
			}

			switch v := b.(type) {
			case int:
				bVal = float64(v)
			case float64:
				bVal = v
			default:
				return 0
			}

			return aVal * bVal
		},
		"div": func(a, b interface{}) float64 {
			var aVal, bVal float64

			switch v := a.(type) {
			case int:
				aVal = float64(v)
			case float64:
				aVal = v
			default:
				return 0
			}

			switch v := b.(type) {
			case int:
				bVal = float64(v)
			case float64:
				bVal = v
			default:
				return 0
			}

			if bVal == 0 {
				return 0
			}
			return aVal / bVal
		},
		"gt": func(a, b interface{}) bool {
			switch av := a.(type) {
			case int:
				if bv, ok := b.(int); ok {
					return av > bv
				}
				if bv, ok := b.(float64); ok {
					return float64(av) > bv
				}
			case float64:
				if bv, ok := b.(float64); ok {
					return av > bv
				}
				if bv, ok := b.(int); ok {
					return av > float64(bv)
				}
			}
			return false
		},
		"dict": func(values ...interface{}) map[string]interface{} {
			if len(values)%2 != 0 {
				return nil
			}
			dict := make(map[string]interface{}, len(values)/2)
			for i := 0; i < len(values); i += 2 {
				key, ok := values[i].(string)
				if !ok {
					return nil
				}
				dict[key] = values[i+1]
			}
			return dict
		},
		"slice": func(values ...interface{}) []interface{} {
			return values
		},
		"len": func(items interface{}) int {
			if items == nil {
				return 0
			}
			v := reflect.ValueOf(items)
			switch v.Kind() {
			case reflect.Slice, reflect.Array:
				return v.Len()
			default:
				return 0
			}
		},
		"printf": fmt.Sprintf,
		"floor": func(val interface{}) int {
			switch v := val.(type) {
			case float64:
				return int(v)
			case int:
				return v
			default:
				return 0
			}
		},
		"if": func(condition bool, trueVal, falseVal interface{}) interface{} {
			if condition {
				return trueVal
			}
			return falseVal
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
	handlers.Render = Render

	// Initialize query service
	queryService := services.NewQueryService()
	log.Info().Msg("Query service initialized")

	// Initialize handlers (using database service which implements models.GameserverServiceInterface)
	handlerInstance := handlers.New(dbGameserverService, tmpl, config.MaxFileEditSize, config.MaxUploadSize, queryService)

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

type LayoutData struct {
	Content          template.HTML
	Title            string
	ShowCreateButton bool
}

func Render(w http.ResponseWriter, r *http.Request, tmpl *template.Template, templateName string, data interface{}) {
	// If request is made using HTMX
	if r.Header.Get("HX-Request") == "true" {
		err := tmpl.ExecuteTemplate(w, templateName, data)
		if err != nil {
			log.Error().Err(err).Str("template", templateName).Msg("Failed to render HTMX template")
			http.Error(w, "Template error", http.StatusInternalServerError)
		}
	} else {
		var buf bytes.Buffer
		err := tmpl.ExecuteTemplate(&buf, templateName, data)
		if err != nil {
			log.Error().Err(err).Str("template", templateName).Msg("Failed to render template content")
			http.Error(w, "Template error", http.StatusInternalServerError)
			return
		}

		// Generate layout data based on the current page
		layoutData := generateLayoutData(r, template.HTML(buf.String()))

		err = tmpl.ExecuteTemplate(w, "layout.html", layoutData)
		if err != nil {
			log.Error().Err(err).Str("template", "layout.html").Msg("Failed to render layout template")
			http.Error(w, "Template error", http.StatusInternalServerError)
		}
	}
}

func generateLayoutData(r *http.Request, content template.HTML) LayoutData {
	path := r.URL.Path

	layout := LayoutData{
		Content:          content,
		ShowCreateButton: false,
	}

	// Simple title generation
	switch {
	case path == "/":
		layout.Title = "Dashboard"
		layout.ShowCreateButton = true
	case path == "/new":
		layout.Title = "Create Server"
	default:
		layout.Title = "Gameserver Control Panel"
	}

	return layout
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

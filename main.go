package main

import (
	"bytes"
	"embed"
	"fmt"
	"html/template"
	"io/fs"
	"net/http"
	"os"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

//go:embed templates/*.html
var templateFiles embed.FS

//go:embed static/*
var staticFiles embed.FS

func main() {
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
	db, err := NewDatabaseManager("gameservers.db")
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to initialize database")
	}
	defer db.Close()
	log.Info().Msg("Database initialized successfully")

	// Initialize Docker manager
	docker, err := NewDockerManager()
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to initialize Docker manager")
	}
	log.Info().Msg("Docker manager initialized successfully")

	// Initialize Gameserver service
	gameServerService := NewGameserverService(db, docker)
	log.Info().Msg("Gameserver service initialized")

	// Initialize and start task scheduler
	taskScheduler := NewTaskScheduler(db, gameServerService)
	taskScheduler.Start()
	log.Info().Msg("Task scheduler started")

	// Ensure scheduler is stopped when application exits
	defer taskScheduler.Stop()

	// Parse html templates with custom functions
	tmpl, err := template.New("").Funcs(template.FuncMap{
		"formatFileSize": formatFileSize,
		"sub": func(a, b int) int { return a - b },
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
		"printf": fmt.Sprintf,
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

	// Initialize handlers
	handlers := NewHandlers(gameServerService, tmpl)

	// Chi HTTP Server
	r := chi.NewRouter()

	// Chi middleware
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
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
	r.Get("/", handlers.IndexGameservers)
	r.Post("/", handlers.CreateGameserver)
	r.Get("/new", handlers.NewGameserver)
	r.Get("/{id}", handlers.ShowGameserver)
	r.Get("/{id}/edit", handlers.EditGameserver)
	r.Put("/{id}", handlers.UpdateGameserver)
	r.Post("/{id}/start", handlers.StartGameserver)
	r.Post("/{id}/stop", handlers.StopGameserver)
	r.Post("/{id}/restart", handlers.RestartGameserver)
	r.Post("/{id}/console", handlers.SendGameserverCommand)
	r.Delete("/{id}", handlers.DestroyGameserver)
	r.Get("/{id}/console", handlers.GameserverConsole)
	r.Get("/{id}/logs", handlers.GameserverLogs)
	r.Get("/{id}/stats", handlers.GameserverStats)
	r.Get("/{id}/tasks", handlers.ListGameserverTasks)
	r.Get("/{id}/tasks/new", handlers.NewGameserverTask)
	r.Post("/{id}/tasks", handlers.CreateGameserverTask)
	r.Get("/{id}/tasks/{taskId}/edit", handlers.EditGameserverTask)
	r.Put("/{id}/tasks/{taskId}", handlers.UpdateGameserverTask)
	r.Delete("/{id}/tasks/{taskId}", handlers.DeleteGameserverTask)
	r.Post("/{id}/restore", handlers.RestoreGameserverBackup)
	r.Post("/{id}/backup", handlers.CreateGameserverBackup)
	r.Get("/{id}/backups", handlers.ListGameserverBackups)
	r.Delete("/{id}/backups/delete", handlers.DeleteGameserverBackup)
	
	// File manager routes
	r.Get("/{id}/files", handlers.GameserverFiles)
	r.Get("/{id}/files/browse", handlers.BrowseGameserverFiles)
	r.Get("/{id}/files/content", handlers.GameserverFileContent)
	r.Post("/{id}/files/save", handlers.SaveGameserverFile)
	r.Get("/{id}/files/download", handlers.DownloadGameserverFile)
	r.Post("/{id}/files/create", handlers.CreateGameserverFile)
	r.Delete("/{id}/files/delete", handlers.DeleteGameserverFile)
	r.Post("/{id}/files/rename", handlers.RenameGameserverFile)

	// Start Chi HTTP server
	log.Info().Str("port", "3000").Msg("Starting HTTP server")
	if err := http.ListenAndServe(":3000", r); err != nil {
		log.Fatal().Err(err).Msg("Failed to start HTTP server")
	}
}

type LayoutData struct {
	Content          template.HTML
	Title           string
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
		Content: content,
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

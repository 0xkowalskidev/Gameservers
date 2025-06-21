package main

import (
	"bytes"
	"embed"
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

	// Parse html templates
	tmpl, err := template.ParseFS(templateFiles, "templates/*.html")
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
	r.Delete("/{id}", handlers.DestroyGameserver)
	r.Get("/{id}/logs", handlers.GameserverLogs)
	r.Get("/{id}/stats", handlers.GameserverStats)

	// Start Chi HTTP server
	log.Info().Str("port", "3000").Msg("Starting HTTP server")
	if err := http.ListenAndServe(":3000", r); err != nil {
		log.Fatal().Err(err).Msg("Failed to start HTTP server")
	}
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
		err = tmpl.ExecuteTemplate(w, "layout.html", map[string]interface{}{
			"Content": template.HTML(buf.String()),
		})
		if err != nil {
			log.Error().Err(err).Str("template", "layout.html").Msg("Failed to render layout template")
			http.Error(w, "Template error", http.StatusInternalServerError)
		}
	}
}

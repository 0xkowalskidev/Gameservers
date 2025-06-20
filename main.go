package main

import (
	"bytes"
	"embed"
	"html/template"
	"io/fs"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

//go:embed templates/*.html
var templateFiles embed.FS

//go:embed static/*
var staticFiles embed.FS

func main() {
	// Parse html templates
	tmpl, err := template.ParseFS(templateFiles, "templates/*.html")
	if err != nil {
		panic(err)
	}

	// Setup static fs
	staticFS, err := fs.Sub(staticFiles, "static")
	if err != nil {
		panic(err)
	}

	// Chi HTTP Server
	r := chi.NewRouter()

	// Chi middleware
	r.Use(middleware.Logger)

	// Static
	r.Handle("/static/*", http.StripPrefix("/static", http.FileServer(http.FS(staticFS))))

	// Chi Routes
	r.Get("/", func(w http.ResponseWriter, r *http.Request) {
		Render(w, r, tmpl, "index.html", nil)
	})

	// Start Chi HTTP server
	http.ListenAndServe(":3000", r)
}

func Render(w http.ResponseWriter, r *http.Request, tmpl *template.Template, templateName string, data interface{}) {
	// If request is made using HTMX
	if r.Header.Get("HX-Request") == "true" {
		// Render partial
		err := tmpl.ExecuteTemplate(w, templateName, data)
		if err != nil {
			// TODO: handle errors
		}
	} else {
		// Render full page
		var buf bytes.Buffer
		err := tmpl.ExecuteTemplate(&buf, templateName, data)
		if err != nil {
			// TODO: handle errors
			return
		}
		err = tmpl.ExecuteTemplate(w, "layout.html", map[string]interface{}{
			"Content": template.HTML(buf.String()),
		})
		if err != nil {
			// TODO: handle errors
		}
	}
}


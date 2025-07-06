package handlers

import (
	"bytes"
	"encoding/json"
	"fmt"
	"html/template"
	"net/http"

	. "0xkowalskidev/gameservers/errors" // Import AppError and error constructors
	"github.com/rs/zerolog/log"
)

// BaseHandlers contains common dependencies for all handlers
type BaseHandlers struct {
	tmpl *template.Template
}

// NewBaseHandlers creates base handlers with common dependencies
func NewBaseHandlers(tmpl *template.Template) *BaseHandlers {
	return &BaseHandlers{
		tmpl: tmpl,
	}
}

// Render handles HTMX partial vs full page rendering
func (h *BaseHandlers) Render(w http.ResponseWriter, r *http.Request, templateName string, data interface{}) {
	// If request is made using HTMX, render just the partial
	if r.Header.Get("HX-Request") == "true" {
		err := h.tmpl.ExecuteTemplate(w, templateName, data)
		if err != nil {
			log.Error().Err(err).Str("template", templateName).Msg("Failed to render HTMX template")
			http.Error(w, "Template error", http.StatusInternalServerError)
		}
	} else {
		// Full page request - render partial with layout
		var buf bytes.Buffer
		err := h.tmpl.ExecuteTemplate(&buf, templateName, data)
		if err != nil {
			log.Error().Err(err).Str("template", templateName).Msg("Failed to render template content")
			http.Error(w, "Template error", http.StatusInternalServerError)
			return
		}

		// Render with layout
		layoutData := map[string]interface{}{
			"Content": template.HTML(buf.String()),
		}

		err = h.tmpl.ExecuteTemplate(w, "layout.html", layoutData)
		if err != nil {
			log.Error().Err(err).Str("template", "layout.html").Msg("Failed to render layout template")
			http.Error(w, "Template error", http.StatusInternalServerError)
		}
	}
}

// SendToast sends a toast notification via HTMX
func (h *BaseHandlers) SendToast(w http.ResponseWriter, toastType, message string) {
	w.Header().Set("HX-Trigger", fmt.Sprintf(`{"showToast": {"type": "%s", "message": "%s"}}`, toastType, message))
}

// HandleError handles errors with HTMX toast support
func (h *BaseHandlers) HandleError(w http.ResponseWriter, r *http.Request, err error) {
	if err == nil {
		return
	}

	// Extract error details
	status := http.StatusInternalServerError
	message := "An error occurred"

	if appErr, ok := err.(AppError); ok {
		status = appErr.Status
		message = appErr.Message
		log.Error().Err(appErr.Err).Str("op", appErr.Op).Int("status", status).Msg(message)
	} else {
		log.Error().Err(err).Int("status", status).Msg(message)
	}

	// For HTMX requests, send toast and status
	if r.Header.Get("HX-Request") == "true" {
		h.SendToast(w, "error", message)
		w.WriteHeader(status)
		return
	}

	// Standard error for non-HTMX
	http.Error(w, message, status)
}

// HandleSuccess sends success notifications for HTMX
func (h *BaseHandlers) HandleSuccess(w http.ResponseWriter, r *http.Request, message string) {
	if r.Header.Get("HX-Request") == "true" {
		h.SendToast(w, "success", message)
	}
}

// jsonResponse sends a JSON response
func (h *BaseHandlers) jsonResponse(w http.ResponseWriter, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(data)
}

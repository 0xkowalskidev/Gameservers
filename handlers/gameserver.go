package handlers

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/rs/zerolog/log"

	"0xkowalskidev/gameservers/models"
)

// IndexGameservers lists all gameservers
func (h *Handlers) IndexGameservers(w http.ResponseWriter, r *http.Request) {
	gameservers, err := h.service.ListGameservers()
	if err != nil {
		HandleError(w, InternalError(err, "Failed to list gameservers"), "index_gameservers")
		return
	}
	Render(w, r, h.tmpl, "index.html", map[string]interface{}{"Gameservers": gameservers})
}

// ShowGameserver displays gameserver details
func (h *Handlers) ShowGameserver(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	gameserver, ok := h.getGameserver(w, id)
	if !ok {
		return
	}
	
	data := map[string]interface{}{"Gameserver": gameserver}
	// If HTMX request, render just the template content
	if r.Header.Get("HX-Request") == "true" {
		if err := h.tmpl.ExecuteTemplate(w, "gameserver-details.html", data); err != nil {
			HandleError(w, InternalError(err, "Failed to render gameserver details template"), "show_gameserver")
		}
	} else {
		// Full page load, use wrapper
		h.renderGameserverPage(w, r, gameserver, "overview", "gameserver-details.html", data)
	}
}

// NewGameserver shows the create gameserver form
func (h *Handlers) NewGameserver(w http.ResponseWriter, r *http.Request) {
	games, err := h.service.ListGames()
	if err != nil {
		HandleError(w, InternalError(err, "Failed to list games"), "new_gameserver")
		return
	}
	Render(w, r, h.tmpl, "new-gameserver.html", map[string]interface{}{"Games": games})
}

// EditGameserver shows the edit gameserver form
func (h *Handlers) EditGameserver(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	gameserver, ok := h.getGameserver(w, id)
	if !ok {
		return
	}
	
	games, err := h.service.ListGames()
	if err != nil {
		HandleError(w, InternalError(err, "Failed to list games"), "edit_gameserver")
		return
	}
	
	data := map[string]interface{}{
		"Gameserver": gameserver,
		"Games":      games,
	}
	
	// If HTMX request, render just the template content
	if r.Header.Get("HX-Request") == "true" {
		if err := h.tmpl.ExecuteTemplate(w, "edit-gameserver.html", data); err != nil {
			HandleError(w, InternalError(err, "Failed to render edit gameserver template"), "edit_gameserver")
		}
	} else {
		// Full page load, use wrapper
		h.renderGameserverPage(w, r, gameserver, "edit", "edit-gameserver.html", data)
	}
}

// CreateGameserver creates a new gameserver
func (h *Handlers) CreateGameserver(w http.ResponseWriter, r *http.Request) {
	formData, err := parseGameserverForm(r)
	if err != nil {
		HandleError(w, err, "create_gameserver_form")
		return
	}

	server := &models.Gameserver{
		ID:          generateID(),
		Name:        formData.Name,
		GameID:      formData.GameID,
		MemoryMB:    formData.MemoryMB,
		CPUCores:    formData.CPUCores,
		MaxBackups:  formData.MaxBackups,
		Environment: formData.Environment,
	}

	log.Info().Str("gameserver_id", server.ID).Str("name", server.Name).Int("memory_mb", formData.MemoryMB).Float64("cpu_cores", formData.CPUCores).Msg("Creating gameserver")

	if err := h.service.CreateGameserver(server); err != nil {
		HandleError(w, InternalError(err, "Failed to create gameserver"), "create_gameserver")
		return
	}

	h.htmxRedirect(w, "/")
}

// UpdateGameserver updates an existing gameserver
func (h *Handlers) UpdateGameserver(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	formData, err := parseGameserverForm(r)
	if err != nil {
		HandleError(w, err, "update_gameserver_form")
		return
	}

	// Get existing gameserver to preserve port mappings
	existingServer, err := h.service.GetGameserver(id)
	if err != nil {
		HandleError(w, InternalError(err, "Failed to get existing gameserver"), "update_gameserver")
		return
	}

	server := &models.Gameserver{
		ID:           id,
		Name:         formData.Name,
		GameID:       formData.GameID,
		MemoryMB:     formData.MemoryMB,
		CPUCores:     formData.CPUCores,
		MaxBackups:   formData.MaxBackups,
		Environment:  formData.Environment,
		PortMappings: existingServer.PortMappings, // Preserve existing port allocations
	}

	log.Info().Str("gameserver_id", server.ID).Str("name", server.Name).Int("memory_mb", formData.MemoryMB).Float64("cpu_cores", formData.CPUCores).Msg("Updating gameserver")

	if err := h.service.UpdateGameserver(server); err != nil {
		HandleError(w, InternalError(err, "Failed to update gameserver"), "update_gameserver")
		return
	}

	h.htmxRedirect(w, "/"+id)
}

// StartGameserver starts a gameserver
func (h *Handlers) StartGameserver(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	log.Info().Str("gameserver_id", id).Msg("Starting gameserver")

	if err := h.service.StartGameserver(id); err != nil {
		HandleError(w, InternalError(err, "Failed to start gameserver"), "start_gameserver")
		return
	}

	h.GameserverRow(w, r)
}

// StopGameserver stops a gameserver
func (h *Handlers) StopGameserver(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if err := h.service.StopGameserver(id); err != nil {
		HandleError(w, InternalError(err, "Failed to stop gameserver"), "stop_gameserver")
		return
	}
	h.GameserverRow(w, r)
}

// RestartGameserver restarts a gameserver
func (h *Handlers) RestartGameserver(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if err := h.service.RestartGameserver(id); err != nil {
		HandleError(w, InternalError(err, "Failed to restart gameserver"), "restart_gameserver")
		return
	}
	h.GameserverRow(w, r)
}

// DestroyGameserver deletes a gameserver
func (h *Handlers) DestroyGameserver(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if err := h.service.DeleteGameserver(id); err != nil {
		HandleError(w, InternalError(err, "Failed to delete gameserver"), "destroy_gameserver")
		return
	}
	w.WriteHeader(http.StatusOK)
}

// GameserverRow renders a single gameserver row (for HTMX updates)
func (h *Handlers) GameserverRow(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	gameserver, ok := h.getGameserver(w, id)
	if !ok {
		return
	}

	if err := h.tmpl.ExecuteTemplate(w, "gameserver-row.html", gameserver); err != nil {
		HandleError(w, InternalError(err, "Failed to render gameserver row"), "gameserver_row")
	}
}
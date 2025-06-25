package handlers

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/rs/zerolog/log"

	"0xkowalskidev/gameservers/models"
)

// DashboardData represents the data for the dashboard page
type DashboardData struct {
	Gameservers        []*models.Gameserver
	SystemInfo         *models.SystemInfo
	CurrentMemoryUsage int
	RunningServers     int
}

// IndexGameservers lists all gameservers with resource usage statistics
func (h *Handlers) IndexGameservers(w http.ResponseWriter, r *http.Request) {
	gameservers, err := h.service.ListGameservers()
	if err != nil {
		HandleError(w, InternalError(err, "Failed to list gameservers"), "index_gameservers")
		return
	}

	// Get system information and calculate current usage from running servers only
	systemInfo, err := models.GetSystemInfo()
	if err != nil {
		log.Warn().Err(err).Msg("Failed to get system information")
	}

	var currentMemoryUsage int
	var runningServers int
	for _, server := range gameservers {
		// Only count memory from running/starting servers
		if server.Status == models.StatusRunning || server.Status == models.StatusStarting {
			currentMemoryUsage += server.MemoryMB
			runningServers++
		}
	}

	data := DashboardData{
		Gameservers:        gameservers,
		SystemInfo:         systemInfo,
		CurrentMemoryUsage: currentMemoryUsage,
		RunningServers:     runningServers,
	}

	Render(w, r, h.tmpl, "index.html", data)
}

// ShowGameserver displays gameserver details
func (h *Handlers) ShowGameserver(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	gameserver, ok := h.getGameserver(w, id)
	if !ok {
		return
	}

	h.renderGameserverPageOrPartial(w, r, gameserver, "overview", "gameserver-details.html", nil)
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
	formData, err := h.parseGameserverForm(r)
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
	formData, err := h.parseGameserverForm(r)
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

// QueryGameserver returns JSON query data for client-side polling
func (h *Handlers) QueryGameserver(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	gameserver, ok := h.getGameserver(w, id)
	if !ok {
		h.jsonError(w, "Gameserver not found")
		return
	}

	// Only query running servers
	if gameserver.Status != models.StatusRunning {
		h.jsonSuccess(w, map[string]interface{}{
			"online": false,
			"status": gameserver.Status,
		})
		return
	}

	if h.queryService == nil {
		h.jsonError(w, "Query service not available")
		return
	}

	// Get game info for query
	game, err := h.service.GetGame(gameserver.GameID)
	if err != nil {
		log.Error().Err(err).Str("game_id", gameserver.GameID).Msg("Failed to get game info")
		h.jsonError(w, "Failed to get game info")
		return
	}

	serverInfo, err := h.queryService.QueryGameserver(gameserver, game)
	if err != nil {
		log.Debug().Err(err).Str("gameserver_id", id).Msg("Failed to query gameserver")
		h.jsonSuccess(w, map[string]interface{}{
			"online": false,
			"error":  err.Error(),
		})
		return
	}

	// Return the server info as JSON
	h.jsonSuccess(w, map[string]interface{}{
		"online":  serverInfo.Online,
		"players": serverInfo.Players,
		"map":     serverInfo.Map,
		"ping":    serverInfo.Ping,
	})
}

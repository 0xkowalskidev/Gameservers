package handlers

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	. "0xkowalskidev/gameservers/errors"
	"github.com/docker/docker/api/types/container"
	"github.com/go-chi/chi/v5"
	"github.com/rs/zerolog/log"

	"0xkowalskidev/gameservers/models"
	"0xkowalskidev/gameservers/services"
)

// GameserverHandlers handles gameserver-related HTTP requests
type GameserverHandlers struct {
	*BaseHandlers
	gameserverService services.GameserverServiceInterface
	gameService       models.GameServiceInterface
}

// NewGameserverHandlers creates new gameserver handlers
func NewGameserverHandlers(base *BaseHandlers, gameserverService services.GameserverServiceInterface, gameService models.GameServiceInterface) *GameserverHandlers {
	return &GameserverHandlers{
		BaseHandlers:      base,
		gameserverService: gameserverService,
		gameService:       gameService,
	}
}

// IndexGameservers lists all gameservers
func (h *GameserverHandlers) IndexGameservers(w http.ResponseWriter, r *http.Request) {
	gameservers, err := h.gameserverService.ListGameservers()
	if err != nil {
		h.HandleError(w, r, err)
		return
	}

	data := map[string]interface{}{
		"Gameservers": gameservers,
	}

	h.Render(w, r, "gameservers-list.html", data)
}

// ShowGameserver displays gameserver details
func (h *GameserverHandlers) ShowGameserver(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	gameserver, err := h.gameserverService.GetGameserver(id)
	if err != nil {
		h.HandleError(w, r, NotFound("Gameserver"))
		return
	}

	// Get the game info for display purposes
	game, err := h.gameService.GetGame(gameserver.GameID)
	if err != nil {
		h.HandleError(w, r, err)
		return
	}

	data := map[string]interface{}{
		"Gameserver": gameserver,
		"Game":       game,
	}
	h.Render(w, r, "gameserver-details.html", data)
}

// NewGameserver shows the create gameserver form
func (h *GameserverHandlers) NewGameserver(w http.ResponseWriter, r *http.Request) {
	games, err := h.gameService.ListGames()
	if err != nil {
		h.HandleError(w, r, err)
		return
	}

	// Get pre-selected game from query parameter
	selectedGameID := r.URL.Query().Get("game")

	data := map[string]interface{}{
		"Games":          games,
		"SelectedGameID": selectedGameID,
	}
	h.Render(w, r, "new-gameserver.html", data)
}

// EditGameserver shows the edit gameserver form
func (h *GameserverHandlers) EditGameserver(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	gameserver, err := h.gameserverService.GetGameserver(id)
	if err != nil {
		h.HandleError(w, r, NotFound("Gameserver"))
		return
	}

	// Get the specific game for this gameserver (game is not editable)
	game, err := h.gameService.GetGame(gameserver.GameID)
	if err != nil {
		h.HandleError(w, r, err)
		return
	}

	data := map[string]interface{}{
		"Gameserver": gameserver,
		"Game":       game,
	}
	h.Render(w, r, "edit-gameserver.html", data)
}

// CreateGameserver creates a new gameserver
func (h *GameserverHandlers) CreateGameserver(w http.ResponseWriter, r *http.Request) {
	if err := ParseForm(r); err != nil {
		h.HandleError(w, r, err)
		return
	}

	name := strings.TrimSpace(r.FormValue("name"))
	gameID := strings.TrimSpace(r.FormValue("game_id"))
	if name == "" || gameID == "" {
		h.HandleError(w, r, BadRequest("name and game_id are required"))
		return
	}

	memoryGB, _ := strconv.ParseFloat(r.FormValue("memory_gb"), 64)
	if memoryGB <= 0 {
		memoryGB = 1.0
	}
	memoryMB := int(memoryGB * 1024)

	cpuCores, _ := strconv.ParseFloat(r.FormValue("cpu_cores"), 64)
	maxBackups, _ := strconv.Atoi(r.FormValue("max_backups"))
	if maxBackups <= 0 {
		maxBackups = 7
	}

	var validEnv []string
	for _, line := range strings.Split(r.FormValue("environment"), "\n") {
		line = strings.TrimSpace(line)
		if line != "" && strings.Contains(line, "=") {
			validEnv = append(validEnv, line)
		}
	}

	req := services.CreateGameserverRequest{
		Name:     name,
		GameID:   gameID,
		MemoryMB: memoryMB,
		CPUCores: cpuCores,
	}

	log.Info().Str("name", req.Name).Str("game_id", req.GameID).Int("memory_mb", memoryMB).Float64("cpu_cores", cpuCores).Msg("Creating gameserver")

	server, err := h.gameserverService.CreateGameserver(req)
	if err != nil {
		h.HandleError(w, r, err)
		return
	}

	// Success - set server ID in header for HTMX to use
	h.HandleSuccess(w, r, "Gameserver created successfully")
	w.Header().Set("X-Server-ID", server.ID)
	w.WriteHeader(http.StatusOK)
}

// UpdateGameserver updates an existing gameserver
func (h *GameserverHandlers) UpdateGameserver(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if err := ParseForm(r); err != nil {
		h.HandleError(w, r, err)
		return
	}

	name := strings.TrimSpace(r.FormValue("name"))
	gameID := strings.TrimSpace(r.FormValue("game_id"))
	if name == "" || gameID == "" {
		h.HandleError(w, r, BadRequest("name and game_id are required"))
		return
	}

	memoryGB, _ := strconv.ParseFloat(r.FormValue("memory_gb"), 64)
	if memoryGB <= 0 {
		memoryGB = 1.0
	}
	memoryMB := int(memoryGB * 1024)

	// Get existing gameserver to preserve port mappings  
	_, err := h.gameserverService.GetGameserver(id)
	if err != nil {
		h.HandleError(w, r, err)
		return
	}

	req := services.UpdateGameserverRequest{
		Name:     name,
		MemoryMB: memoryMB,
	}

	log.Info().Str("gameserver_id", id).Str("name", req.Name).Int("memory_mb", memoryMB).Msg("Updating gameserver")

	if err := h.gameserverService.UpdateGameserver(id, req); err != nil {
		h.HandleError(w, r, err)
		return
	}

	// Success
	h.HandleSuccess(w, r, "Gameserver updated successfully")
	w.Header().Set("HX-Redirect", "/"+id)
	w.WriteHeader(http.StatusOK)
}

// StartGameserver starts a gameserver
func (h *GameserverHandlers) StartGameserver(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	log.Info().Str("gameserver_id", id).Msg("Starting gameserver")

	if err := h.gameserverService.StartGameserver(id); err != nil {
		h.HandleError(w, r, err)
		return
	}

	w.WriteHeader(http.StatusOK)
}

// StopGameserver stops a gameserver
func (h *GameserverHandlers) StopGameserver(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if err := h.gameserverService.StopGameserver(id); err != nil {
		h.HandleError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusOK)
}

// RestartGameserver restarts a gameserver
func (h *GameserverHandlers) RestartGameserver(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if err := h.gameserverService.RestartGameserver(id); err != nil {
		h.HandleError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusOK)
}

// DestroyGameserver deletes a gameserver
func (h *GameserverHandlers) DestroyGameserver(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if err := h.gameserverService.DeleteGameserver(id); err != nil {
		h.HandleError(w, r, err)
		return
	}
	// Success
	h.HandleSuccess(w, r, "Gameserver deleted successfully")
	w.WriteHeader(http.StatusOK)
}

// GetGameserverQuery returns protocol query data for client-side polling (players, map, etc.)
func (h *GameserverHandlers) GetGameserverQuery(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	info, err := h.gameserverService.GetGameserverQuery(r.Context(), id)
	if err != nil {
		data := map[string]interface{}{
			"online": false,
			"error":  err.Error(),
		}
		h.Render(w, r, "server-query.html", data)
		return
	}

	h.Render(w, r, "server-query.html", info)
}

// GetGameserverStatus checks if gameserver is running via Docker status
func (h *GameserverHandlers) GetGameserverStatus(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	
	status, err := h.gameserverService.GetGameserverStatus(id)
	if err != nil {
		h.HandleError(w, r, NotFound("Gameserver"))
		return
	}

	// Format status with title case for display
	var formattedStatus string
	switch status {
	case models.StatusRunning:
		formattedStatus = "Running"
	case models.StatusStopped:
		formattedStatus = "Stopped"
	case models.StatusStarting:
		formattedStatus = "Starting"
	case models.StatusStopping:
		formattedStatus = "Stopping"
	default:
		formattedStatus = string(status)
	}

	data := map[string]interface{}{
		"Status": formattedStatus,
	}

	h.Render(w, r, "server-status.html", data)
}

// GameserverStats streams real-time stats (CPU, memory) via SSE
func (h *GameserverHandlers) GameserverStats(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	flusher, ok := w.(http.Flusher)
	if !ok {
		h.HandleError(w, r, InternalError(nil, "Streaming unsupported"))
		return
	}

	stats, err := h.gameserverService.StreamGameserverStats(r.Context(), id)
	if err != nil {
		log.Error().Err(err).Str("gameserver_id", id).Msg("Failed to stream stats")
		fmt.Fprintf(w, "event: error\ndata: Failed to stream stats: %v\n\n", err)
		flusher.Flush()
		return
	}
	defer stats.Close()

	scanner := bufio.NewScanner(stats)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.TrimSpace(line) != "" {
			var v container.StatsResponse
			if err := json.Unmarshal([]byte(line), &v); err != nil {
				continue
			}

			// Calculate CPU percentage
			cpuDelta := float64(v.CPUStats.CPUUsage.TotalUsage - v.PreCPUStats.CPUUsage.TotalUsage)
			systemDelta := float64(v.CPUStats.SystemUsage - v.PreCPUStats.SystemUsage)
			cpuPercent := 0.0

			if systemDelta > 0.0 && cpuDelta > 0.0 {
				onlineCPUs := float64(len(v.CPUStats.CPUUsage.PercpuUsage))
				if onlineCPUs == 0 {
					onlineCPUs = float64(v.CPUStats.OnlineCPUs)
					if onlineCPUs == 0 {
						onlineCPUs = 1
					}
				}
				cpuPercent = (cpuDelta / systemDelta) * onlineCPUs * 100.0
			}

			// Memory stats
			memUsage := v.MemoryStats.Usage
			if cache, ok := v.MemoryStats.Stats["cache"]; ok {
				memUsage -= cache
			}
			memLimit := v.MemoryStats.Limit
			memPercent := 0.0
			if memLimit > 0 {
				memPercent = (float64(memUsage) / float64(memLimit)) * 100.0
			}

			// Format memory values
			memUsageMB := float64(memUsage) / 1024 / 1024
			memLimitMB := float64(memLimit) / 1024 / 1024

			// Create stats JSON
			statsJSON := map[string]interface{}{
				"cpu":           cpuPercent,
				"memoryUsageGB": memUsageMB / 1024, // Convert MB to GB
				"memoryLimitGB": memLimitMB / 1024, // Convert MB to GB
				"memoryPercent": memPercent,
			}

			statsData, _ := json.Marshal(statsJSON)
			fmt.Fprintf(w, "event: stats\ndata: %s\n\n", string(statsData))
			flusher.Flush()
		}
	}
}

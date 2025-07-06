package handlers

import (
	"net/http"
	"strconv"

	. "0xkowalskidev/gameservers/errors"
	"github.com/go-chi/chi/v5"
	"github.com/rs/zerolog/log"

	"0xkowalskidev/gameservers/models"
	"0xkowalskidev/gameservers/services"
)

// GameHandlers handles game-related HTTP requests
type GameHandlers struct {
	*BaseHandlers
	gameService       models.GameServiceInterface
	gameserverService services.GameserverServiceInterface // Needed for validation
}

// NewGameHandlers creates new game handlers
func NewGameHandlers(base *BaseHandlers, gameService models.GameServiceInterface, gameserverService services.GameserverServiceInterface) *GameHandlers {
	return &GameHandlers{
		BaseHandlers:      base,
		gameService:       gameService,
		gameserverService: gameserverService,
	}
}

// GameWithCounts represents a game with pre-calculated counts
type GameWithCounts struct {
	*models.Game
	PortMappingsCount int
	ConfigVarsCount   int
}

// IndexGames lists all games
func (h *GameHandlers) IndexGames(w http.ResponseWriter, r *http.Request) {
	games, err := h.gameService.ListGames()
	if err != nil {
		h.HandleError(w, r, err)
		return
	}

	// Enrich games with counts
	var gamesWithCounts []GameWithCounts
	for _, game := range games {
		gamesWithCounts = append(gamesWithCounts, GameWithCounts{
			Game:              game,
			PortMappingsCount: len(game.PortMappings),
			ConfigVarsCount:   len(game.ConfigVars),
		})
	}

	data := map[string]interface{}{
		"Games": gamesWithCounts,
	}

	h.Render(w, r, "games-list.html", data)
}

// ShowGame displays a specific game's details
func (h *GameHandlers) ShowGame(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	game, err := h.gameService.GetGame(id)
	if err != nil {
		h.HandleError(w, r, err)
		return
	}

	data := map[string]interface{}{
		"Game": game,
	}

	h.Render(w, r, "game-details.html", data)
}

// NewGame shows the create game form
func (h *GameHandlers) NewGame(w http.ResponseWriter, r *http.Request) {
	h.Render(w, r, "new-game.html", nil)
}

// EditGame shows the edit game form
func (h *GameHandlers) EditGame(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	game, err := h.gameService.GetGame(id)
	if err != nil {
		h.HandleError(w, r, err)
		return
	}

	data := map[string]interface{}{
		"Game": game,
	}

	h.Render(w, r, "edit-game.html", data)
}

// CreateGame handles game creation
func (h *GameHandlers) CreateGame(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		h.HandleError(w, r, err)
		return
	}

	// Parse memory values
	minMemoryMB, err := strconv.Atoi(r.FormValue("min_memory_mb"))
	if err != nil {
		h.HandleError(w, r, err)
		return
	}

	recMemoryMB, err := strconv.Atoi(r.FormValue("rec_memory_mb"))
	if err != nil {
		h.HandleError(w, r, err)
		return
	}

	// Parse port mappings
	portMappings, err := h.parsePortMappings(r)
	if err != nil {
		h.HandleError(w, r, err)
		return
	}

	// Parse config vars
	configVars, err := h.parseConfigVars(r)
	if err != nil {
		h.HandleError(w, r, err)
		return
	}

	game := &models.Game{
		ID:            r.FormValue("id"),
		Name:          r.FormValue("name"),
		Slug:          r.FormValue("slug"),
		Image:         r.FormValue("image"),
		IconPath:      r.FormValue("icon_path"),
		GridImagePath: r.FormValue("grid_image_path"),
		PortMappings:  portMappings,
		ConfigVars:    configVars,
		MinMemoryMB:   minMemoryMB,
		RecMemoryMB:   recMemoryMB,
	}

	if err := h.gameService.CreateGame(game); err != nil {
		h.HandleError(w, r, err)
		return
	}

	log.Info().Str("game_id", game.ID).Str("game_name", game.Name).Msg("Game created successfully")

	// Return success with game ID for redirect
	w.Header().Set("X-Game-ID", game.ID)
	if r.Header.Get("HX-Request") == "true" {
		w.WriteHeader(http.StatusCreated)
	} else {
		http.Redirect(w, r, "/games/"+game.ID, http.StatusSeeOther)
	}
}

// UpdateGame handles game updates
func (h *GameHandlers) UpdateGame(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	game, err := h.gameService.GetGame(id)
	if err != nil {
		h.HandleError(w, r, err)
		return
	}

	if err := r.ParseForm(); err != nil {
		h.HandleError(w, r, err)
		return
	}

	// Parse memory values
	minMemoryMB, err := strconv.Atoi(r.FormValue("min_memory_mb"))
	if err != nil {
		h.HandleError(w, r, err)
		return
	}

	recMemoryMB, err := strconv.Atoi(r.FormValue("rec_memory_mb"))
	if err != nil {
		h.HandleError(w, r, err)
		return
	}

	// Parse port mappings
	portMappings, err := h.parsePortMappings(r)
	if err != nil {
		h.HandleError(w, r, err)
		return
	}

	// Parse config vars
	configVars, err := h.parseConfigVars(r)
	if err != nil {
		h.HandleError(w, r, err)
		return
	}

	// Update game fields
	game.Name = r.FormValue("name")
	game.Slug = r.FormValue("slug")
	game.Image = r.FormValue("image")
	game.IconPath = r.FormValue("icon_path")
	game.GridImagePath = r.FormValue("grid_image_path")
	game.PortMappings = portMappings
	game.ConfigVars = configVars
	game.MinMemoryMB = minMemoryMB
	game.RecMemoryMB = recMemoryMB

	if err := h.gameService.UpdateGame(game); err != nil {
		h.HandleError(w, r, err)
		return
	}

	log.Info().Str("game_id", game.ID).Str("game_name", game.Name).Msg("Game updated successfully")

	if r.Header.Get("HX-Request") == "true" {
		w.WriteHeader(http.StatusOK)
	} else {
		http.Redirect(w, r, "/games/"+game.ID, http.StatusSeeOther)
	}
}

// DestroyGame handles game deletion
func (h *GameHandlers) DestroyGame(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	// Check if any gameservers are using this game
	gameservers, err := h.gameserverService.ListGameservers()
	if err != nil {
		h.HandleError(w, r, err)
		return
	}

	for _, server := range gameservers {
		if server.GameID == id {
			h.HandleError(w, r, BadRequest("Cannot delete game: it is being used by gameserver '%s'", server.Name))
			return
		}
	}

	if err := h.gameService.DeleteGame(id); err != nil {
		h.HandleError(w, r, err)
		return
	}

	log.Info().Str("game_id", id).Msg("Game deleted successfully")

	if r.Header.Get("HX-Request") == "true" {
		w.WriteHeader(http.StatusOK)
	} else {
		http.Redirect(w, r, "/games", http.StatusSeeOther)
	}
}

// parsePortMappings parses port mapping form data
func (h *GameHandlers) parsePortMappings(r *http.Request) ([]models.PortMapping, error) {
	var portMappings []models.PortMapping

	names := r.Form["port_name"]
	protocols := r.Form["port_protocol"]
	containerPorts := r.Form["port_container"]

	for i := 0; i < len(names); i++ {
		if names[i] == "" || protocols[i] == "" || containerPorts[i] == "" {
			continue
		}

		containerPort, err := strconv.Atoi(containerPorts[i])
		if err != nil {
			return nil, err
		}

		portMappings = append(portMappings, models.PortMapping{
			Name:          names[i],
			Protocol:      protocols[i],
			ContainerPort: containerPort,
			HostPort:      0, // Host port is allocated dynamically
		})
	}

	return portMappings, nil
}

// parseConfigVars parses configuration variable form data
func (h *GameHandlers) parseConfigVars(r *http.Request) ([]models.ConfigVar, error) {
	var configVars []models.ConfigVar

	names := r.Form["config_name"]
	displayNames := r.Form["config_display_name"]
	required := r.Form["config_required"]
	defaults := r.Form["config_default"]
	descriptions := r.Form["config_description"]

	for i := 0; i < len(names); i++ {
		if names[i] == "" || displayNames[i] == "" {
			continue
		}

		isRequired := false
		if i < len(required) && required[i] == "on" {
			isRequired = true
		}

		defaultValue := ""
		if i < len(defaults) {
			defaultValue = defaults[i]
		}

		description := ""
		if i < len(descriptions) {
			description = descriptions[i]
		}

		configVars = append(configVars, models.ConfigVar{
			Name:        names[i],
			DisplayName: displayNames[i],
			Required:    isRequired,
			Default:     defaultValue,
			Description: description,
		})
	}

	return configVars, nil
}


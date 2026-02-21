package handlers

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"

	"0xkowalskidev/gameservers/models"
)

// GamesListData represents the data for the games list page
type GamesListData struct {
	Games []*models.Game
}

// ListGames shows the games list page
func (h *Handlers) ListGames(w http.ResponseWriter, r *http.Request) {
	games, err := h.service.ListGames()
	if err != nil {
		HandleError(w, InternalError(err, "Failed to list games"), "list_games")
		return
	}

	data := GamesListData{
		Games: games,
	}

	h.render(w, r, "games.html", data)
}

// NewGame shows the create game form
func (h *Handlers) NewGame(w http.ResponseWriter, r *http.Request) {
	h.render(w, r, "game-form.html", map[string]interface{}{"Game": nil})
}

// ShowGame displays game details
func (h *Handlers) ShowGame(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	game, ok := h.getGame(w, id)
	if !ok {
		return
	}

	// Get count of gameservers using this game
	count, err := h.service.CountGameserversByGameID(id)
	if err != nil {
		HandleError(w, InternalError(err, "Failed to count gameservers"), "show_game")
		return
	}

	// Get mods for this game
	mods, err := h.service.GetModsForGame(id)
	if err != nil {
		mods = nil // Don't fail if mods can't be loaded
	}

	data := map[string]interface{}{
		"Game":            game,
		"GameserverCount": count,
		"Mods":            mods,
	}

	h.render(w, r, "game-details.html", data)
}

// EditGame shows the edit game form
func (h *Handlers) EditGame(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	game, ok := h.getGame(w, id)
	if !ok {
		return
	}

	// Get mods for this game
	mods, err := h.service.GetModsForGame(id)
	if err != nil {
		mods = nil // Don't fail if mods can't be loaded
	}

	h.render(w, r, "game-form.html", map[string]interface{}{
		"Game": game,
		"Mods": mods,
	})
}

// CreateGame creates a new game
func (h *Handlers) CreateGame(w http.ResponseWriter, r *http.Request) {
	game, err := h.parseGameForm(r)
	if err != nil {
		HandleError(w, err, "create_game_form")
		return
	}

	if err := h.service.CreateGame(game); err != nil {
		HandleError(w, InternalError(err, "Failed to create game"), "create_game")
		return
	}

	// Save mods for this game
	mods := parseMods(r, game.ID)
	if err := h.service.SaveModsForGame(game.ID, mods); err != nil {
		HandleError(w, InternalError(err, "Failed to save mods"), "create_game_mods")
		return
	}

	h.htmxRedirect(w, "/games/"+game.ID)
}

// UpdateGame updates an existing game
func (h *Handlers) UpdateGame(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	// Get existing game to preserve timestamps
	existingGame, err := h.service.GetGame(id)
	if err != nil {
		HandleError(w, NotFound("Game"), "update_game")
		return
	}

	game, err := h.parseGameForm(r)
	if err != nil {
		HandleError(w, err, "update_game_form")
		return
	}

	// Preserve ID and created timestamp
	game.ID = id
	game.CreatedAt = existingGame.CreatedAt

	if err := h.service.UpdateGame(game); err != nil {
		HandleError(w, InternalError(err, "Failed to update game"), "update_game")
		return
	}

	// Save mods for this game
	mods := parseMods(r, id)
	if err := h.service.SaveModsForGame(id, mods); err != nil {
		HandleError(w, InternalError(err, "Failed to save mods"), "update_game_mods")
		return
	}

	h.htmxRedirect(w, "/games/"+id)
}

// DeleteGame deletes a game
func (h *Handlers) DeleteGame(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	// Check if any gameservers are using this game
	count, err := h.service.CountGameserversByGameID(id)
	if err != nil {
		HandleError(w, InternalError(err, "Failed to check gameservers"), "delete_game")
		return
	}

	if count > 0 {
		HandleError(w, BadRequest("Cannot delete game: %d gameserver(s) are using it", count), "delete_game")
		return
	}

	if err := h.service.DeleteGame(id); err != nil {
		HandleError(w, InternalError(err, "Failed to delete game"), "delete_game")
		return
	}

	w.WriteHeader(http.StatusOK)
}

// getGame is a helper function to get a game with error handling
func (h *Handlers) getGame(w http.ResponseWriter, id string) (*models.Game, bool) {
	game, err := h.service.GetGame(id)
	if err != nil {
		HandleError(w, NotFound("Game"), "get_game")
		return nil, false
	}
	return game, true
}

// parseGameForm parses and validates game form data
func (h *Handlers) parseGameForm(r *http.Request) (*models.Game, error) {
	if err := ParseForm(r); err != nil {
		return nil, err
	}

	id := strings.TrimSpace(r.FormValue("id"))
	name := strings.TrimSpace(r.FormValue("name"))
	slug := strings.TrimSpace(r.FormValue("slug"))
	image := strings.TrimSpace(r.FormValue("image"))

	if id == "" || name == "" || image == "" {
		return nil, BadRequest("id, name, and image are required")
	}

	iconPath := strings.TrimSpace(r.FormValue("icon_path"))
	gridImagePath := strings.TrimSpace(r.FormValue("grid_image_path"))

	minMemoryMB, _ := strconv.Atoi(r.FormValue("min_memory_mb"))
	recMemoryMB, _ := strconv.Atoi(r.FormValue("rec_memory_mb"))

	if minMemoryMB <= 0 {
		minMemoryMB = 512
	}
	if recMemoryMB <= 0 {
		recMemoryMB = 1024
	}
	if recMemoryMB < minMemoryMB {
		recMemoryMB = minMemoryMB
	}

	// Parse port mappings
	portMappings := parsePortMappings(r)

	// Parse config vars
	configVars := parseConfigVars(r)

	return &models.Game{
		ID:            id,
		Name:          name,
		Slug:          slug,
		Image:         image,
		IconPath:      iconPath,
		GridImagePath: gridImagePath,
		MinMemoryMB:   minMemoryMB,
		RecMemoryMB:   recMemoryMB,
		PortMappings:  portMappings,
		ConfigVars:    configVars,
	}, nil
}

// parsePortMappings parses port mappings from form data
func parsePortMappings(r *http.Request) []models.PortMapping {
	var portMappings []models.PortMapping

	// Get all port mapping indices
	for i := 0; ; i++ {
		nameKey := "port_mappings[" + strconv.Itoa(i) + "].name"
		name := strings.TrimSpace(r.FormValue(nameKey))
		if name == "" {
			break
		}

		protocolKey := "port_mappings[" + strconv.Itoa(i) + "].protocol"
		protocol := strings.TrimSpace(r.FormValue(protocolKey))
		if protocol == "" {
			protocol = "tcp"
		}

		containerPortKey := "port_mappings[" + strconv.Itoa(i) + "].container_port"
		containerPort, _ := strconv.Atoi(r.FormValue(containerPortKey))

		if containerPort > 0 {
			portMappings = append(portMappings, models.PortMapping{
				Name:          name,
				Protocol:      protocol,
				ContainerPort: containerPort,
				HostPort:      0, // Auto-assign on gameserver creation
			})
		}
	}

	return portMappings
}

// parseConfigVars parses config vars from form data
func parseConfigVars(r *http.Request) []models.ConfigVar {
	var configVars []models.ConfigVar

	// Get all config var indices
	for i := 0; ; i++ {
		nameKey := "config_vars[" + strconv.Itoa(i) + "].name"
		name := strings.TrimSpace(r.FormValue(nameKey))
		if name == "" {
			break
		}

		displayNameKey := "config_vars[" + strconv.Itoa(i) + "].display_name"
		displayName := strings.TrimSpace(r.FormValue(displayNameKey))

		typeKey := "config_vars[" + strconv.Itoa(i) + "].type"
		varType := strings.TrimSpace(r.FormValue(typeKey))
		if varType == "" {
			varType = "text"
		}

		optionsKey := "config_vars[" + strconv.Itoa(i) + "].options"
		options := strings.TrimSpace(r.FormValue(optionsKey))

		requiredKey := "config_vars[" + strconv.Itoa(i) + "].required"
		required := r.FormValue(requiredKey) == "true" || r.FormValue(requiredKey) == "on"

		defaultKey := "config_vars[" + strconv.Itoa(i) + "].default"
		defaultValue := strings.TrimSpace(r.FormValue(defaultKey))

		descriptionKey := "config_vars[" + strconv.Itoa(i) + "].description"
		description := strings.TrimSpace(r.FormValue(descriptionKey))

		configVars = append(configVars, models.ConfigVar{
			Name:        name,
			DisplayName: displayName,
			Type:        varType,
			Options:     options,
			Required:    required,
			Default:     defaultValue,
			Description: description,
		})
	}

	return configVars
}

// parseMods parses mods from form data
func parseMods(r *http.Request, gameID string) []*models.Mod {
	var mods []*models.Mod

	for i := 0; ; i++ {
		idKey := "mods[" + strconv.Itoa(i) + "].id"
		id := strings.TrimSpace(r.FormValue(idKey))
		if id == "" {
			break
		}

		nameKey := "mods[" + strconv.Itoa(i) + "].name"
		name := strings.TrimSpace(r.FormValue(nameKey))

		descriptionKey := "mods[" + strconv.Itoa(i) + "].description"
		description := strings.TrimSpace(r.FormValue(descriptionKey))

		mods = append(mods, &models.Mod{
			ID:          id,
			GameID:      gameID,
			Name:        name,
			Description: description,
		})
	}

	return mods
}

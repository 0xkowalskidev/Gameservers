package main

import (
	"bufio"
	"fmt"
	"html/template"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/rs/zerolog/log"
)

type Handlers struct {
	service *GameServerService
	tmpl    *template.Template
}

func NewHandlers(service *GameServerService, tmpl *template.Template) *Handlers {
	return &Handlers{service: service, tmpl: tmpl}
}

func (h *Handlers) IndexGameservers(w http.ResponseWriter, r *http.Request) {
	gameservers, err := h.service.ListGameServers()
	if err != nil {
		log.Error().Err(err).Msg("Failed to list gameservers")
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	Render(w, r, h.tmpl, "index.html", map[string]interface{}{"Gameservers": gameservers})
}

func (h *Handlers) ShowGameserver(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	gameserver, err := h.service.GetGameServer(id)
	if err != nil {
		http.Error(w, "Gameserver not found", http.StatusNotFound)
		return
	}
	Render(w, r, h.tmpl, "gameserver-details.html", map[string]interface{}{"Gameserver": gameserver})
}

func (h *Handlers) NewGameserver(w http.ResponseWriter, r *http.Request) {
	games, err := h.service.ListGames()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	Render(w, r, h.tmpl, "new-gameserver.html", map[string]interface{}{"Games": games})
}

func (h *Handlers) CreateGameserver(w http.ResponseWriter, r *http.Request) {
	r.ParseForm()

	port, _ := strconv.Atoi(r.FormValue("port"))
	env := strings.Split(r.FormValue("environment"), "\n")
	if len(env) == 1 && env[0] == "" {
		env = []string{}
	}

	server := &GameServer{
		ID:          generateID(),
		Name:        r.FormValue("name"),
		GameID:      r.FormValue("game_id"),
		Port:        port,
		Environment: env,
	}

	log.Info().Str("gameserver_id", server.ID).Str("name", server.Name).Msg("Creating gameserver")

	if err := h.service.CreateGameServer(server); err != nil {
		log.Error().Err(err).Str("gameserver_id", server.ID).Str("name", server.Name).Msg("Failed to create gameserver")
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("HX-Redirect", "/")
	w.WriteHeader(http.StatusOK)
}

func (h *Handlers) StartGameserver(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	log.Info().Str("gameserver_id", id).Msg("Starting gameserver")
	
	if err := h.service.StartGameServer(id); err != nil {
		log.Error().Err(err).Str("gameserver_id", id).Msg("Failed to start gameserver")
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	
	h.GameserverRow(w, r)
}

func (h *Handlers) StopGameserver(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if err := h.service.StopGameServer(id); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	h.GameserverRow(w, r)
}

func (h *Handlers) RestartGameserver(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if err := h.service.RestartGameServer(id); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	h.GameserverRow(w, r)
}

func (h *Handlers) DestroyGameserver(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if err := h.service.DeleteGameServer(id); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusOK)
}

func (h *Handlers) GameserverRow(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	gameserver, err := h.service.GetGameServer(id)
	if err != nil {
		http.Error(w, "Gameserver not found", http.StatusNotFound)
		return
	}

	err = h.tmpl.ExecuteTemplate(w, "gameserver-row.html", gameserver)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func (h *Handlers) GameserverLogs(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	gameserver, err := h.service.GetGameServer(id)
	if err != nil {
		http.Error(w, "Gameserver not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "Streaming unsupported", http.StatusInternalServerError)
		return
	}

	logs, err := h.service.docker.StreamContainerLogs(gameserver.ContainerID)
	if err != nil {
		log.Error().Err(err).Str("gameserver_id", id).Msg("Failed to stream logs")
		fmt.Fprintf(w, "event: error\ndata: Failed to stream logs: %v\n\n", err)
		flusher.Flush()
		return
	}
	defer logs.Close()

	scanner := bufio.NewScanner(logs)
	for scanner.Scan() {
		line := scanner.Text()
		if len(line) > 8 {
			cleanLine := line[8:]
			if strings.TrimSpace(cleanLine) != "" {
				fmt.Fprintf(w, "event: log\ndata: %s\n\n", cleanLine)
				flusher.Flush()
			}
		}
	}
}

func generateID() string {
	return strconv.FormatInt(time.Now().UnixNano(), 36)
}


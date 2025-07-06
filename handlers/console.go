package handlers

import (
	"bufio"
	"fmt"
	"net/http"
	"strings"

	. "0xkowalskidev/gameservers/errors"
	"0xkowalskidev/gameservers/services"
	"github.com/go-chi/chi/v5"
	"github.com/rs/zerolog/log"
)

// ConsoleHandlers handles console-related HTTP requests
type ConsoleHandlers struct {
	*BaseHandlers
	gameserverService services.GameserverServiceInterface
}

// NewConsoleHandlers creates new console handlers
func NewConsoleHandlers(base *BaseHandlers, gameserverService services.GameserverServiceInterface) *ConsoleHandlers {
	return &ConsoleHandlers{
		BaseHandlers:      base,
		gameserverService: gameserverService,
	}
}

// GameserverConsole displays the console interface
func (h *ConsoleHandlers) GameserverConsole(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	gameserver, err := h.gameserverService.GetGameserver(id)
	if err != nil {
		h.HandleError(w, r, NotFound("Gameserver"))
		return
	}
	data := map[string]interface{}{
		"Gameserver": gameserver,
	}
	h.Render(w, r, "gameserver-console.html", data)
}

// SendGameserverCommand sends a command to the gameserver console
func (h *ConsoleHandlers) SendGameserverCommand(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if err := ParseForm(r); err != nil {
		h.HandleError(w, r, err)
		return
	}

	command := strings.TrimSpace(r.FormValue("command"))
	if command == "" {
		h.HandleError(w, r, BadRequest("command is required"))
		return
	}
	log.Info().Str("gameserver_id", id).Str("command", command).Msg("Sending console command")

	if err := h.gameserverService.SendGameserverCommand(id, command); err != nil {
		h.HandleError(w, r, InternalError(err, "Failed to send console command"))
		return
	}
	w.WriteHeader(http.StatusOK)
}

// GameserverLogs streams gameserver logs via Server-Sent Events
func (h *ConsoleHandlers) GameserverLogs(w http.ResponseWriter, r *http.Request) {
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

	logs, err := h.gameserverService.StreamGameserverLogs(r.Context(), id)
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


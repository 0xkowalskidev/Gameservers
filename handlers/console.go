package handlers

import (
	"bufio"
	"encoding/json"
	"fmt"
	"html/template"
	"net/http"
	"strings"

	"github.com/docker/docker/api/types/container"
	"github.com/go-chi/chi/v5"
	"github.com/rs/zerolog/log"
)

// GameserverConsole displays the console interface
func (h *Handlers) GameserverConsole(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	gameserver, ok := h.getGameserver(w, id)
	if !ok {
		return
	}
	h.renderGameserver(w, r, gameserver, "console", "gameserver-console.html", nil)
}

// SendGameserverCommand sends a command to the gameserver console
func (h *Handlers) SendGameserverCommand(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if err := h.validateFormFields(r, "command"); err != nil {
		HandleError(w, err, "send_command")
		return
	}

	command := r.FormValue("command")
	log.Info().Str("gameserver_id", id).Str("command", command).Msg("Sending console command")

	if err := h.service.SendGameserverCommand(id, command); err != nil {
		HandleError(w, InternalError(err, "Failed to send console command"), "send_command")
		return
	}
	w.WriteHeader(http.StatusOK)
}

// GameserverLogs streams gameserver logs via Server-Sent Events
func (h *Handlers) GameserverLogs(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	flusher, ok := w.(http.Flusher)
	if !ok {
		HandleError(w, InternalError(nil, "Streaming unsupported"), "gameserver_logs")
		return
	}

	logs, err := h.service.StreamGameserverLogs(id)
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
				// Escape HTML to prevent XSS
				cleanLine = template.HTMLEscapeString(cleanLine)
				fmt.Fprintf(w, "event: log\ndata: <div class=\"whitespace-pre-wrap break-all\">%s</div>\n\n", cleanLine)
				flusher.Flush()
			}
		}
	}
}

// GameserverStats streams gameserver statistics via Server-Sent Events
func (h *Handlers) GameserverStats(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	flusher, ok := w.(http.Flusher)
	if !ok {
		HandleError(w, InternalError(nil, "Streaming unsupported"), "gameserver_stats")
		return
	}

	stats, err := h.service.StreamGameserverStats(id)
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

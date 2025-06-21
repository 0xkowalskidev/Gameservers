package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"html/template"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/docker/docker/api/types/container"
	"github.com/go-chi/chi/v5"
	"github.com/rs/zerolog/log"
)

type GameserverServiceInterface interface {
	CreateGameserver(server *Gameserver) error
	GetGameserver(id string) (*Gameserver, error)
	UpdateGameserver(server *Gameserver) error
	ListGameservers() ([]*Gameserver, error)
	StartGameserver(id string) error
	StopGameserver(id string) error
	RestartGameserver(id string) error
	DeleteGameserver(id string) error
	StreamGameserverLogs(id string) (io.ReadCloser, error)
	StreamGameserverStats(id string) (io.ReadCloser, error)
	ListGames() ([]*Game, error)
	GetGame(id string) (*Game, error)
	CreateGame(game *Game) error
	CreateScheduledTask(task *ScheduledTask) error
	GetScheduledTask(id string) (*ScheduledTask, error)
	UpdateScheduledTask(task *ScheduledTask) error
	DeleteScheduledTask(id string) error
	ListScheduledTasksForGameserver(gameserverID string) ([]*ScheduledTask, error)
	RestoreGameserverBackup(gameserverID, backupPath string) error
}

type Handlers struct {
	service GameserverServiceInterface
	tmpl    *template.Template
}

func NewHandlers(service GameserverServiceInterface, tmpl *template.Template) *Handlers {
	return &Handlers{service: service, tmpl: tmpl}
}

func (h *Handlers) IndexGameservers(w http.ResponseWriter, r *http.Request) {
	gameservers, err := h.service.ListGameservers()
	if err != nil {
		log.Error().Err(err).Msg("Failed to list gameservers")
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	Render(w, r, h.tmpl, "index.html", map[string]interface{}{"Gameservers": gameservers})
}

func (h *Handlers) ShowGameserver(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	gameserver, err := h.service.GetGameserver(id)
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

func (h *Handlers) EditGameserver(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	gameserver, err := h.service.GetGameserver(id)
	if err != nil {
		http.Error(w, "Gameserver not found", http.StatusNotFound)
		return
	}
	
	games, err := h.service.ListGames()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	
	Render(w, r, h.tmpl, "edit-gameserver.html", map[string]interface{}{
		"Gameserver": gameserver,
		"Games":      games,
	})
}

func (h *Handlers) CreateGameserver(w http.ResponseWriter, r *http.Request) {
	r.ParseForm()

	port, _ := strconv.Atoi(r.FormValue("port"))
	memoryGB, _ := strconv.ParseFloat(r.FormValue("memory_gb"), 64)
	cpuCores, _ := strconv.ParseFloat(r.FormValue("cpu_cores"), 64)
	
	// Convert GB to MB for storage
	memoryMB := int(memoryGB * 1024)
	
	// Set default memory if not provided (1GB = 1024MB)
	if memoryMB <= 0 {
		memoryMB = 1024
	}
	
	// CPU cores are optional (0 = unlimited)
	
	env := strings.Split(r.FormValue("environment"), "\n")
	// Filter out empty lines and validate format
	var validEnv []string
	for _, line := range env {
		line = strings.TrimSpace(line)
		if line != "" && strings.Contains(line, "=") {
			validEnv = append(validEnv, line)
		}
	}
	env = validEnv

	server := &Gameserver{
		ID:          generateID(),
		Name:        r.FormValue("name"),
		GameID:      r.FormValue("game_id"),
		Port:        port,
		MemoryMB:    memoryMB,
		CPUCores:    cpuCores,
		Environment: env,
	}

	log.Info().Str("gameserver_id", server.ID).Str("name", server.Name).Int("memory_mb", memoryMB).Float64("cpu_cores", cpuCores).Msg("Creating gameserver")

	if err := h.service.CreateGameserver(server); err != nil {
		log.Error().Err(err).Str("gameserver_id", server.ID).Str("name", server.Name).Msg("Failed to create gameserver")
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("HX-Redirect", "/")
	w.WriteHeader(http.StatusOK)
}

func (h *Handlers) UpdateGameserver(w http.ResponseWriter, r *http.Request) {
	r.ParseForm()
	id := chi.URLParam(r, "id")

	port, _ := strconv.Atoi(r.FormValue("port"))
	memoryGB, _ := strconv.ParseFloat(r.FormValue("memory_gb"), 64)
	cpuCores, _ := strconv.ParseFloat(r.FormValue("cpu_cores"), 64)
	
	// Convert GB to MB for storage
	memoryMB := int(memoryGB * 1024)
	
	// Set default memory if not provided (1GB = 1024MB)
	if memoryMB <= 0 {
		memoryMB = 1024
	}
	
	// CPU cores are optional (0 = unlimited)
	
	env := strings.Split(r.FormValue("environment"), "\n")
	// Filter out empty lines and validate format
	var validEnv []string
	for _, line := range env {
		line = strings.TrimSpace(line)
		if line != "" && strings.Contains(line, "=") {
			validEnv = append(validEnv, line)
		}
	}
	env = validEnv

	server := &Gameserver{
		ID:          id,
		Name:        r.FormValue("name"),
		GameID:      r.FormValue("game_id"),
		Port:        port,
		MemoryMB:    memoryMB,
		CPUCores:    cpuCores,
		Environment: env,
	}

	log.Info().Str("gameserver_id", server.ID).Str("name", server.Name).Int("memory_mb", memoryMB).Float64("cpu_cores", cpuCores).Msg("Updating gameserver")

	if err := h.service.UpdateGameserver(server); err != nil {
		log.Error().Err(err).Str("gameserver_id", server.ID).Str("name", server.Name).Msg("Failed to update gameserver")
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("HX-Redirect", "/"+id)
	w.WriteHeader(http.StatusOK)
}

func (h *Handlers) StartGameserver(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	log.Info().Str("gameserver_id", id).Msg("Starting gameserver")

	if err := h.service.StartGameserver(id); err != nil {
		log.Error().Err(err).Str("gameserver_id", id).Msg("Failed to start gameserver")
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	h.GameserverRow(w, r)
}

func (h *Handlers) StopGameserver(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if err := h.service.StopGameserver(id); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	h.GameserverRow(w, r)
}

func (h *Handlers) RestartGameserver(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if err := h.service.RestartGameserver(id); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	h.GameserverRow(w, r)
}

func (h *Handlers) DestroyGameserver(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if err := h.service.DeleteGameserver(id); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusOK)
}

func (h *Handlers) GameserverRow(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	gameserver, err := h.service.GetGameserver(id)
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

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "Streaming unsupported", http.StatusInternalServerError)
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
				fmt.Fprintf(w, "event: log\ndata: %s\n\n", cleanLine)
				flusher.Flush()
			}
		}
	}
}

func (h *Handlers) GameserverStats(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "Streaming unsupported", http.StatusInternalServerError)
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

			memUsageGB := float64(memUsage) / (1024 * 1024 * 1024)
			memLimitGB := float64(memLimit) / (1024 * 1024 * 1024)

			data := fmt.Sprintf(`{"cpu":%.2f,"memoryUsageGB":%.2f,"memoryLimitGB":%.2f}`,
				cpuPercent, memUsageGB, memLimitGB)
			fmt.Fprintf(w, "event: stats\ndata: %s\n\n", data)
			flusher.Flush()
		}

		select {
		case <-r.Context().Done():
			return
		default:
		}
	}
}

// =============================================================================
// Scheduled Task Handlers
// =============================================================================

func (h *Handlers) ListGameserverTasks(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	
	tasks, err := h.service.ListScheduledTasksForGameserver(id)
	if err != nil {
		log.Error().Err(err).Str("gameserver_id", id).Msg("Failed to list scheduled tasks")
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	gameserver, err := h.service.GetGameserver(id)
	if err != nil {
		http.Error(w, "Gameserver not found", http.StatusNotFound)
		return
	}

	data := map[string]interface{}{
		"Gameserver": gameserver,
		"Tasks":      tasks,
	}

	Render(w, r, h.tmpl, "gameserver-tasks.html", data)
}

func (h *Handlers) NewGameserverTask(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	
	gameserver, err := h.service.GetGameserver(id)
	if err != nil {
		http.Error(w, "Gameserver not found", http.StatusNotFound)
		return
	}

	data := map[string]interface{}{
		"Gameserver": gameserver,
	}

	Render(w, r, h.tmpl, "new-task.html", data)
}

func (h *Handlers) CreateGameserverTask(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	r.ParseForm()

	task := &ScheduledTask{
		GameserverID: id,
		Name:         r.FormValue("name"),
		Type:         TaskType(r.FormValue("type")),
		Status:       TaskStatusActive,
		CronSchedule: r.FormValue("cron_schedule"),
	}

	log.Info().Str("gameserver_id", id).Str("task_name", task.Name).Str("type", string(task.Type)).Str("cron", task.CronSchedule).Msg("Creating scheduled task")

	if err := h.service.CreateScheduledTask(task); err != nil {
		log.Error().Err(err).Str("gameserver_id", id).Str("task_name", task.Name).Msg("Failed to create scheduled task")
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("HX-Redirect", fmt.Sprintf("/%s/tasks", id))
	w.WriteHeader(http.StatusOK)
}

func (h *Handlers) EditGameserverTask(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	taskID := chi.URLParam(r, "taskId")

	gameserver, err := h.service.GetGameserver(id)
	if err != nil {
		http.Error(w, "Gameserver not found", http.StatusNotFound)
		return
	}

	task, err := h.service.GetScheduledTask(taskID)
	if err != nil {
		http.Error(w, "Task not found", http.StatusNotFound)
		return
	}

	data := map[string]interface{}{
		"Gameserver": gameserver,
		"Task":       task,
	}

	Render(w, r, h.tmpl, "edit-task.html", data)
}

func (h *Handlers) UpdateGameserverTask(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	taskID := chi.URLParam(r, "taskId")
	r.ParseForm()

	task, err := h.service.GetScheduledTask(taskID)
	if err != nil {
		http.Error(w, "Task not found", http.StatusNotFound)
		return
	}

	task.Name = r.FormValue("name")
	task.Type = TaskType(r.FormValue("type"))
	task.Status = TaskStatus(r.FormValue("status"))
	task.CronSchedule = r.FormValue("cron_schedule")

	log.Info().Str("task_id", taskID).Str("task_name", task.Name).Msg("Updating scheduled task")

	if err := h.service.UpdateScheduledTask(task); err != nil {
		log.Error().Err(err).Str("task_id", taskID).Msg("Failed to update scheduled task")
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("HX-Redirect", fmt.Sprintf("/%s/tasks", id))
	w.WriteHeader(http.StatusOK)
}

func (h *Handlers) DeleteGameserverTask(w http.ResponseWriter, r *http.Request) {
	taskID := chi.URLParam(r, "taskId")

	if err := h.service.DeleteScheduledTask(taskID); err != nil {
		log.Error().Err(err).Str("task_id", taskID).Msg("Failed to delete scheduled task")
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
}

func (h *Handlers) RestoreGameserverBackup(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	r.ParseForm()
	
	backupPath := r.FormValue("backup_path")
	if backupPath == "" {
		http.Error(w, "Backup path required", http.StatusBadRequest)
		return
	}

	gameserver, err := h.service.GetGameserver(id)
	if err != nil {
		http.Error(w, "Gameserver not found", http.StatusNotFound)
		return
	}

	log.Info().Str("gameserver_id", id).Str("backup_path", backupPath).Msg("Restoring backup")

	if err := h.service.RestoreGameserverBackup(gameserver.ID, backupPath); err != nil {
		log.Error().Err(err).Str("gameserver_id", id).Str("backup_path", backupPath).Msg("Failed to restore backup")
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("HX-Redirect", fmt.Sprintf("/%s", id))
	w.WriteHeader(http.StatusOK)
}

func generateID() string {
	return strconv.FormatInt(time.Now().UnixNano(), 36)
}

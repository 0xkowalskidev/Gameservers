package main

import (
	"archive/tar"
	"bufio"
	"encoding/json"
	"fmt"
	"html/template"
	"io"
	"net/http"
	"path/filepath"
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
	// File operations
	ListFiles(containerID string, path string) ([]*FileInfo, error)
	ReadFile(containerID string, path string) ([]byte, error)
	WriteFile(containerID string, path string, content []byte) error
	CreateDirectory(containerID string, path string) error
	DeletePath(containerID string, path string) error
	DownloadFile(containerID string, path string) (io.ReadCloser, error)
	RenameFile(containerID string, oldPath string, newPath string) error
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

// =============================================================================
// File Manager Handlers
// =============================================================================

func (h *Handlers) GameserverFiles(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	
	gameserver, err := h.service.GetGameserver(id)
	if err != nil {
		http.Error(w, "Gameserver not found", http.StatusNotFound)
		return
	}
	
	// Get root directory listing - always start at /data/server
	files, err := h.service.ListFiles(gameserver.ContainerID, "/data/server")
	if err != nil {
		log.Error().Err(err).Str("gameserver_id", id).Msg("Failed to list files")
	}
	
	data := map[string]interface{}{
		"Gameserver": gameserver,
		"Files":      files,
		"CurrentPath": "/data/server",
	}
	
	Render(w, r, h.tmpl, "gameserver-files.html", data)
}

func (h *Handlers) BrowseGameserverFiles(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	path := r.URL.Query().Get("path")
	
	if path == "" {
		path = "/data/server"
	}
	
	// Sanitize path
	path = sanitizePath(path)
	
	gameserver, err := h.service.GetGameserver(id)
	if err != nil {
		http.Error(w, "Gameserver not found", http.StatusNotFound)
		return
	}
	
	files, err := h.service.ListFiles(gameserver.ContainerID, path)
	if err != nil {
		log.Error().Err(err).Str("gameserver_id", id).Str("path", path).Msg("Failed to list files")
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	
	data := map[string]interface{}{
		"Gameserver": gameserver,
		"Files":      files,
		"CurrentPath": path,
	}
	
	// Return partial for HTMX
	err = h.tmpl.ExecuteTemplate(w, "file-browser.html", data)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func (h *Handlers) GameserverFileContent(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	path := r.URL.Query().Get("path")
	
	if path == "" {
		http.Error(w, "Path required", http.StatusBadRequest)
		return
	}
	
	// Sanitize path
	path = sanitizePath(path)
	
	// Check if file is editable based on extension FIRST
	isEditable := isEditableFile(path)
	if !isEditable {
		// Don't read the file content if it's not editable
		data := map[string]interface{}{
			"Path":      path,
			"Content":   "",
			"Supported": false,
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(data)
		return
	}
	
	gameserver, err := h.service.GetGameserver(id)
	if err != nil {
		http.Error(w, "Gameserver not found", http.StatusNotFound)
		return
	}
	
	// Only read file content if it's editable
	content, err := h.service.ReadFile(gameserver.ContainerID, path)
	if err != nil {
		log.Error().Err(err).Str("gameserver_id", id).Str("path", path).Msg("Failed to read file")
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	
	data := map[string]interface{}{
		"Path":      path,
		"Content":   string(content),
		"Supported": true,
	}
	
	// Return JSON for editor
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(data)
}

func (h *Handlers) SaveGameserverFile(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	r.ParseForm()
	
	path := r.FormValue("path")
	content := r.FormValue("content")
	
	if path == "" {
		http.Error(w, "Path required", http.StatusBadRequest)
		return
	}
	
	// Sanitize path
	path = sanitizePath(path)
	
	gameserver, err := h.service.GetGameserver(id)
	if err != nil {
		http.Error(w, "Gameserver not found", http.StatusNotFound)
		return
	}
	
	err = h.service.WriteFile(gameserver.ContainerID, path, []byte(content))
	if err != nil {
		log.Error().Err(err).Str("gameserver_id", id).Str("path", path).Msg("Failed to write file")
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"status": "saved"})
}

func (h *Handlers) DownloadGameserverFile(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	path := r.URL.Query().Get("path")
	
	if path == "" {
		http.Error(w, "Path required", http.StatusBadRequest)
		return
	}
	
	// Sanitize path
	path = sanitizePath(path)
	
	gameserver, err := h.service.GetGameserver(id)
	if err != nil {
		http.Error(w, "Gameserver not found", http.StatusNotFound)
		return
	}
	
	reader, err := h.service.DownloadFile(gameserver.ContainerID, path)
	if err != nil {
		log.Error().Err(err).Str("gameserver_id", id).Str("path", path).Msg("Failed to download file")
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer reader.Close()
	
	// Extract filename from path
	filename := filepath.Base(path)
	
	// The reader contains a tar archive, we need to extract the file
	tarReader := tar.NewReader(reader)
	
	// Read the first (and should be only) file from the tar
	header, err := tarReader.Next()
	if err != nil {
		log.Error().Err(err).Str("gameserver_id", id).Str("path", path).Msg("Failed to read tar header")
		http.Error(w, "Failed to read file from archive", http.StatusInternalServerError)
		return
	}
	
	// Set headers for download
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", filename))
	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Content-Length", strconv.FormatInt(header.Size, 10))
	
	// Stream the actual file content (not the tar archive)
	io.Copy(w, tarReader)
}

func (h *Handlers) CreateGameserverFile(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	r.ParseForm()
	
	path := r.FormValue("path")
	name := r.FormValue("name")
	isDir := r.FormValue("type") == "directory"
	
	if path == "" || name == "" {
		http.Error(w, "Path and name required", http.StatusBadRequest)
		return
	}
	
	// Sanitize inputs
	path = sanitizePath(path)
	fullPath := filepath.Join(path, name)
	
	gameserver, err := h.service.GetGameserver(id)
	if err != nil {
		http.Error(w, "Gameserver not found", http.StatusNotFound)
		return
	}
	
	if isDir {
		err = h.service.CreateDirectory(gameserver.ContainerID, fullPath)
	} else {
		// Create empty file
		err = h.service.WriteFile(gameserver.ContainerID, fullPath, []byte(""))
	}
	
	if err != nil {
		log.Error().Err(err).Str("gameserver_id", id).Str("path", fullPath).Msg("Failed to create file/directory")
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	
	// Return updated file listing
	h.BrowseGameserverFiles(w, r)
}

func (h *Handlers) DeleteGameserverFile(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	path := r.URL.Query().Get("path")
	
	if path == "" {
		http.Error(w, "Path required", http.StatusBadRequest)
		return
	}
	
	// Sanitize path
	path = sanitizePath(path)
	
	gameserver, err := h.service.GetGameserver(id)
	if err != nil {
		http.Error(w, "Gameserver not found", http.StatusNotFound)
		return
	}
	
	err = h.service.DeletePath(gameserver.ContainerID, path)
	if err != nil {
		log.Error().Err(err).Str("gameserver_id", id).Str("path", path).Msg("Failed to delete file/directory")
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	
	w.WriteHeader(http.StatusOK)
}

func (h *Handlers) RenameGameserverFile(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	r.ParseForm()
	
	oldPath := r.FormValue("old_path")
	newName := r.FormValue("new_name")
	
	if oldPath == "" || newName == "" {
		http.Error(w, "Old path and new name required", http.StatusBadRequest)
		return
	}
	
	// Sanitize paths
	oldPath = sanitizePath(oldPath)
	newPath := sanitizePath(filepath.Join(filepath.Dir(oldPath), newName))
	
	gameserver, err := h.service.GetGameserver(id)
	if err != nil {
		http.Error(w, "Gameserver not found", http.StatusNotFound)
		return
	}
	
	err = h.service.RenameFile(gameserver.ContainerID, oldPath, newPath)
	if err != nil {
		log.Error().Err(err).Str("gameserver_id", id).Str("old_path", oldPath).Str("new_path", newPath).Msg("Failed to rename file")
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	
	// Return updated file listing
	h.BrowseGameserverFiles(w, r)
}

// Helper functions

func sanitizePath(path string) string {
	// Server directory is the user root
	const serverDir = "/data/server"
	
	// Clean the path
	path = filepath.Clean(path)
	
	// If path is empty or just "/", use server directory
	if path == "" || path == "/" {
		return serverDir
	}
	
	// Ensure path is absolute
	if !filepath.IsAbs(path) {
		path = "/" + path
	}
	
	// If path doesn't start with /data/server, prepend it
	if !strings.HasPrefix(path, serverDir) {
		// If user is trying to access parent directories, return server root
		if strings.HasPrefix(path, "/..") || path == ".." {
			return serverDir
		}
		// Otherwise, append the path to /data/server
		path = filepath.Join(serverDir, path)
	}
	
	// Clean again to resolve any .. sequences
	path = filepath.Clean(path)
	
	// Final check - ensure we're still within /data/server
	if !strings.HasPrefix(path, serverDir) {
		return serverDir
	}
	
	return path
}

func isEditableFile(filename string) bool {
	// Get file extension
	ext := strings.ToLower(filepath.Ext(filename))
	
	// Whitelist of editable file extensions
	editableExtensions := map[string]bool{
		".txt":        true,
		".json":       true,
		".xml":        true,
		".yaml":       true,
		".yml":        true,
		".toml":       true,
		".ini":        true,
		".conf":       true,
		".config":     true,
		".cfg":        true,
		".properties": true,
		".log":        true,
		".md":         true,
		".js":         true,
		".ts":         true,
		".html":       true,
		".htm":        true,
		".css":        true,
		".scss":       true,
		".less":       true,
		".sql":        true,
		".sh":         true,
		".bash":       true,
		".bat":        true,
		".cmd":        true,
		".ps1":        true,
		".py":         true,
		".go":         true,
		".java":       true,
		".c":          true,
		".cpp":        true,
		".h":          true,
		".hpp":        true,
		".cs":         true,
		".php":        true,
		".rb":         true,
		".pl":         true,
		".r":          true,
		".lua":        true,
		".dockerfile": true,
		".dockerignore": true,
		".gitignore":  true,
		".env":        true,
		".example":    true,
		"":            true, // Files without extension (like README, LICENSE)
	}
	
	// Special cases for files without extension that are typically text
	if ext == "" {
		baseName := strings.ToLower(filepath.Base(filename))
		textFiles := map[string]bool{
			"readme":     true,
			"license":    true,
			"changelog":  true,
			"authors":    true,
			"contributors": true,
			"copying":    true,
			"install":    true,
			"news":       true,
			"todo":       true,
			"makefile":   true,
			"dockerfile": true,
			"vagrantfile": true,
		}
		
		if textFiles[baseName] {
			return true
		}
	}
	
	return editableExtensions[ext]
}

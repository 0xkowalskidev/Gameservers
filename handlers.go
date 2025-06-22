package main

import (
	"archive/tar"
	"bufio"
	"bytes"
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
	SendGameserverCommand(id string, command string) error
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
	CreateGameserverBackup(gameserverID string) error
	RestoreGameserverBackup(gameserverID, backupFilename string) error
	ListGameserverBackups(gameserverID string) ([]*FileInfo, error)
	// File operations
	ListFiles(containerID string, path string) ([]*FileInfo, error)
	ReadFile(containerID string, path string) ([]byte, error)
	WriteFile(containerID string, path string, content []byte) error
	CreateDirectory(containerID string, path string) error
	DeletePath(containerID string, path string) error
	DownloadFile(containerID string, path string) (io.ReadCloser, error)
	RenameFile(containerID string, oldPath string, newPath string) error
	UploadFile(containerID string, destPath string, reader io.Reader) error
}

type Handlers struct {
	service GameserverServiceInterface
	tmpl    *template.Template
}

func NewHandlers(service GameserverServiceInterface, tmpl *template.Template) *Handlers {
	return &Handlers{service: service, tmpl: tmpl}
}

// Helper function to get gameserver with error handling
func (h *Handlers) getGameserver(w http.ResponseWriter, id string) (*Gameserver, bool) {
	gameserver, err := h.service.GetGameserver(id)
	if err != nil {
		HandleError(w, NotFound("Gameserver"), "get_gameserver")
		return nil, false
	}
	return gameserver, true
}

// Helper function to handle redirects with HTMX
func (h *Handlers) htmxRedirect(w http.ResponseWriter, url string) {
	w.Header().Set("HX-Redirect", url)
	w.WriteHeader(http.StatusOK)
}

// Helper function to handle standard JSON response
func (h *Handlers) jsonResponse(w http.ResponseWriter, data interface{}) {
	h.jsonResponse(w, data)
}

// Helper function to parse gameserver form data
type GameserverFormData struct {
	Name        string
	GameID      string
	Port        int
	MemoryMB    int
	CPUCores    float64
	MaxBackups  int
	Environment []string
}

func parseGameserverForm(r *http.Request) (*GameserverFormData, error) {
	if err := ParseForm(r); err != nil {
		return nil, err
	}

	port, _ := strconv.Atoi(r.FormValue("port"))
	memoryGB, _ := strconv.ParseFloat(r.FormValue("memory_gb"), 64)
	cpuCores, _ := strconv.ParseFloat(r.FormValue("cpu_cores"), 64)
	maxBackups, _ := strconv.Atoi(r.FormValue("max_backups"))
	
	// Convert GB to MB for storage
	memoryMB := int(memoryGB * 1024)
	if memoryMB <= 0 {
		memoryMB = 1024 // Default 1GB
	}
	if maxBackups <= 0 {
		maxBackups = 7 // Default 7 backups
	}

	// Parse and validate environment variables
	env := strings.Split(r.FormValue("environment"), "\n")
	var validEnv []string
	for _, line := range env {
		line = strings.TrimSpace(line)
		if line != "" && strings.Contains(line, "=") {
			validEnv = append(validEnv, line)
		}
	}

	return &GameserverFormData{
		Name:        r.FormValue("name"),
		GameID:      r.FormValue("game_id"),
		Port:        port,
		MemoryMB:    memoryMB,
		CPUCores:    cpuCores,
		MaxBackups:  maxBackups,
		Environment: validEnv,
	}, nil
}

// Helper function to parse scheduled task form data
func parseScheduledTaskForm(r *http.Request, gameserverID string) (*ScheduledTask, error) {
	if err := ParseForm(r); err != nil {
		return nil, err
	}

	return &ScheduledTask{
		GameserverID: gameserverID,
		Name:         r.FormValue("name"),
		Type:         TaskType(r.FormValue("type")),
		Status:       TaskStatusActive,
		CronSchedule: r.FormValue("cron_schedule"),
	}, nil
}

// Helper function to update task from form data
func updateTaskFromForm(task *ScheduledTask, r *http.Request) error {
	if err := ParseForm(r); err != nil {
		return err
	}

	task.Name = r.FormValue("name")
	task.Type = TaskType(r.FormValue("type"))
	task.Status = TaskStatus(r.FormValue("status"))
	task.CronSchedule = r.FormValue("cron_schedule")
	return nil
}

func (h *Handlers) renderGameserverPage(w http.ResponseWriter, r *http.Request, gameserver *Gameserver, currentPage string, contentTemplate string, data map[string]interface{}) {
	// Render the content template to get the inner content
	var buf bytes.Buffer
	err := h.tmpl.ExecuteTemplate(&buf, contentTemplate, data)
	if err != nil {
		HandleError(w, InternalError(err, "Failed to render content template"), "render_gameserver_page")
		return
	}
	
	// Wrap with gameserver wrapper
	wrapperData := map[string]interface{}{
		"Gameserver":  gameserver,
		"CurrentPage": currentPage,
		"Content":     template.HTML(buf.String()),
	}
	
	Render(w, r, h.tmpl, "gameserver-wrapper.html", wrapperData)
}

// Helper function to require query parameter and return error if missing
func requireQueryParam(r *http.Request, param string) (string, error) {
	value := r.URL.Query().Get(param)
	if value == "" {
		return "", BadRequest("%s parameter required", param)
	}
	return value, nil
}

// Helper function to validate form fields
func validateFormFields(r *http.Request, fields ...string) error {
	if err := ParseForm(r); err != nil {
		return err
	}
	
	for _, field := range fields {
		if r.FormValue(field) == "" {
			return BadRequest("%s is required", field)
		}
	}
	return nil
}

// Helper function to handle HTMX-aware rendering with gameserver wrapper
func (h *Handlers) renderWithGameserverContext(w http.ResponseWriter, r *http.Request, gameserver *Gameserver, currentPage string, templateName string, data map[string]interface{}) {
	// If HTMX request, render just the content
	if r.Header.Get("HX-Request") == "true" {
		Render(w, r, h.tmpl, templateName, data)
	} else {
		// Full page load, use wrapper
		h.renderGameserverPage(w, r, gameserver, currentPage, templateName, data)
	}
}

func (h *Handlers) IndexGameservers(w http.ResponseWriter, r *http.Request) {
	gameservers, err := h.service.ListGameservers()
	if err != nil {
		HandleError(w, InternalError(err, "Failed to list gameservers"), "index_gameservers")
		return
	}
	Render(w, r, h.tmpl, "index.html", map[string]interface{}{"Gameservers": gameservers})
}

func (h *Handlers) ShowGameserver(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	gameserver, ok := h.getGameserver(w, id)
	if !ok {
		return
	}
	
	data := map[string]interface{}{"Gameserver": gameserver}
	h.renderWithGameserverContext(w, r, gameserver, "overview", "gameserver-details.html", data)
}

func (h *Handlers) NewGameserver(w http.ResponseWriter, r *http.Request) {
	games, err := h.service.ListGames()
	if err != nil {
		HandleError(w, InternalError(err, "Failed to list games"), "new_gameserver")
		return
	}
	Render(w, r, h.tmpl, "new-gameserver.html", map[string]interface{}{"Games": games})
}

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
	
	h.renderWithGameserverContext(w, r, gameserver, "edit", "edit-gameserver.html", data)
}

func (h *Handlers) CreateGameserver(w http.ResponseWriter, r *http.Request) {
	formData, err := parseGameserverForm(r)
	if err != nil {
		HandleError(w, err, "create_gameserver_form")
		return
	}

	server := &Gameserver{
		ID:          generateID(),
		Name:        formData.Name,
		GameID:      formData.GameID,
		Port:        formData.Port,
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

func (h *Handlers) UpdateGameserver(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	formData, err := parseGameserverForm(r)
	if err != nil {
		HandleError(w, err, "update_gameserver_form")
		return
	}

	server := &Gameserver{
		ID:          id,
		Name:        formData.Name,
		GameID:      formData.GameID,
		Port:        formData.Port,
		MemoryMB:    formData.MemoryMB,
		CPUCores:    formData.CPUCores,
		MaxBackups:  formData.MaxBackups,
		Environment: formData.Environment,
	}

	log.Info().Str("gameserver_id", server.ID).Str("name", server.Name).Int("memory_mb", formData.MemoryMB).Float64("cpu_cores", formData.CPUCores).Msg("Updating gameserver")

	if err := h.service.UpdateGameserver(server); err != nil {
		HandleError(w, InternalError(err, "Failed to update gameserver"), "update_gameserver")
		return
	}

	h.htmxRedirect(w, "/"+id)
}

func (h *Handlers) StartGameserver(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	log.Info().Str("gameserver_id", id).Msg("Starting gameserver")

	if err := h.service.StartGameserver(id); err != nil {
		HandleError(w, InternalError(err, "Failed to start gameserver"), "start_gameserver")
		return
	}

	h.GameserverRow(w, r)
}

func (h *Handlers) StopGameserver(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if err := h.service.StopGameserver(id); err != nil {
		HandleError(w, InternalError(err, "Failed to stop gameserver"), "stop_gameserver")
		return
	}
	h.GameserverRow(w, r)
}

func (h *Handlers) RestartGameserver(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if err := h.service.RestartGameserver(id); err != nil {
		HandleError(w, InternalError(err, "Failed to restart gameserver"), "restart_gameserver")
		return
	}
	h.GameserverRow(w, r)
}

func (h *Handlers) GameserverConsole(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	gameserver, ok := h.getGameserver(w, id)
	if !ok {
		return
	}
	
	data := map[string]interface{}{"Gameserver": gameserver}
	h.renderWithGameserverContext(w, r, gameserver, "console", "gameserver-console.html", data)
}

func (h *Handlers) SendGameserverCommand(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if err := validateFormFields(r, "command"); err != nil {
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

func (h *Handlers) DestroyGameserver(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if err := h.service.DeleteGameserver(id); err != nil {
		HandleError(w, InternalError(err, "Failed to delete gameserver"), "destroy_gameserver")
		return
	}
	w.WriteHeader(http.StatusOK)
}

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
	gameserver, ok := h.getGameserver(w, id)
	if !ok {
		return
	}
	
	tasks, err := h.service.ListScheduledTasksForGameserver(id)
	if err != nil {
		HandleError(w, InternalError(err, "Failed to list scheduled tasks"), "list_tasks")
		return
	}

	data := map[string]interface{}{
		"Gameserver": gameserver,
		"Tasks":      tasks,
	}

	h.renderWithGameserverContext(w, r, gameserver, "tasks", "gameserver-tasks.html", data)
}

func (h *Handlers) NewGameserverTask(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	gameserver, ok := h.getGameserver(w, id)
	if !ok {
		return
	}

	data := map[string]interface{}{"Gameserver": gameserver}
	h.renderWithGameserverContext(w, r, gameserver, "tasks", "new-task.html", data)
}

func (h *Handlers) CreateGameserverTask(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	task, err := parseScheduledTaskForm(r, id)
	if err != nil {
		HandleError(w, err, "create_task_form")
		return
	}

	log.Info().Str("gameserver_id", id).Str("task_name", task.Name).Str("type", string(task.Type)).Str("cron", task.CronSchedule).Msg("Creating scheduled task")

	if err := h.service.CreateScheduledTask(task); err != nil {
		HandleError(w, InternalError(err, "Failed to create scheduled task"), "create_task")
		return
	}

	h.htmxRedirect(w, fmt.Sprintf("/%s/tasks", id))
}

func (h *Handlers) EditGameserverTask(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	taskID := chi.URLParam(r, "taskId")

	gameserver, ok := h.getGameserver(w, id)
	if !ok {
		return
	}

	task, err := h.service.GetScheduledTask(taskID)
	if err != nil {
		HandleError(w, NotFound("Task"), "edit_task")
		return
	}

	data := map[string]interface{}{
		"Gameserver": gameserver,
		"Task":       task,
	}

	h.renderWithGameserverContext(w, r, gameserver, "tasks", "edit-task.html", data)
}

func (h *Handlers) UpdateGameserverTask(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	taskID := chi.URLParam(r, "taskId")

	task, err := h.service.GetScheduledTask(taskID)
	if err != nil {
		HandleError(w, NotFound("Task"), "update_task")
		return
	}

	if err := updateTaskFromForm(task, r); err != nil {
		HandleError(w, err, "update_task_form")
		return
	}

	log.Info().Str("task_id", taskID).Str("task_name", task.Name).Msg("Updating scheduled task")

	if err := h.service.UpdateScheduledTask(task); err != nil {
		HandleError(w, InternalError(err, "Failed to update scheduled task"), "update_task")
		return
	}

	h.htmxRedirect(w, fmt.Sprintf("/%s/tasks", id))
}

func (h *Handlers) DeleteGameserverTask(w http.ResponseWriter, r *http.Request) {
	taskID := chi.URLParam(r, "taskId")

	if err := h.service.DeleteScheduledTask(taskID); err != nil {
		HandleError(w, InternalError(err, "Failed to delete scheduled task"), "delete_task")
		return
	}

	w.WriteHeader(http.StatusOK)
}

func (h *Handlers) RestoreGameserverBackup(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	backupFilename, err := requireQueryParam(r, "backup")
	if err != nil {
		HandleError(w, err, "restore_backup")
		return
	}

	gameserver, ok := h.getGameserver(w, id)
	if !ok {
		return
	}

	log.Info().Str("gameserver_id", id).Str("backup_filename", backupFilename).Msg("Restoring backup")

	if err := h.service.RestoreGameserverBackup(gameserver.ID, backupFilename); err != nil {
		HandleError(w, InternalError(err, "Failed to restore backup"), "restore_backup")
		return
	}

	w.WriteHeader(http.StatusOK)
}

func (h *Handlers) CreateGameserverBackup(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	
	log.Info().Str("gameserver_id", id).Msg("Creating backup")
	
	if err := h.service.CreateGameserverBackup(id); err != nil {
		HandleError(w, InternalError(err, "Failed to create backup"), "create_backup")
		return
	}
	
	w.WriteHeader(http.StatusOK)
}

func (h *Handlers) ListGameserverBackups(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	gameserver, ok := h.getGameserver(w, id)
	if !ok {
		return
	}
	
	// Get backup files
	backups, err := h.service.ListGameserverBackups(id)
	if err != nil {
		HandleError(w, InternalError(err, "Failed to list backup files"), "list_backups")
		return
	}
	
	data := map[string]interface{}{
		"Gameserver":   gameserver,
		"Backups":      backups,
		"GameserverID": id,
		"BackupCount":  len(backups),
		"MaxBackups":   gameserver.MaxBackups,
	}
	
	// If HTMX request, check if it's targeting a specific element
	if r.Header.Get("HX-Request") == "true" {
		// If the request is targeting #backup-list specifically, return just the list
		target := r.Header.Get("HX-Target")
		templateName := "gameserver-backups.html"
		if target == "#backup-list" || r.URL.Query().Get("list") == "true" {
			templateName = "backup-list.html"
		}
		if err := h.tmpl.ExecuteTemplate(w, templateName, data); err != nil {
			HandleError(w, InternalError(err, "Failed to render backup template"), "list_backups")
		}
	} else {
		// Full page load, use wrapper
		h.renderGameserverPage(w, r, gameserver, "backups", "gameserver-backups.html", data)
	}
}

func (h *Handlers) DeleteGameserverBackup(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	backupFilename, err := requireQueryParam(r, "backup")
	if err != nil {
		HandleError(w, err, "delete_backup")
		return
	}
	
	gameserver, ok := h.getGameserver(w, id)
	if !ok {
		return
	}
	
	log.Info().Str("gameserver_id", id).Str("backup_filename", backupFilename).Msg("Deleting backup")
	
	// Delete the backup file from /data/backups
	backupPath := fmt.Sprintf("/data/backups/%s", backupFilename)
	if err := h.service.DeletePath(gameserver.ContainerID, backupPath); err != nil {
		HandleError(w, InternalError(err, "Failed to delete backup"), "delete_backup")
		return
	}
	
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
	gameserver, ok := h.getGameserver(w, id)
	if !ok {
		return
	}
	
	// Get root directory listing - always start at /data/server
	files, err := h.service.ListFiles(gameserver.ContainerID, "/data/server")
	if err != nil {
		log.Error().Err(err).Str("gameserver_id", id).Msg("Failed to list files")
		// Continue with empty files list on error
	}
	
	data := map[string]interface{}{
		"Gameserver":  gameserver,
		"Files":       files,
		"CurrentPath": "/data/server",
	}
	
	h.renderWithGameserverContext(w, r, gameserver, "files", "gameserver-files.html", data)
}

func (h *Handlers) BrowseGameserverFiles(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	path := r.URL.Query().Get("path")
	
	if path == "" {
		path = "/data/server"
	}
	
	// Sanitize path
	path = sanitizePath(path)
	
	gameserver, ok := h.getGameserver(w, id)
	if !ok {
		return
	}
	
	files, err := h.service.ListFiles(gameserver.ContainerID, path)
	if err != nil {
		HandleError(w, InternalError(err, "Failed to list files"), "browse_files")
		return
	}
	
	data := map[string]interface{}{
		"Gameserver": gameserver,
		"Files":      files,
		"CurrentPath": path,
	}
	
	// Return partial for HTMX
	if err := h.tmpl.ExecuteTemplate(w, "file-browser.html", data); err != nil {
		HandleError(w, InternalError(err, "Failed to render file browser"), "browse_files")
	}
}

func (h *Handlers) GameserverFileContent(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	path, err := requireQueryParam(r, "path")
	if err != nil {
		data := map[string]interface{}{
			"Path":      "",
			"Content":   "",
			"Supported": false,
			"Error":     err.Error(),
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(data)
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
	
	gameserver, ok := h.getGameserver(w, id)
	if !ok {
		return
	}
	
	// Only read file content if it's editable
	content, err := h.service.ReadFile(gameserver.ContainerID, path)
	if err != nil {
		log.Error().Err(err).Str("path", path).Msg("Failed to read file")
		data := map[string]interface{}{
			"Path":      path,
			"Content":   "",
			"Supported": false,
			"Error":     "Failed to read file",
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(data)
		return
	}
	
	data := map[string]interface{}{
		"Path":      path,
		"Content":   string(content),
		"Supported": true,
	}
	
	// Return JSON for editor
	h.jsonResponse(w, data)
}

func (h *Handlers) SaveGameserverFile(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if err := validateFormFields(r, "path"); err != nil {
		HandleError(w, err, "save_file")
		return
	}
	
	path := r.FormValue("path")
	content := r.FormValue("content")
	
	// Sanitize path
	path = sanitizePath(path)
	
	gameserver, ok := h.getGameserver(w, id)
	if !ok {
		return
	}
	
	if err := h.service.WriteFile(gameserver.ContainerID, path, []byte(content)); err != nil {
		HandleError(w, InternalError(err, "Failed to write file"), "save_file")
		return
	}
	
	h.jsonResponse(w, map[string]string{"status": "saved"})
}

func (h *Handlers) DownloadGameserverFile(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	path, err := requireQueryParam(r, "path")
	if err != nil {
		HandleError(w, err, "download_file")
		return
	}
	
	// Sanitize path
	path = sanitizePath(path)
	
	gameserver, ok := h.getGameserver(w, id)
	if !ok {
		return
	}
	
	// Read file content directly
	content, err := h.service.ReadFile(gameserver.ContainerID, path)
	if err != nil {
		HandleError(w, InternalError(err, "Failed to download file"), "download_file")
		return
	}
	
	// Extract filename from path
	filename := filepath.Base(path)
	
	// Set headers for download
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", filename))
	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Content-Length", strconv.Itoa(len(content)))
	
	// Write the file content
	if _, err := w.Write(content); err != nil {
		log.Error().Err(err).Str("path", path).Msg("Failed to write file content")
	}
}

func (h *Handlers) CreateGameserverFile(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if err := validateFormFields(r, "path", "name"); err != nil {
		HandleError(w, err, "create_file")
		return
	}
	
	path := r.FormValue("path")
	name := r.FormValue("name")
	isDir := r.FormValue("type") == "directory"
	
	// Sanitize inputs
	path = sanitizePath(path)
	fullPath := filepath.Join(path, name)
	
	gameserver, ok := h.getGameserver(w, id)
	if !ok {
		return
	}
	
	var err error
	if isDir {
		err = h.service.CreateDirectory(gameserver.ContainerID, fullPath)
	} else {
		// Create empty file
		err = h.service.WriteFile(gameserver.ContainerID, fullPath, []byte(""))
	}
	
	if err != nil {
		HandleError(w, InternalError(err, "Failed to create file/directory"), "create_file")
		return
	}
	
	// Return updated file listing
	h.BrowseGameserverFiles(w, r)
}

func (h *Handlers) DeleteGameserverFile(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	path, err := requireQueryParam(r, "path")
	if err != nil {
		HandleError(w, err, "delete_file")
		return
	}
	
	// Sanitize path
	path = sanitizePath(path)
	
	gameserver, ok := h.getGameserver(w, id)
	if !ok {
		return
	}
	
	if err := h.service.DeletePath(gameserver.ContainerID, path); err != nil {
		HandleError(w, InternalError(err, "Failed to delete file/directory"), "delete_file")
		return
	}
	
	w.WriteHeader(http.StatusOK)
}

func (h *Handlers) RenameGameserverFile(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if err := validateFormFields(r, "old_path", "new_name"); err != nil {
		HandleError(w, err, "rename_file")
		return
	}
	
	oldPath := r.FormValue("old_path")
	newName := r.FormValue("new_name")
	
	// Sanitize paths
	oldPath = sanitizePath(oldPath)
	newPath := sanitizePath(filepath.Join(filepath.Dir(oldPath), newName))
	
	gameserver, ok := h.getGameserver(w, id)
	if !ok {
		return
	}
	
	if err := h.service.RenameFile(gameserver.ContainerID, oldPath, newPath); err != nil {
		HandleError(w, InternalError(err, "Failed to rename file"), "rename_file")
		return
	}
	
	// Return updated file listing
	h.BrowseGameserverFiles(w, r)
}

func (h *Handlers) UploadGameserverFile(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	
	// Parse multipart form with 10MB limit
	if err := r.ParseMultipartForm(10 << 20); err != nil {
		HandleError(w, BadRequest("Invalid upload format"), "upload_file")
		return
	}
	
	// Get the destination path
	destPath := r.FormValue("path")
	if destPath == "" {
		destPath = "/data/server"
	}
	destPath = sanitizePath(destPath)
	
	// Get the uploaded file
	file, header, err := r.FormFile("file")
	if err != nil {
		HandleError(w, BadRequest("No file provided"), "upload_file")
		return
	}
	defer file.Close()
	
	// Validate file size (100MB limit)
	if header.Size > 100<<20 {
		HandleError(w, BadRequest("File too large (max 100MB)"), "upload_file")
		return
	}
	
	gameserver, ok := h.getGameserver(w, id)
	if !ok {
		return
	}
	
	// Create a tar archive for the file
	buf := new(bytes.Buffer)
	tw := tar.NewWriter(buf)
	
	// Add file to tar archive
	hdr := &tar.Header{
		Name: header.Filename,
		Mode: 0644,
		Size: header.Size,
	}
	
	if err := tw.WriteHeader(hdr); err != nil {
		HandleError(w, InternalError(err, "Failed to create archive"), "upload_file")
		return
	}
	
	if _, err := io.Copy(tw, file); err != nil {
		HandleError(w, InternalError(err, "Failed to write file"), "upload_file")
		return
	}
	
	if err := tw.Close(); err != nil {
		HandleError(w, InternalError(err, "Failed to close archive"), "upload_file")
		return
	}
	
	// Upload file to container
	if err := h.service.UploadFile(gameserver.ContainerID, destPath, bytes.NewReader(buf.Bytes())); err != nil {
		HandleError(w, InternalError(err, "Failed to upload file"), "upload_file")
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

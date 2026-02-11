package handlers

import (
	"bytes"
	"encoding/json"
	"fmt"
	"html/template"
	"net/http"
	"strconv"
	"strings"

	"0xkowalskidev/gameservers/database"
	"0xkowalskidev/gameservers/models"
	"github.com/0xkowalskidev/gameserverquery/protocol"
)

// QueryServiceInterface defines the interface for game server queries
type QueryServiceInterface interface {
	QueryGameserver(gameserver *models.Gameserver, game *models.Game) (*protocol.ServerInfo, error)
}

// Error handling functions - imported from main package
var (
	HandleError   func(w http.ResponseWriter, err error, context string)
	NotFound      func(resource string) error
	BadRequest    func(format string, args ...interface{}) error
	InternalError func(err error, message string) error
	ParseForm     func(r *http.Request) error
	RequireMethod func(r *http.Request, method string) error
)

// Utility functions - imported from main package
var (
	Render func(w http.ResponseWriter, r *http.Request, tmpl *template.Template, templateName string, data interface{})
)

// Handlers contains all HTTP handlers and their dependencies
type Handlers struct {
	service         *database.GameserverRepository
	docker          models.DockerManagerInterface
	tmpl            *template.Template
	maxFileEditSize int64
	maxUploadSize   int64
	queryService    QueryServiceInterface
}

// New creates a new handlers instance
func New(service *database.GameserverRepository, docker models.DockerManagerInterface, tmpl *template.Template, maxFileEditSize, maxUploadSize int64, queryService QueryServiceInterface) *Handlers {
	return &Handlers{
		service:         service,
		docker:          docker,
		tmpl:            tmpl,
		maxFileEditSize: maxFileEditSize,
		maxUploadSize:   maxUploadSize,
		queryService:    queryService,
	}
}

// Helper function to get gameserver with error handling
func (h *Handlers) getGameserver(w http.ResponseWriter, id string) (*models.Gameserver, bool) {
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

// renderGameserverPageOrPartial handles the common HTMX vs full page rendering pattern
func (h *Handlers) renderGameserverPageOrPartial(w http.ResponseWriter, r *http.Request, gameserver *models.Gameserver, currentPage, templateName string, data map[string]interface{}) {
	if data == nil {
		data = make(map[string]interface{})
	}
	data["Gameserver"] = gameserver

	if r.Header.Get("HX-Request") == "true" {
		if err := h.tmpl.ExecuteTemplate(w, templateName, data); err != nil {
			HandleError(w, InternalError(err, "Failed to render template"), "render_template")
		}
	} else {
		h.renderGameserverWithWrapper(w, r, gameserver, currentPage, templateName, data)
	}
}

// GameserverFormData represents parsed gameserver form data
type GameserverFormData struct {
	Name        string
	GameID      string
	MemoryMB    int
	CPUCores    float64
	MaxBackups  int
	Environment []string
}

// parseGameserverForm parses and validates gameserver form data
func (h *Handlers) parseGameserverForm(r *http.Request) (*GameserverFormData, error) {
	if err := ParseForm(r); err != nil {
		return nil, err
	}

	name := strings.TrimSpace(r.FormValue("name"))
	gameID := strings.TrimSpace(r.FormValue("game_id"))
	if name == "" || gameID == "" {
		return nil, BadRequest("name and game_id are required")
	}

	memoryGB, _ := strconv.ParseFloat(r.FormValue("memory_gb"), 64)
	cpuCores, _ := strconv.ParseFloat(r.FormValue("cpu_cores"), 64)
	maxBackups, _ := strconv.Atoi(r.FormValue("max_backups"))

	memoryMB := int(memoryGB * 1024)
	if memoryMB <= 0 {
		memoryMB = 1024
	}
	if maxBackups <= 0 {
		maxBackups = 7
	}

	// Parse environment variables
	var validEnv []string
	for _, line := range strings.Split(r.FormValue("environment"), "\n") {
		line = strings.TrimSpace(line)
		if line != "" && strings.Contains(line, "=") {
			validEnv = append(validEnv, line)
		}
	}

	return &GameserverFormData{
		Name: name, GameID: gameID, MemoryMB: memoryMB,
		CPUCores: cpuCores, MaxBackups: maxBackups, Environment: validEnv,
	}, nil
}

// parseScheduledTaskForm parses and validates scheduled task form data
func (h *Handlers) parseScheduledTaskForm(r *http.Request, gameserverID string) (*models.ScheduledTask, error) {
	if err := ParseForm(r); err != nil {
		return nil, err
	}

	name := strings.TrimSpace(r.FormValue("name"))
	taskType := strings.TrimSpace(r.FormValue("type"))
	cronSchedule := strings.TrimSpace(r.FormValue("cron_schedule"))

	if name == "" || taskType == "" || cronSchedule == "" {
		return nil, BadRequest("name, type and cron_schedule are required")
	}

	parsedType := models.TaskType(taskType)
	if parsedType != models.TaskTypeRestart && parsedType != models.TaskTypeBackup {
		return nil, BadRequest("invalid task type: %s", taskType)
	}

	return &models.ScheduledTask{
		GameserverID: gameserverID, Name: name, Type: parsedType,
		Status: models.TaskStatusActive, CronSchedule: cronSchedule,
	}, nil
}

// updateTaskFromForm updates task from form data
func (h *Handlers) updateTaskFromForm(task *models.ScheduledTask, r *http.Request) error {
	if err := ParseForm(r); err != nil {
		return err
	}

	name := strings.TrimSpace(r.FormValue("name"))
	taskType := strings.TrimSpace(r.FormValue("type"))
	status := strings.TrimSpace(r.FormValue("status"))
	cronSchedule := strings.TrimSpace(r.FormValue("cron_schedule"))

	if taskType != "" {
		parsedType := models.TaskType(taskType)
		if parsedType != models.TaskTypeRestart && parsedType != models.TaskTypeBackup {
			return BadRequest("invalid task type: %s", taskType)
		}
		task.Type = parsedType
	}

	if status != "" {
		parsedStatus := models.TaskStatus(status)
		if parsedStatus != models.TaskStatusActive && parsedStatus != models.TaskStatusDisabled {
			return BadRequest("invalid task status: %s", status)
		}
		task.Status = parsedStatus
	}

	if name != "" {
		task.Name = name
	}
	if cronSchedule != "" {
		task.CronSchedule = cronSchedule
	}
	return nil
}

// requireQueryParam validates required query parameter
func (h *Handlers) requireQueryParam(r *http.Request, param string) (string, error) {
	if value := r.URL.Query().Get(param); value != "" {
		return value, nil
	}
	return "", BadRequest("%s parameter required", param)
}

// validateFormFields validates required form fields
func (h *Handlers) validateFormFields(r *http.Request, fields ...string) error {
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

// renderGameserverWithWrapper renders a gameserver page with wrapper (for full page loads)
func (h *Handlers) renderGameserverWithWrapper(w http.ResponseWriter, r *http.Request, gameserver *models.Gameserver, currentPage string, templateName string, data map[string]interface{}) {
	// Set up page data with gameserver context
	pageData := map[string]interface{}{
		"Gameserver":  gameserver,
		"CurrentPage": currentPage,
	}

	// Merge any additional data
	for k, v := range data {
		pageData[k] = v
	}

	// Render the content template
	var contentBuf bytes.Buffer
	if err := h.tmpl.ExecuteTemplate(&contentBuf, templateName, pageData); err != nil {
		HandleError(w, err, "render_content_template")
		return
	}

	// Create wrapper data with the rendered content
	wrapperData := map[string]interface{}{
		"Gameserver":  gameserver,
		"CurrentPage": currentPage,
		"Content":     template.HTML(contentBuf.String()),
	}

	// Render the wrapper (which uses Render for layout if full page)
	if r.Header.Get("HX-Request") == "true" {
		if err := h.tmpl.ExecuteTemplate(w, "gameserver-wrapper.html", wrapperData); err != nil {
			HandleError(w, err, "render_wrapper_template")
		}
	} else {
		Render(w, r, h.tmpl, "gameserver-wrapper.html", wrapperData)
	}
}

// formatFileSize formats file size in human readable format
func formatFileSize(size int64) string {
	const unit = 1024
	if size < unit {
		return fmt.Sprintf("%d B", size)
	}
	div, exp := int64(unit), 0
	for n := size / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(size)/float64(div), "KMGTPE"[exp])
}

// JSON response helpers
func (h *Handlers) jsonError(w http.ResponseWriter, message string) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"Supported": false, "Error": message})
}

func (h *Handlers) jsonSuccess(w http.ResponseWriter, data map[string]interface{}) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(data)
}

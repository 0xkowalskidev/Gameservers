package handlers

import (
	"bytes"
	"encoding/json"
	"fmt"
	"html/template"
	"net/http"
	"strconv"
	"strings"
	"time"

	"0xkowalskidev/gameservers/models"
)

// Error handling functions - imported from main package
var (
	HandleError   func(w http.ResponseWriter, err error, context string)
	NotFound      func(resource string) error
	BadRequest    func(format string, args ...interface{}) error
	InternalError func(err error, message string) error
	ParseForm     func(r *http.Request) error
	RequireMethod func(r *http.Request, method string) error
	LogAndRespond func(w http.ResponseWriter, status int, message string, args ...interface{})
)

// Utility functions - imported from main package
var (
	Render func(w http.ResponseWriter, r *http.Request, tmpl *template.Template, templateName string, data interface{})
)

// BaseHandler provides common functionality for all handlers
type BaseHandler struct {
	service         models.GameserverServiceInterface
	tmpl            *template.Template
	maxFileEditSize int64
	maxUploadSize   int64
}

// NewBaseHandler creates a new base handler
func NewBaseHandler(service models.GameserverServiceInterface, tmpl *template.Template, maxFileEditSize, maxUploadSize int64) *BaseHandler {
	return &BaseHandler{
		service:         service,
		tmpl:            tmpl,
		maxFileEditSize: maxFileEditSize,
		maxUploadSize:   maxUploadSize,
	}
}

// Handlers embeds BaseHandler for backward compatibility
type Handlers struct {
	*BaseHandler
}

func New(service models.GameserverServiceInterface, tmpl *template.Template, maxFileEditSize, maxUploadSize int64) *Handlers {
	return &Handlers{BaseHandler: NewBaseHandler(service, tmpl, maxFileEditSize, maxUploadSize)}
}

// Helper function to get gameserver with error handling
func (h *BaseHandler) getGameserver(w http.ResponseWriter, id string) (*models.Gameserver, bool) {
	gameserver, err := h.service.GetGameserver(id)
	if err != nil {
		HandleError(w, NotFound("Gameserver"), "get_gameserver")
		return nil, false
	}
	return gameserver, true
}

// Helper function to handle redirects with HTMX
func (h *BaseHandler) htmxRedirect(w http.ResponseWriter, url string) {
	w.Header().Set("HX-Redirect", url)
	w.WriteHeader(http.StatusOK)
}

// renderGameserverPageOrPartial handles the common HTMX vs full page rendering pattern
func (h *BaseHandler) renderGameserverPageOrPartial(w http.ResponseWriter, r *http.Request, gameserver *models.Gameserver, currentPage, templateName string, data map[string]interface{}) {
	if data == nil {
		data = make(map[string]interface{})
	}
	data["Gameserver"] = gameserver

	if r.Header.Get("HX-Request") == "true" {
		if err := h.tmpl.ExecuteTemplate(w, templateName, data); err != nil {
			HandleError(w, InternalError(err, "Failed to render template"), "render_template")
		}
	} else {
		h.renderGameserverPage(w, r, gameserver, currentPage, templateName, data)
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
func (h *BaseHandler) parseGameserverForm(r *http.Request) (*GameserverFormData, error) {
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
func (h *BaseHandler) parseScheduledTaskForm(r *http.Request, gameserverID string) (*models.ScheduledTask, error) {
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
func (h *BaseHandler) updateTaskFromForm(task *models.ScheduledTask, r *http.Request) error {
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
func (h *BaseHandler) requireQueryParam(r *http.Request, param string) (string, error) {
	if value := r.URL.Query().Get(param); value != "" {
		return value, nil
	}
	return "", BadRequest("%s parameter required", param)
}

// validateFormFields validates required form fields
func (h *BaseHandler) validateFormFields(r *http.Request, fields ...string) error {
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

// renderGameserverPage is a helper that combines renderWithGameserverContext for the most common use case
func (h *BaseHandler) renderGameserverPage(w http.ResponseWriter, r *http.Request, gameserver *models.Gameserver, currentPage string, contentTemplate string, data map[string]interface{}) {
	h.renderWithGameserverContext(w, r, gameserver, currentPage, contentTemplate, data)
}

// renderWithGameserverContext handles the standard gameserver page layout with navigation
func (h *BaseHandler) renderWithGameserverContext(w http.ResponseWriter, r *http.Request, gameserver *models.Gameserver, currentPage string, templateName string, data map[string]interface{}) {
	// Set up page data with gameserver context
	pageData := map[string]interface{}{
		"Gameserver":  gameserver,
		"CurrentPage": currentPage,
	}

	// Merge any additional data
	for k, v := range data {
		pageData[k] = v
	}

	// Always render the layout for full page requests
	if r.Header.Get("HX-Request") != "true" {
		// For full page requests, we need to render the content template first,
		// then wrap it in the gameserver-wrapper, then in the layout
		var contentBuf bytes.Buffer
		err := h.tmpl.ExecuteTemplate(&contentBuf, templateName, pageData)
		if err != nil {
			HandleError(w, err, "render_content_template")
			return
		}

		// Create wrapper data with the rendered content
		wrapperData := map[string]interface{}{
			"Gameserver":  gameserver,
			"CurrentPage": currentPage,
			"Content":     template.HTML(contentBuf.String()),
		}

		// Use the Render function to wrap in layout
		Render(w, r, h.tmpl, "gameserver-wrapper.html", wrapperData)
		return
	}

	// For HTMX requests, render the gameserver-wrapper which includes navigation
	// First render the content template
	var contentBuf bytes.Buffer
	err := h.tmpl.ExecuteTemplate(&contentBuf, templateName, pageData)
	if err != nil {
		HandleError(w, err, "render_content_template")
		return
	}

	// Create wrapper data with the rendered content
	wrapperData := map[string]interface{}{
		"Gameserver":  gameserver,
		"CurrentPage": currentPage,
		"Content":     template.HTML(contentBuf.String()),
	}

	// Render the wrapper template
	err = h.tmpl.ExecuteTemplate(w, "gameserver-wrapper.html", wrapperData)
	if err != nil {
		HandleError(w, err, "render_wrapper_template")
	}
}

// generateID generates a unique ID for entities
func generateID() string {
	return strconv.FormatInt(time.Now().UnixNano(), 36)
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
func (h *BaseHandler) jsonError(w http.ResponseWriter, message string) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"Supported": false, "Error": message})
}

func (h *BaseHandler) jsonSuccess(w http.ResponseWriter, data map[string]interface{}) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(data)
}

func (h *BaseHandler) jsonStatus(w http.ResponseWriter, status, message string) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": status, "message": message})
}

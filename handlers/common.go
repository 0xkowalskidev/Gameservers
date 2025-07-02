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
	queryService    QueryServiceInterface
}

// NewBaseHandler creates a new base handler
func NewBaseHandler(service models.GameserverServiceInterface, tmpl *template.Template, maxFileEditSize, maxUploadSize int64, queryService QueryServiceInterface) *BaseHandler {
	return &BaseHandler{
		service:         service,
		tmpl:            tmpl,
		maxFileEditSize: maxFileEditSize,
		maxUploadSize:   maxUploadSize,
		queryService:    queryService,
	}
}

// Handlers embeds BaseHandler for backward compatibility
type Handlers struct {
	*BaseHandler
}

func New(service models.GameserverServiceInterface, tmpl *template.Template, maxFileEditSize, maxUploadSize int64, queryService QueryServiceInterface) *Handlers {
	return &Handlers{BaseHandler: NewBaseHandler(service, tmpl, maxFileEditSize, maxUploadSize, queryService)}
}

// Error handling helpers
func (h *BaseHandler) handleDBError(w http.ResponseWriter, err error, context string) {
	if err != nil {
		HandleError(w, InternalError(err, "Database operation failed"), context)
	}
}

func (h *BaseHandler) handleServiceError(w http.ResponseWriter, err error, operation string) {
	if err != nil {
		HandleError(w, InternalError(err, "Service operation failed"), operation)
	}
}

func (h *BaseHandler) handleError(w http.ResponseWriter, err error, context, message string) {
	if err != nil {
		HandleError(w, InternalError(err, message), context)
	}
}

// requireGameserver gets a gameserver and handles error automatically, returns nil if error occurred
func (h *BaseHandler) requireGameserver(w http.ResponseWriter, id string) *models.Gameserver {
	gameserver, err := h.service.GetGameserver(id)
	if err != nil {
		HandleError(w, NotFound("Gameserver"), "get_gameserver")
		return nil
	}
	return gameserver
}


// Helper function to handle redirects with HTMX
func (h *BaseHandler) htmxRedirect(w http.ResponseWriter, url string) {
	w.Header().Set("HX-Redirect", url)
	w.WriteHeader(http.StatusOK)
}

// Form parsing helpers
func (h *BaseHandler) getFormValue(r *http.Request, key string, required bool) (string, error) {
	value := strings.TrimSpace(r.FormValue(key))
	if required && value == "" {
		return "", BadRequest("Field '%s' is required", key)
	}
	return value, nil
}

func (h *BaseHandler) parseIntForm(r *http.Request, key string, defaultVal int) (int, error) {
	value := strings.TrimSpace(r.FormValue(key))
	if value == "" {
		return defaultVal, nil
	}
	result, err := strconv.Atoi(value)
	if err != nil {
		return 0, BadRequest("Field '%s' must be a valid integer", key)
	}
	return result, nil
}

func (h *BaseHandler) parseFloatForm(r *http.Request, key string, defaultVal float64) (float64, error) {
	value := strings.TrimSpace(r.FormValue(key))
	if value == "" {
		return defaultVal, nil
	}
	result, err := strconv.ParseFloat(value, 64)
	if err != nil {
		return 0, BadRequest("Field '%s' must be a valid number", key)
	}
	return result, nil
}

// renderGameserverPageOrPartial handles the common HTMX vs full page rendering pattern
func (h *BaseHandler) renderGameserverPageOrPartial(w http.ResponseWriter, r *http.Request, gameserver *models.Gameserver, currentPage, templateName string, data map[string]interface{}) {
	if data == nil {
		data = make(map[string]interface{})
	}
	data["Gameserver"] = gameserver
	data["CurrentPage"] = currentPage

	if r.Header.Get("HX-Request") == "true" {
		// HTMX request - render template directly
		h.handleError(w, h.tmpl.ExecuteTemplate(w, templateName, data), "render_template", "Failed to render template")
	} else {
		// Full page request - render with wrapper
		h.renderContentWithWrapper(w, r, gameserver, currentPage, templateName, data)
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
	name, err := h.getFormValue(r, "name", true)
	if err != nil {
		return nil, err
	}
	gameID, err := h.getFormValue(r, "game_id", true)
	if err != nil {
		return nil, err
	}

	memoryGB, err := h.parseFloatForm(r, "memory_gb", 1.0)
	if err != nil {
		return nil, err
	}
	cpuCores, err := h.parseFloatForm(r, "cpu_cores", 0.0)
	if err != nil {
		return nil, err
	}
	maxBackups, err := h.parseIntForm(r, "max_backups", 7)
	if err != nil {
		return nil, err
	}

	memoryMB := int(memoryGB * 1024)
	if memoryMB <= 0 {
		memoryMB = 1024
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
	name, err := h.getFormValue(r, "name", true)
	if err != nil {
		return nil, err
	}
	taskType, err := h.getFormValue(r, "type", true)
	if err != nil {
		return nil, err
	}
	cronSchedule, err := h.getFormValue(r, "cron_schedule", true)
	if err != nil {
		return nil, err
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

	name, _ := h.getFormValue(r, "name", false)
	taskType, _ := h.getFormValue(r, "type", false)
	status, _ := h.getFormValue(r, "status", false)
	cronSchedule, _ := h.getFormValue(r, "cron_schedule", false)

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


// getRequiredFormValues gets multiple required form values with validation
func (h *BaseHandler) getRequiredFormValues(r *http.Request, fields ...string) (map[string]string, error) {
	if err := ParseForm(r); err != nil {
		return nil, err
	}
	result := make(map[string]string)
	for _, field := range fields {
		value := strings.TrimSpace(r.FormValue(field))
		if value == "" {
			return nil, BadRequest("%s is required", field)
		}
		result[field] = value
	}
	return result, nil
}

// validateFormFields validates required form fields (keeping for backward compatibility)
func (h *BaseHandler) validateFormFields(r *http.Request, fields ...string) error {
	_, err := h.getRequiredFormValues(r, fields...)
	return err
}

// renderContentWithWrapper renders content inside gameserver wrapper layout
func (h *BaseHandler) renderContentWithWrapper(w http.ResponseWriter, r *http.Request, gameserver *models.Gameserver, currentPage, templateName string, data map[string]interface{}) {
	// Render content template to buffer
	var contentBuf bytes.Buffer
	if err := h.tmpl.ExecuteTemplate(&contentBuf, templateName, data); err != nil {
		h.handleError(w, err, "render_content_template", "Failed to render content template")
		return
	}

	// Prepare wrapper data
	wrapperData := map[string]interface{}{
		"Gameserver":  gameserver,
		"CurrentPage": currentPage,
		"Content":     template.HTML(contentBuf.String()),
	}

	// Render with full layout
	Render(w, r, h.tmpl, "gameserver-wrapper.html", wrapperData)
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
func (h *BaseHandler) jsonResponse(w http.ResponseWriter, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(data)
}

func (h *BaseHandler) jsonError(w http.ResponseWriter, message string) {
	h.jsonResponse(w, map[string]interface{}{"Supported": false, "Error": message})
}

func (h *BaseHandler) jsonSuccess(w http.ResponseWriter, data map[string]interface{}) {
	h.jsonResponse(w, data)
}

func (h *BaseHandler) jsonStatus(w http.ResponseWriter, status, message string) {
	h.jsonResponse(w, map[string]string{"status": status, "message": message})
}

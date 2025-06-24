package handlers

import (
	"bytes"
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

// GameserverServiceInterface is now defined in models package

type Handlers struct {
	service models.GameserverServiceInterface
	tmpl    *template.Template
}

func New(service models.GameserverServiceInterface, tmpl *template.Template) *Handlers {
	return &Handlers{service: service, tmpl: tmpl}
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

// Helper function to parse gameserver form data
type GameserverFormData struct {
	Name        string
	GameID      string
	MemoryMB    int
	CPUCores    float64
	MaxBackups  int
	Environment []string
}

func parseGameserverForm(r *http.Request) (*GameserverFormData, error) {
	if err := ParseForm(r); err != nil {
		return nil, err
	}

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
		MemoryMB:    memoryMB,
		CPUCores:    cpuCores,
		MaxBackups:  maxBackups,
		Environment: validEnv,
	}, nil
}

// Helper function to parse scheduled task form data
func parseScheduledTaskForm(r *http.Request, gameserverID string) (*models.ScheduledTask, error) {
	if err := ParseForm(r); err != nil {
		return nil, err
	}

	return &models.ScheduledTask{
		GameserverID: gameserverID,
		Name:         r.FormValue("name"),
		Type:         models.TaskType(r.FormValue("type")),
		Status:       models.TaskStatusActive,
		CronSchedule: r.FormValue("cron_schedule"),
	}, nil
}

// Helper function to update task from form data
func updateTaskFromForm(task *models.ScheduledTask, r *http.Request) error {
	if err := ParseForm(r); err != nil {
		return err
	}

	task.Name = r.FormValue("name")
	task.Type = models.TaskType(r.FormValue("type"))
	task.Status = models.TaskStatus(r.FormValue("status"))
	task.CronSchedule = r.FormValue("cron_schedule")
	return nil
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

// renderGameserverPage is a helper that combines renderWithGameserverContext for the most common use case
func (h *Handlers) renderGameserverPage(w http.ResponseWriter, r *http.Request, gameserver *models.Gameserver, currentPage string, contentTemplate string, data map[string]interface{}) {
	h.renderWithGameserverContext(w, r, gameserver, currentPage, contentTemplate, data)
}

// renderWithGameserverContext handles the standard gameserver page layout with navigation
func (h *Handlers) renderWithGameserverContext(w http.ResponseWriter, r *http.Request, gameserver *models.Gameserver, currentPage string, templateName string, data map[string]interface{}) {
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


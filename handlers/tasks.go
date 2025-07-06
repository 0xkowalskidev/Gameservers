package handlers

import (
	"fmt"
	"net/http"
	"strings"

	. "0xkowalskidev/gameservers/errors"
	"github.com/go-chi/chi/v5"
	"github.com/rs/zerolog/log"

	"0xkowalskidev/gameservers/models"
	"0xkowalskidev/gameservers/services"
)

// TaskHandlers handles task-related HTTP requests
type TaskHandlers struct {
	*BaseHandlers
	gameserverService services.GameserverServiceInterface
	taskService       models.TaskServiceInterface
}

// NewTaskHandlers creates new task handlers
func NewTaskHandlers(base *BaseHandlers, gameserverService services.GameserverServiceInterface, taskService models.TaskServiceInterface) *TaskHandlers {
	return &TaskHandlers{
		BaseHandlers:      base,
		gameserverService: gameserverService,
		taskService:       taskService,
	}
}

// ListGameserverTasks displays all scheduled tasks for a gameserver
func (h *TaskHandlers) ListGameserverTasks(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	gameserver, err := h.gameserverService.GetGameserver(id)
	if err != nil {
		h.HandleError(w, r, NotFound("Gameserver"))
		return
	}

	tasks, err := h.taskService.ListScheduledTasksForGameserver(id)
	if err != nil {
		h.HandleError(w, r, err)
		return
	}

	data := map[string]interface{}{
		"Tasks":      tasks,
		"Gameserver": gameserver,
	}
	h.Render(w, r, "gameserver-tasks.html", data)
}

// NewGameserverTask shows the create task form
func (h *TaskHandlers) NewGameserverTask(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	gameserver, err := h.gameserverService.GetGameserver(id)
	if err != nil {
		h.HandleError(w, r, NotFound("Gameserver"))
		return
	}
	data := map[string]interface{}{
		"Gameserver": gameserver,
	}
	h.Render(w, r, "new-task.html", data)
}

// CreateGameserverTask creates a new scheduled task
func (h *TaskHandlers) CreateGameserverTask(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if err := ParseForm(r); err != nil {
		h.HandleError(w, r, err)
		return
	}

	task := &models.ScheduledTask{
		GameserverID: id,
		Name:         strings.TrimSpace(r.FormValue("name")),
		Type:         models.TaskType(r.FormValue("type")),
		Status:       models.TaskStatusActive,
		CronSchedule: strings.TrimSpace(r.FormValue("cron_schedule")),
	}

	if task.Name == "" || task.CronSchedule == "" {
		h.HandleError(w, r, BadRequest("name and cron_schedule are required"))
		return
	}

	log.Info().Str("gameserver_id", id).Str("task_name", task.Name).Str("type", string(task.Type)).Str("cron", task.CronSchedule).Msg("Creating scheduled task")

	if err := h.taskService.CreateScheduledTask(task); err != nil {
		h.HandleError(w, r, err)
		return
	}
	w.Header().Set("HX-Redirect", fmt.Sprintf("/%s/tasks", id))
	w.WriteHeader(http.StatusOK)
}

// EditGameserverTask shows the edit task form
func (h *TaskHandlers) EditGameserverTask(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	taskID := chi.URLParam(r, "taskId")

	gameserver, err := h.gameserverService.GetGameserver(id)
	if err != nil {
		h.HandleError(w, r, NotFound("Gameserver"))
		return
	}

	task, err := h.taskService.GetScheduledTask(taskID)
	if err != nil {
		h.HandleError(w, r, NotFound("Task"))
		return
	}

	data := map[string]interface{}{
		"Task":       task,
		"Gameserver": gameserver,
	}
	h.Render(w, r, "edit-task.html", data)
}

// UpdateGameserverTask updates an existing scheduled task
func (h *TaskHandlers) UpdateGameserverTask(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	taskID := chi.URLParam(r, "taskId")

	task, err := h.taskService.GetScheduledTask(taskID)
	if err != nil {
		h.HandleError(w, r, NotFound("Task"))
		return
	}

	if err := ParseForm(r); err != nil {
		h.HandleError(w, r, err)
		return
	}

	name := strings.TrimSpace(r.FormValue("name"))
	taskType := r.FormValue("type")
	status := r.FormValue("status")
	cronSchedule := strings.TrimSpace(r.FormValue("cron_schedule"))

	if name != "" {
		task.Name = name
	}
	if taskType != "" {
		task.Type = models.TaskType(taskType)
	}
	if status != "" {
		task.Status = models.TaskStatus(status)
	}
	if cronSchedule != "" {
		task.CronSchedule = cronSchedule
	}

	log.Info().Str("task_id", taskID).Str("task_name", task.Name).Msg("Updating scheduled task")

	if err := h.taskService.UpdateScheduledTask(task); err != nil {
		h.HandleError(w, r, err)
		return
	}

	w.Header().Set("HX-Redirect", fmt.Sprintf("/%s/tasks", id))
	w.WriteHeader(http.StatusOK)
}

// DeleteGameserverTask deletes a scheduled task
func (h *TaskHandlers) DeleteGameserverTask(w http.ResponseWriter, r *http.Request) {
	taskID := chi.URLParam(r, "taskId")

	if err := h.taskService.DeleteScheduledTask(taskID); err != nil {
		h.HandleError(w, r, err)
		return
	}

	w.WriteHeader(http.StatusOK)
}

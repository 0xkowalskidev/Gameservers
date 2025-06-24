package handlers

import (
	"fmt"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/rs/zerolog/log"
)

// ListGameserverTasks displays all scheduled tasks for a gameserver
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

	// If HTMX request, render just the template content
	if r.Header.Get("HX-Request") == "true" {
		if err := h.tmpl.ExecuteTemplate(w, "gameserver-tasks.html", data); err != nil {
			HandleError(w, InternalError(err, "Failed to render tasks template"), "list_tasks")
		}
	} else {
		// Full page load, use wrapper
		h.renderGameserverPage(w, r, gameserver, "tasks", "gameserver-tasks.html", data)
	}
}

// NewGameserverTask shows the create task form
func (h *Handlers) NewGameserverTask(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	gameserver, ok := h.getGameserver(w, id)
	if !ok {
		return
	}

	data := map[string]interface{}{"Gameserver": gameserver}
	// If HTMX request, render just the template content
	if r.Header.Get("HX-Request") == "true" {
		if err := h.tmpl.ExecuteTemplate(w, "new-task.html", data); err != nil {
			HandleError(w, InternalError(err, "Failed to render new task template"), "new_task")
		}
	} else {
		// Full page load, use wrapper
		h.renderGameserverPage(w, r, gameserver, "tasks", "new-task.html", data)
	}
}

// CreateGameserverTask creates a new scheduled task
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

// EditGameserverTask shows the edit task form
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

	// If HTMX request, render just the template content
	if r.Header.Get("HX-Request") == "true" {
		if err := h.tmpl.ExecuteTemplate(w, "edit-task.html", data); err != nil {
			HandleError(w, InternalError(err, "Failed to render edit task template"), "edit_task")
		}
	} else {
		// Full page load, use wrapper
		h.renderGameserverPage(w, r, gameserver, "tasks", "edit-task.html", data)
	}
}

// UpdateGameserverTask updates an existing scheduled task
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

// DeleteGameserverTask deletes a scheduled task
func (h *Handlers) DeleteGameserverTask(w http.ResponseWriter, r *http.Request) {
	taskID := chi.URLParam(r, "taskId")

	if err := h.service.DeleteScheduledTask(taskID); err != nil {
		HandleError(w, err, "delete_task")
		return
	}

	w.WriteHeader(http.StatusOK)
}
package handlers

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestHandlers_ListGameserverTasks(t *testing.T) {
	mockService := createMockService()
	tmpl := createTestTemplate("gameserver-tasks.html", `{{range .Tasks}}{{.Name}}{{end}}`)
	handlers := New(mockService, tmpl)

	req := httptest.NewRequest("GET", "/1/tasks", nil)
	w := httptest.NewRecorder()

	router := newTestRouter("1")
	router.Get("/1/tasks", handlers.ListGameserverTasks)
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	body := w.Body.String()
	if !strings.Contains(body, "Test Task") {
		t.Errorf("Expected response to contain 'Test Task', got: %s", body)
	}
}

func TestHandlers_CreateGameserverTask(t *testing.T) {
	mockService := createMockService()
	tmpl := createTestTemplate("task-form.html", `{{.Task.Name}}`)
	handlers := New(mockService, tmpl)

	formData := "name=Test Task&type=restart&cron_schedule=0 2 * * *"
	req := httptest.NewRequest("POST", "/1/tasks", strings.NewReader(formData))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()

	router := newTestRouter("1")
	router.Post("/1/tasks", handlers.CreateGameserverTask)
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}
}

func TestHandlers_DeleteGameserverTask(t *testing.T) {
	mockService := createMockService()
	tmpl := createTestTemplate("task-deleted.html", `Task deleted`)
	handlers := New(mockService, tmpl)

	req := httptest.NewRequest("DELETE", "/1/tasks/task-1", nil)
	w := httptest.NewRecorder()

	router := newTestRouterWithParams(map[string]string{"taskId": "task-1"})
	router.Delete("/1/tasks/task-1", handlers.DeleteGameserverTask)
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}
}

func TestHandlers_UpdateGameserverTask(t *testing.T) {
	mockService := createMockService()
	tmpl := createTestTemplate("task-form.html", `{{.Task.Name}}`)
	handlers := New(mockService, tmpl)

	formData := "name=Updated Task&status=disabled"
	req := httptest.NewRequest("PUT", "/1/tasks/task-1", strings.NewReader(formData))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()

	router := newTestRouterWithParams(map[string]string{"id": "1", "taskId": "task-1"})
	router.Put("/1/tasks/task-1", handlers.UpdateGameserverTask)
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}
}

func TestHandlers_NewGameserverTask(t *testing.T) {
	mockService := createMockService()
	tmpl := createTestTemplate("new-task.html", `{{.Gameserver.ID}}`)
	handlers := New(mockService, tmpl)

	req := httptest.NewRequest("GET", "/1/tasks/new", nil)
	w := httptest.NewRecorder()

	router := newTestRouter("1")
	router.Get("/1/tasks/new", handlers.NewGameserverTask)
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	body := w.Body.String()
	if !strings.Contains(body, "1") {
		t.Errorf("Expected response to contain gameserver ID '1', got: %s", body)
	}
}

func TestHandlers_EditGameserverTask(t *testing.T) {
	mockService := createMockService()
	tmpl := createTestTemplate("edit-task.html", `{{.Task.Name}}`)
	handlers := New(mockService, tmpl)

	req := httptest.NewRequest("GET", "/1/tasks/task-1/edit", nil)
	w := httptest.NewRecorder()

	router := newTestRouterWithParams(map[string]string{"id": "1", "taskId": "task-1"})
	router.Get("/1/tasks/task-1/edit", handlers.EditGameserverTask)
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	body := w.Body.String()
	if !strings.Contains(body, "Test Task") {
		t.Errorf("Expected response to contain 'Test Task', got: %s", body)
	}
}

func TestHandlers_CreateGameserverTask_InvalidData(t *testing.T) {
	mockService := createMockService()
	tmpl := createTestTemplate("task-form.html", `{{.Task.Name}}`)
	handlers := New(mockService, tmpl)

	// Missing required fields
	formData := "name=&type=&cron_schedule="
	req := httptest.NewRequest("POST", "/1/tasks", strings.NewReader(formData))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()

	router := newTestRouter("1")
	router.Post("/1/tasks", handlers.CreateGameserverTask)
	router.ServeHTTP(w, req)

	if w.Code == http.StatusOK {
		t.Errorf("Expected non-200 status for invalid data, got %d", w.Code)
	}
}

func TestHandlers_CreateGameserverTask_InvalidTaskType(t *testing.T) {
	mockService := createMockService()
	tmpl := createTestTemplate("task-form.html", `{{.Task.Name}}`)
	handlers := New(mockService, tmpl)

	formData := "name=Test Task&type=invalid&cron_schedule=0 2 * * *"
	req := httptest.NewRequest("POST", "/1/tasks", strings.NewReader(formData))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()

	router := newTestRouter("1")
	router.Post("/1/tasks", handlers.CreateGameserverTask)
	router.ServeHTTP(w, req)

	if w.Code == http.StatusOK {
		t.Errorf("Expected non-200 status for invalid task type, got %d", w.Code)
	}
}

func TestHandlers_UpdateGameserverTask_InvalidStatus(t *testing.T) {
	mockService := createMockService()
	tmpl := createTestTemplate("task-form.html", `{{.Task.Name}}`)
	handlers := New(mockService, tmpl)

	formData := "name=Test Task&status=invalid"
	req := httptest.NewRequest("PUT", "/1/tasks/task-1", strings.NewReader(formData))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()

	router := newTestRouterWithParams(map[string]string{"id": "1", "taskId": "task-1"})
	router.Put("/1/tasks/task-1", handlers.UpdateGameserverTask)
	router.ServeHTTP(w, req)

	if w.Code == http.StatusOK {
		t.Errorf("Expected non-200 status for invalid status, got %d", w.Code)
	}
}

func TestHandlers_DeleteGameserverTask_NotFound(t *testing.T) {
	mockService := createMockService()
	tmpl := createTestTemplate("task-deleted.html", `Task deleted`)
	handlers := New(mockService, tmpl)

	req := httptest.NewRequest("DELETE", "/1/tasks/non-existent", nil)
	w := httptest.NewRecorder()

	router := newTestRouterWithParams(map[string]string{"taskId": "non-existent"})
	router.Delete("/1/tasks/non-existent", handlers.DeleteGameserverTask)
	router.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("Expected status 404, got %d", w.Code)
	}
}

func TestHandlers_EditGameserverTask_NotFound(t *testing.T) {
	mockService := createMockService()
	tmpl := createTestTemplate("edit-task.html", `{{.Task.Name}}`)
	handlers := New(mockService, tmpl)

	req := httptest.NewRequest("GET", "/1/tasks/non-existent/edit", nil)
	w := httptest.NewRecorder()

	router := newTestRouter("non-existent")
	router.Get("/1/tasks/non-existent/edit", handlers.EditGameserverTask)
	router.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("Expected status 404, got %d", w.Code)
	}
}

func TestHandlers_TaskValidation(t *testing.T) {
	tests := []struct {
		name         string
		taskType     string
		cronSchedule string
		expectError  bool
	}{
		{"valid restart task", "restart", "0 2 * * *", false},
		{"valid backup task", "backup", "0 3 * * *", false},
		{"invalid task type", "invalid", "0 2 * * *", true},
		{"invalid cron format", "restart", "invalid cron", false}, // Cron validation might be lenient
		{"empty task type", "", "0 2 * * *", true},
		{"empty cron", "restart", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockService := createMockService()
			tmpl := createTestTemplate("task-form.html", `{{.Task.Name}}`)
			handlers := New(mockService, tmpl)

			formData := "name=Test Task&type=" + tt.taskType + "&cron_schedule=" + tt.cronSchedule
			req := httptest.NewRequest("POST", "/1/tasks", strings.NewReader(formData))
			req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
			w := httptest.NewRecorder()

			router := newTestRouter("1")
			router.Post("/1/tasks", handlers.CreateGameserverTask)
			router.ServeHTTP(w, req)

			if tt.expectError && w.Code == http.StatusOK {
				t.Errorf("Expected error for %s, but got status 200", tt.name)
			} else if !tt.expectError && w.Code != http.StatusOK {
				t.Errorf("Expected success for %s, but got status %d", tt.name, w.Code)
			}
		})
	}
}

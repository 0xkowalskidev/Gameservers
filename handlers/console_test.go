package handlers

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"0xkowalskidev/gameservers/services"
)

func TestHandlers_GameserverConsole(t *testing.T) {
	mockService := createMockService()
	tmpl := createTestTemplate("gameserver-console.html", `Console for {{.Gameserver.Name}}`)
	handlers := New(mockService, tmpl, 1024*1024, 10*1024*1024)

	req := httptest.NewRequest("GET", "/1/console", nil)
	w := httptest.NewRecorder()

	router := newTestRouter("1")
	router.Get("/1/console", handlers.GameserverConsole)
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	body := w.Body.String()
	if !strings.Contains(body, "Console for Test Server") {
		t.Errorf("Expected response to contain 'Console for Test Server', got: %s", body)
	}
}

func TestHandlers_GameserverConsole_HTMX(t *testing.T) {
	mockService := createMockService()
	tmpl := createTestTemplate("gameserver-console.html", `Console for {{.Gameserver.Name}}`)
	handlers := New(mockService, tmpl, 1024*1024, 10*1024*1024)

	req := httptest.NewRequest("GET", "/1/console", nil)
	req.Header.Set("HX-Request", "true")
	w := httptest.NewRecorder()

	router := newTestRouter("1")
	router.Get("/1/console", handlers.GameserverConsole)
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	body := w.Body.String()
	if !strings.Contains(body, "Console for Test Server") {
		t.Errorf("Expected response to contain 'Console for Test Server', got: %s", body)
	}
}

func TestHandlers_GameserverConsole_NotFound(t *testing.T) {
	mockService := createMockService()
	tmpl := createTestTemplate("gameserver-console.html", `Console for {{.Gameserver.Name}}`)
	handlers := New(mockService, tmpl, 1024*1024, 10*1024*1024)

	req := httptest.NewRequest("GET", "/999/console", nil)
	w := httptest.NewRecorder()

	router := newTestRouter("999")
	router.Get("/999/console", handlers.GameserverConsole)
	router.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("Expected status 404, got %d", w.Code)
	}
}

func TestHandlers_SendGameserverCommand(t *testing.T) {
	mockService := createMockService()
	tmpl := createTestTemplate("gameserver-console.html", `Console`)
	handlers := New(mockService, tmpl, 1024*1024, 10*1024*1024)

	formData := "command=say Hello World"
	req := httptest.NewRequest("POST", "/1/console/command", strings.NewReader(formData))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()

	router := newTestRouter("1")
	router.Post("/1/console/command", handlers.SendGameserverCommand)
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}
}

func TestHandlers_SendGameserverCommand_MissingCommand(t *testing.T) {
	mockService := createMockService()
	tmpl := createTestTemplate("gameserver-console.html", `Console`)
	handlers := New(mockService, tmpl, 1024*1024, 10*1024*1024)

	// Missing command field
	formData := "other_field=value"
	req := httptest.NewRequest("POST", "/1/console/command", strings.NewReader(formData))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()

	router := newTestRouter("1")
	router.Post("/1/console/command", handlers.SendGameserverCommand)
	router.ServeHTTP(w, req)

	if w.Code == http.StatusOK {
		t.Errorf("Expected non-200 status for missing command, got %d", w.Code)
	}
}

func TestHandlers_SendGameserverCommand_ServiceError(t *testing.T) {
	// Create a mock service that returns an error for SendGameserverCommand
	mockService := &mockGameserverServiceWithErrors{
		mockGameserverService: createMockService(),
		sendCommandError:      &services.HTTPError{Status: 500, Message: "Command failed"},
	}
	tmpl := createTestTemplate("gameserver-console.html", `Console`)
	handlers := New(mockService, tmpl, 1024*1024, 10*1024*1024)

	formData := "command=invalid command"
	req := httptest.NewRequest("POST", "/1/console/command", strings.NewReader(formData))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()

	router := newTestRouter("1")
	router.Post("/1/console/command", handlers.SendGameserverCommand)
	router.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("Expected status 500, got %d", w.Code)
	}
}

func TestHandlers_GameserverLogs(t *testing.T) {
	mockService := createMockService()
	tmpl := createTestTemplate("gameserver-console.html", `Console`)
	handlers := New(mockService, tmpl, 1024*1024, 10*1024*1024)

	req := httptest.NewRequest("GET", "/1/logs", nil)
	w := httptest.NewRecorder()

	router := newTestRouter("1")
	router.Get("/1/logs", handlers.GameserverLogs)
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	// Check SSE headers
	if w.Header().Get("Content-Type") != "text/event-stream" {
		t.Errorf("Expected Content-Type text/event-stream, got %s", w.Header().Get("Content-Type"))
	}
	if w.Header().Get("Cache-Control") != "no-cache" {
		t.Errorf("Expected Cache-Control no-cache, got %s", w.Header().Get("Cache-Control"))
	}
}

func TestHandlers_GameserverLogs_ServiceError(t *testing.T) {
	// Create a mock service that returns an error for StreamGameserverLogs
	mockService := &mockGameserverServiceWithErrors{
		mockGameserverService: createMockService(),
		streamLogsError:       &services.HTTPError{Status: 500, Message: "Stream failed"},
	}
	tmpl := createTestTemplate("gameserver-console.html", `Console`)
	handlers := New(mockService, tmpl, 1024*1024, 10*1024*1024)

	req := httptest.NewRequest("GET", "/1/logs", nil)
	w := httptest.NewRecorder()

	router := newTestRouter("1")
	router.Get("/1/logs", handlers.GameserverLogs)
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200 even with stream error, got %d", w.Code)
	}

	body := w.Body.String()
	if !strings.Contains(body, "event: error") {
		t.Errorf("Expected error event in response, got: %s", body)
	}
}

func TestHandlers_GameserverStats(t *testing.T) {
	mockService := createMockService()
	tmpl := createTestTemplate("gameserver-console.html", `Console`)
	handlers := New(mockService, tmpl, 1024*1024, 10*1024*1024)

	req := httptest.NewRequest("GET", "/1/stats", nil)
	w := httptest.NewRecorder()

	router := newTestRouter("1")
	router.Get("/1/stats", handlers.GameserverStats)
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	// Check SSE headers
	if w.Header().Get("Content-Type") != "text/event-stream" {
		t.Errorf("Expected Content-Type text/event-stream, got %s", w.Header().Get("Content-Type"))
	}
	if w.Header().Get("Cache-Control") != "no-cache" {
		t.Errorf("Expected Cache-Control no-cache, got %s", w.Header().Get("Cache-Control"))
	}
}

func TestHandlers_GameserverStats_ServiceError(t *testing.T) {
	// Create a mock service that returns an error for StreamGameserverStats
	mockService := &mockGameserverServiceWithErrors{
		mockGameserverService: createMockService(),
		streamStatsError:      &services.HTTPError{Status: 500, Message: "Stats failed"},
	}
	tmpl := createTestTemplate("gameserver-console.html", `Console`)
	handlers := New(mockService, tmpl, 1024*1024, 10*1024*1024)

	req := httptest.NewRequest("GET", "/1/stats", nil)
	w := httptest.NewRecorder()

	router := newTestRouter("1")
	router.Get("/1/stats", handlers.GameserverStats)
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200 even with stream error, got %d", w.Code)
	}

	body := w.Body.String()
	if !strings.Contains(body, "event: error") {
		t.Errorf("Expected error event in response, got: %s", body)
	}
}

// Mock service with error injection for testing error scenarios
type mockGameserverServiceWithErrors struct {
	*mockGameserverService
	sendCommandError error
	streamLogsError  error
	streamStatsError error
}

func (m *mockGameserverServiceWithErrors) SendGameserverCommand(id string, command string) error {
	if m.sendCommandError != nil {
		return m.sendCommandError
	}
	return m.mockGameserverService.SendGameserverCommand(id, command)
}

func (m *mockGameserverServiceWithErrors) StreamGameserverLogs(id string) (io.ReadCloser, error) {
	if m.streamLogsError != nil {
		return nil, m.streamLogsError
	}
	return m.mockGameserverService.StreamGameserverLogs(id)
}

func (m *mockGameserverServiceWithErrors) StreamGameserverStats(id string) (io.ReadCloser, error) {
	if m.streamStatsError != nil {
		return nil, m.streamStatsError
	}
	return m.mockGameserverService.StreamGameserverStats(id)
}

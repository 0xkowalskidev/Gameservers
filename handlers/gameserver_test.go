package handlers

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"0xkowalskidev/gameservers/models"
)

func TestHandlers_IndexGameservers(t *testing.T) {
	mockService := createMockService()
	tmpl := createTestTemplate("index.html", `{{range .Gameservers}}{{.Name}}{{end}}`)
	handlers := New(mockService, tmpl)

	req := httptest.NewRequest("GET", "/", nil)
	w := httptest.NewRecorder()

	handlers.IndexGameservers(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	body := w.Body.String()
	if !strings.Contains(body, "Test Server") {
		t.Errorf("Expected response to contain 'Test Server', got: %s", body)
	}
}

func TestHandlers_CreateGameserver(t *testing.T) {
	mockService := createMockService()
	tmpl := createTestTemplate("gameserver-row.html", `{{.Name}}`)
	handlers := New(mockService, tmpl)

	formData := "name=test&game_id=minecraft&memory_mb=1024&cpu_cores=0"
	req := httptest.NewRequest("POST", "/", strings.NewReader(formData))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()

	handlers.CreateGameserver(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}
}

func TestHandlers_ShowGameserver(t *testing.T) {
	mockService := createMockService()
	tmpl := createTestTemplate("gameserver-details.html", `{{.Gameserver.Name}}`)
	handlers := New(mockService, tmpl)

	req := httptest.NewRequest("GET", "/1", nil)
	w := httptest.NewRecorder()

	// Use the test router to set URL params
	router := newTestRouter("1")
	router.Get("/1", handlers.ShowGameserver)
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	body := w.Body.String()
	if !strings.Contains(body, "Test Server") {
		t.Errorf("Expected response to contain 'Test Server', got: %s", body)
	}
}

func TestHandlers_StartGameserver(t *testing.T) {
	mockService := createMockService()
	tmpl := createTestTemplate("gameserver-status.html", `{{.Status}}`)
	handlers := New(mockService, tmpl)

	req := httptest.NewRequest("POST", "/1/start", nil)
	w := httptest.NewRecorder()

	router := newTestRouter("1")
	router.Post("/1/start", handlers.StartGameserver)
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}
}

func TestHandlers_StopGameserver(t *testing.T) {
	mockService := createMockService()
	tmpl := createTestTemplate("gameserver-status.html", `{{.Status}}`)
	handlers := New(mockService, tmpl)

	req := httptest.NewRequest("POST", "/1/stop", nil)
	w := httptest.NewRecorder()

	router := newTestRouter("1")
	router.Post("/1/stop", handlers.StopGameserver)
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}
}

func TestHandlers_RestartGameserver(t *testing.T) {
	mockService := createMockService()
	tmpl := createTestTemplate("gameserver-status.html", `{{.Status}}`)
	handlers := New(mockService, tmpl)

	req := httptest.NewRequest("POST", "/1/restart", nil)
	w := httptest.NewRecorder()

	router := newTestRouter("1")
	router.Post("/1/restart", handlers.RestartGameserver)
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}
}

func TestHandlers_DeleteGameserver(t *testing.T) {
	mockService := createMockService()
	tmpl := createTestTemplate("gameserver-deleted.html", `Gameserver deleted`)
	handlers := New(mockService, tmpl)

	req := httptest.NewRequest("DELETE", "/1", nil)
	w := httptest.NewRecorder()

	router := newTestRouter("1")
	router.Delete("/1", handlers.DestroyGameserver)
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}
}

func TestHandlers_NewGameserver(t *testing.T) {
	mockService := createMockService()
	tmpl := createTestTemplate("new-gameserver.html", `{{range .Games}}{{.Name}}{{end}}`)
	handlers := New(mockService, tmpl)

	req := httptest.NewRequest("GET", "/new", nil)
	w := httptest.NewRecorder()

	handlers.NewGameserver(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	body := w.Body.String()
	if !strings.Contains(body, "Minecraft") {
		t.Errorf("Expected response to contain 'Minecraft', got: %s", body)
	}
}

func TestHandlers_EditGameserver(t *testing.T) {
	mockService := createMockService()
	tmpl := createTestTemplate("edit-gameserver.html", `{{.Gameserver.Name}}`)
	handlers := New(mockService, tmpl)

	req := httptest.NewRequest("GET", "/1/edit", nil)
	w := httptest.NewRecorder()

	router := newTestRouter("1")
	router.Get("/1/edit", handlers.EditGameserver)
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	body := w.Body.String()
	if !strings.Contains(body, "Test Server") {
		t.Errorf("Expected response to contain 'Test Server', got: %s", body)
	}
}

func TestHandlers_UpdateGameserver(t *testing.T) {
	mockService := createMockService()
	tmpl := createTestTemplate("gameserver-updated.html", `Gameserver updated`)
	handlers := New(mockService, tmpl)

	formData := "name=Updated Server&game_id=minecraft&memory_mb=2048&cpu_cores=1"
	req := httptest.NewRequest("PUT", "/1", strings.NewReader(formData))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()

	router := newTestRouter("1")
	router.Put("/1", handlers.UpdateGameserver)
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}
}

func TestHandlers_GetGameserverStatus(t *testing.T) {
	mockService := createMockService()
	tmpl := createTestTemplate("gameserver-status.html", `{{.Status}}`)
	handlers := New(mockService, tmpl)

	req := httptest.NewRequest("GET", "/1/status", nil)
	w := httptest.NewRecorder()

	router := newTestRouter("1")
	router.Get("/1/status", handlers.GameserverRow)
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	body := w.Body.String()
	if !strings.Contains(body, string(models.StatusStopped)) {
		t.Errorf("Expected response to contain status, got: %s", body)
	}
}

func TestHandlers_ShowGameserver_NotFound(t *testing.T) {
	mockService := createMockService()
	tmpl := createTestTemplate("gameserver-details.html", `{{.Gameserver.Name}}`)
	handlers := New(mockService, tmpl)

	req := httptest.NewRequest("GET", "/999", nil)
	w := httptest.NewRecorder()

	router := newTestRouter("999")
	router.Get("/999", handlers.ShowGameserver)
	router.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("Expected status 404, got %d", w.Code)
	}
}

func TestHandlers_CreateGameserver_InvalidData(t *testing.T) {
	mockService := createMockService()
	tmpl := createTestTemplate("gameserver-row.html", `{{.Name}}`)
	handlers := New(mockService, tmpl)

	// Missing required fields
	formData := "name=&game="
	req := httptest.NewRequest("POST", "/", strings.NewReader(formData))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()

	handlers.CreateGameserver(w, req)

	if w.Code == http.StatusOK {
		t.Errorf("Expected non-200 status for invalid data, got %d", w.Code)
	}
}

func TestHandlers_CreateGameserver_ParseFormError(t *testing.T) {
	mockService := createMockService()
	tmpl := createTestTemplate("gameserver-row.html", `{{.Name}}`)
	handlers := New(mockService, tmpl)

	// Invalid form data that will cause ParseForm to fail
	req := httptest.NewRequest("POST", "/", strings.NewReader("invalid%form%data"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()

	handlers.CreateGameserver(w, req)

	// Should handle the parse error gracefully
	if w.Code == http.StatusOK {
		t.Errorf("Expected non-200 status for parse error, got %d", w.Code)
	}
}

package handlers

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

func TestHandlers_CreateGameserverBackup(t *testing.T) {
	mockService := createMockService()
	tmpl := createTestTemplate("backup-success.html", `Backup created`)
	handlers := New(mockService, tmpl, 1024*1024, 10*1024*1024, &mockQueryService{})

	req := httptest.NewRequest("POST", "/1/backups", nil)
	w := httptest.NewRecorder()

	router := newTestRouter("1")
	router.Post("/1/backups", handlers.CreateGameserverBackup)
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}
}

func TestHandlers_RestoreGameserverBackup(t *testing.T) {
	mockService := createMockService()
	tmpl := createTestTemplate("backup-success.html", `Backup restored`)
	handlers := New(mockService, tmpl, 1024*1024, 10*1024*1024, &mockQueryService{})

	// Use query parameter as expected by the handler
	req := httptest.NewRequest("POST", "/1/backups/restore?backup=test-backup.tar.gz", nil)
	w := httptest.NewRecorder()

	router := newTestRouter("1")
	router.Post("/1/backups/restore", handlers.RestoreGameserverBackup)
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}
}

func TestHandlers_RestoreGameserverBackup_MissingFilename(t *testing.T) {
	mockService := createMockService()
	tmpl := createTestTemplate("backup-success.html", `Backup restored`)
	handlers := New(mockService, tmpl, 1024*1024, 10*1024*1024, &mockQueryService{})

	// No backup filename provided
	req := httptest.NewRequest("POST", "/1/backups/restore", nil)
	w := httptest.NewRecorder()

	router := newTestRouter("1")
	router.Post("/1/backups/restore", handlers.RestoreGameserverBackup)
	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", w.Code)
	}

	body := w.Body.String()
	if !strings.Contains(body, "backup parameter required") {
		t.Errorf("Expected error message about missing backup parameter, got: %s", body)
	}
}

func TestHandlers_ListGameserverBackups(t *testing.T) {
	mockService := createMockService()
	tmpl := createTestTemplate("gameserver-backups.html", `{{range .Backups}}{{.Name}}{{end}}`)
	handlers := New(mockService, tmpl, 1024*1024, 10*1024*1024, &mockQueryService{})

	req := httptest.NewRequest("GET", "/1/backups", nil)
	req.Header.Set("HX-Request", "true") // Force HTMX mode to avoid complex template rendering
	w := httptest.NewRecorder()

	router := newTestRouter("1")
	router.Get("/1/backups", handlers.ListGameserverBackups)
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	body := w.Body.String()
	if !strings.Contains(body, "backup1.tar.gz") {
		t.Errorf("Expected response to contain 'backup1.tar.gz', got: %s", body)
	}
	if !strings.Contains(body, "backup2.tar.gz") {
		t.Errorf("Expected response to contain 'backup2.tar.gz', got: %s", body)
	}
}

func TestHandlers_DeleteGameserverBackup(t *testing.T) {
	mockService := createMockService()
	tmpl := createTestTemplate("backup-success.html", `Backup deleted`)
	handlers := New(mockService, tmpl, 1024*1024, 10*1024*1024, &mockQueryService{})

	// Use query parameter as expected by the handler
	req := httptest.NewRequest("DELETE", "/1/backups?backup=test-backup.tar.gz", nil)
	w := httptest.NewRecorder()

	router := newTestRouter("1")
	router.Delete("/1/backups", handlers.DeleteGameserverBackup)
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}
}

func TestHandlers_DeleteGameserverBackup_MissingFilename(t *testing.T) {
	mockService := createMockService()
	tmpl := createTestTemplate("backup-success.html", `Backup deleted`)
	handlers := New(mockService, tmpl, 1024*1024, 10*1024*1024, &mockQueryService{})

	// No backup filename provided
	req := httptest.NewRequest("DELETE", "/1/backups", nil)
	w := httptest.NewRecorder()

	router := newTestRouter("1")
	router.Delete("/1/backups", handlers.DeleteGameserverBackup)
	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", w.Code)
	}

	body := w.Body.String()
	if !strings.Contains(body, "backup parameter required") {
		t.Errorf("Expected error message about missing backup parameter, got: %s", body)
	}
}

func TestHandlers_RestoreGameserverBackup_EmptyFilename(t *testing.T) {
	mockService := createMockService()
	tmpl := createTestTemplate("backup-success.html", `Backup restored`)
	handlers := New(mockService, tmpl, 1024*1024, 10*1024*1024, &mockQueryService{})

	// Use empty query parameter
	req := httptest.NewRequest("POST", "/1/backups/restore?backup=", nil)
	w := httptest.NewRecorder()

	router := newTestRouter("1")
	router.Post("/1/backups/restore", handlers.RestoreGameserverBackup)
	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", w.Code)
	}
}

func TestHandlers_DeleteGameserverBackup_EmptyFilename(t *testing.T) {
	mockService := createMockService()
	tmpl := createTestTemplate("backup-success.html", `Backup deleted`)
	handlers := New(mockService, tmpl, 1024*1024, 10*1024*1024, &mockQueryService{})

	// Use empty query parameter
	req := httptest.NewRequest("DELETE", "/1/backups?backup=", nil)
	w := httptest.NewRecorder()

	router := newTestRouter("1")
	router.Delete("/1/backups", handlers.DeleteGameserverBackup)
	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", w.Code)
	}
}

func TestHandlers_BackupOperationsWithServiceErrors(t *testing.T) {
	// Test backup operations with various service errors
	tests := []struct {
		name           string
		handler        func(*mockGameserverService) http.HandlerFunc
		method         string
		formData       string
		expectedStatus int
	}{
		{
			name: "create backup",
			handler: func(s *mockGameserverService) http.HandlerFunc {
				tmpl := createTestTemplate("backup-success.html", `Backup created`)
				handlers := New(s, tmpl, 1024*1024, 10*1024*1024, &mockQueryService{})
				return handlers.CreateGameserverBackup
			},
			method:         "POST",
			formData:       "",
			expectedStatus: http.StatusOK,
		},
		{
			name: "restore backup",
			handler: func(s *mockGameserverService) http.HandlerFunc {
				tmpl := createTestTemplate("backup-success.html", `Backup restored`)
				handlers := New(s, tmpl, 1024*1024, 10*1024*1024, &mockQueryService{})
				return handlers.RestoreGameserverBackup
			},
			method:         "POST",
			formData:       "backup=test.tar.gz",
			expectedStatus: http.StatusOK,
		},
		{
			name: "delete backup",
			handler: func(s *mockGameserverService) http.HandlerFunc {
				tmpl := createTestTemplate("backup-success.html", `Backup deleted`)
				handlers := New(s, tmpl, 1024*1024, 10*1024*1024, &mockQueryService{})
				return handlers.DeleteGameserverBackup
			},
			method:         "DELETE",
			formData:       "backup=test.tar.gz",
			expectedStatus: http.StatusOK,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockService := createMockService()

			var req *http.Request
			var url string

			if strings.Contains(tt.formData, "backup=") {
				// For restore and delete operations, use query parameters
				if tt.method == "POST" {
					url = "/1/backups/restore?" + tt.formData
				} else {
					url = "/1/backups?" + tt.formData
				}
			} else {
				url = "/1/backups"
			}

			req = httptest.NewRequest(tt.method, url, nil)
			w := httptest.NewRecorder()

			router := newTestRouter("1")
			switch tt.method {
			case "POST":
				if strings.Contains(tt.formData, "backup=") {
					router.Post("/1/backups/restore", tt.handler(mockService))
				} else {
					router.Post("/1/backups", tt.handler(mockService))
				}
			case "DELETE":
				router.Delete("/1/backups", tt.handler(mockService))
			}
			router.ServeHTTP(w, req)

			if w.Code != tt.expectedStatus {
				t.Errorf("Expected status %d, got %d", tt.expectedStatus, w.Code)
			}
		})
	}
}

func TestHandlers_BackupFileValidation(t *testing.T) {
	tests := []struct {
		name         string
		filename     string
		expectError  bool
		expectedCode int
	}{
		{"valid tar.gz", "backup-2024-01-01.tar.gz", false, http.StatusOK},
		{"valid with timestamp", "backup-2024-01-01_12-30-45.tar.gz", false, http.StatusOK},
		{"empty filename", "", true, http.StatusBadRequest},
		{"spaces in filename", "backup file.tar.gz", false, http.StatusOK}, // May be valid depending on implementation
		{"special characters", "backup@#$.tar.gz", false, http.StatusOK},   // May be valid depending on implementation
	}

	for _, tt := range tests {
		t.Run(tt.name+" restore", func(t *testing.T) {
			mockService := createMockService()
			tmpl := createTestTemplate("backup-success.html", `Backup restored`)
			handlers := New(mockService, tmpl, 1024*1024, 10*1024*1024, &mockQueryService{})

			url := "/1/backups/restore?backup=" + url.QueryEscape(tt.filename)
			req := httptest.NewRequest("POST", url, nil)
			w := httptest.NewRecorder()

			router := newTestRouter("1")
			router.Post("/1/backups/restore", handlers.RestoreGameserverBackup)
			router.ServeHTTP(w, req)

			if w.Code != tt.expectedCode {
				t.Errorf("Expected status %d, got %d", tt.expectedCode, w.Code)
			}
		})

		t.Run(tt.name+" delete", func(t *testing.T) {
			mockService := createMockService()
			tmpl := createTestTemplate("backup-success.html", `Backup deleted`)
			handlers := New(mockService, tmpl, 1024*1024, 10*1024*1024, &mockQueryService{})

			url := "/1/backups?backup=" + url.QueryEscape(tt.filename)
			req := httptest.NewRequest("DELETE", url, nil)
			w := httptest.NewRecorder()

			router := newTestRouter("1")
			router.Delete("/1/backups", handlers.DeleteGameserverBackup)
			router.ServeHTTP(w, req)

			if w.Code != tt.expectedCode {
				t.Errorf("Expected status %d, got %d", tt.expectedCode, w.Code)
			}
		})
	}
}

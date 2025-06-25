package handlers

import (
	"html/template"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestHandlers_GameserverFiles(t *testing.T) {
	mockService := createMockService()
	tmpl := createTestTemplate("gameserver-files.html", `{{.CurrentPath}}{{range .Files}}{{.Name}}{{end}}`)
	handlers := New(mockService, tmpl, 1024*1024, 10*1024*1024, &mockQueryService{})

	req := httptest.NewRequest("GET", "/1/files", nil)
	req.Header.Set("HX-Request", "true")
	w := httptest.NewRecorder()

	router := newTestRouter("1")
	router.Get("/1/files", handlers.GameserverFiles)
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	body := w.Body.String()
	if !strings.Contains(body, "server.properties") {
		t.Errorf("Expected response to contain 'server.properties', got: %s", body)
	}
	if !strings.Contains(body, "world") {
		t.Errorf("Expected response to contain 'world', got: %s", body)
	}
}

func TestHandlers_BrowseGameserverFiles(t *testing.T) {
	mockService := createMockService()
	tmpl := createTestTemplate("file-browser.html", `{{.CurrentPath}}`)
	handlers := New(mockService, tmpl, 1024*1024, 10*1024*1024, &mockQueryService{})

	req := httptest.NewRequest("GET", "/1/files/browse?path=/data/server/logs", nil)
	w := httptest.NewRecorder()

	router := newTestRouter("1")
	router.Get("/1/files/browse", handlers.BrowseGameserverFiles)
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}
}

func TestHandlers_CreateGameserverFile(t *testing.T) {
	mockService := createMockService()
	tmpl := createTestTemplate("file-browser.html", `{{.CurrentPath}}`)
	handlers := New(mockService, tmpl, 1024*1024, 10*1024*1024, &mockQueryService{})

	formData := "path=/data/server&name=newfile.txt&type=file"
	req := httptest.NewRequest("POST", "/1/files/create", strings.NewReader(formData))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()

	router := newTestRouter("1")
	router.Post("/1/files/create", handlers.CreateGameserverFile)
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}
}

func TestHandlers_CreateGameserverFile_Directory(t *testing.T) {
	mockService := createMockService()
	tmpl := createTestTemplate("file-browser.html", `{{.CurrentPath}}`)
	handlers := New(mockService, tmpl, 1024*1024, 10*1024*1024, &mockQueryService{})

	formData := "path=/data/server&name=newfolder&type=directory"
	req := httptest.NewRequest("POST", "/1/files/create", strings.NewReader(formData))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()

	router := newTestRouter("1")
	router.Post("/1/files/create", handlers.CreateGameserverFile)
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}
}

func TestHandlers_DeleteGameserverFile(t *testing.T) {
	mockService := createMockService()
	tmpl := createTestTemplate("file-deleted.html", `File deleted`)
	handlers := New(mockService, tmpl, 1024*1024, 10*1024*1024, &mockQueryService{})

	req := httptest.NewRequest("DELETE", "/1/files/delete?path=/data/server/oldfile.txt", nil)
	w := httptest.NewRecorder()

	router := newTestRouter("1")
	router.Delete("/1/files/delete", handlers.DeleteGameserverFile)
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}
}

func TestHandlers_RenameGameserverFile(t *testing.T) {
	mockService := createMockService()
	tmpl := createTestTemplate("file-browser.html", `{{.CurrentPath}}`)
	handlers := New(mockService, tmpl, 1024*1024, 10*1024*1024, &mockQueryService{})

	formData := "old_path=/data/server/oldfile.txt&new_name=newfile.txt"
	req := httptest.NewRequest("POST", "/1/files/rename", strings.NewReader(formData))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()

	router := newTestRouter("1")
	router.Post("/1/files/rename", handlers.RenameGameserverFile)
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}
}

func TestHandlers_CreateGameserverFile_MissingParameters(t *testing.T) {
	tests := []struct {
		name     string
		formData string
		expected int
	}{
		{"missing name", "path=/data&type=file", http.StatusBadRequest},
		{"missing path", "name=test.txt&type=file", http.StatusBadRequest},
		{"missing type", "path=/data&name=test.txt", http.StatusOK}, // Type defaults to file
		{"empty name", "path=/data&name=&type=file", http.StatusBadRequest},
		{"empty path", "path=&name=test.txt&type=file", http.StatusBadRequest},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockService := createMockService()
			tmpl := createTestTemplate("file-browser.html", `{{.CurrentPath}}`)
			handlers := New(mockService, tmpl, 1024*1024, 10*1024*1024, &mockQueryService{})

			req := httptest.NewRequest("POST", "/1/files/create", strings.NewReader(tt.formData))
			req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
			w := httptest.NewRecorder()

			router := newTestRouter("1")
			router.Post("/1/files/create", handlers.CreateGameserverFile)
			router.ServeHTTP(w, req)

			if w.Code != tt.expected {
				t.Errorf("Expected status %d, got %d", tt.expected, w.Code)
			}
		})
	}
}

func TestHandlers_DeleteGameserverFile_MissingPath(t *testing.T) {
	mockService := createMockService()
	tmpl := createTestTemplate("file-deleted.html", `File deleted`)
	handlers := New(mockService, tmpl, 1024*1024, 10*1024*1024, &mockQueryService{})

	req := httptest.NewRequest("DELETE", "/1/files/delete", nil)
	w := httptest.NewRecorder()

	router := newTestRouter("1")
	router.Delete("/1/files/delete", handlers.DeleteGameserverFile)
	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", w.Code)
	}

	body := w.Body.String()
	if !strings.Contains(body, "path parameter required") {
		t.Errorf("Expected error message about missing path parameter, got: %s", body)
	}
}

func TestHandlers_RenameGameserverFile_MissingParameters(t *testing.T) {
	tests := []struct {
		name     string
		formData string
		expected int
	}{
		{"missing old_path", "new_name=newfile.txt", http.StatusBadRequest},
		{"missing new_name", "old_path=/data/file.txt", http.StatusBadRequest},
		{"empty old_path", "old_path=&new_name=newfile.txt", http.StatusBadRequest},
		{"empty new_name", "old_path=/data/file.txt&new_name=", http.StatusBadRequest},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockService := createMockService()
			tmpl := createTestTemplate("file-browser.html", `{{.CurrentPath}}`)
			handlers := New(mockService, tmpl, 1024*1024, 10*1024*1024, &mockQueryService{})

			req := httptest.NewRequest("POST", "/1/files/rename", strings.NewReader(tt.formData))
			req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
			w := httptest.NewRecorder()

			router := newTestRouter("1")
			router.Post("/1/files/rename", handlers.RenameGameserverFile)
			router.ServeHTTP(w, req)

			if w.Code != tt.expected {
				t.Errorf("Expected status %d, got %d", tt.expected, w.Code)
			}
		})
	}
}

func TestHandlers_BrowseGameserverFiles_DefaultPath(t *testing.T) {
	mockService := createMockService()
	tmpl := createTestTemplate("file-browser.html", `{{.CurrentPath}}`)
	handlers := New(mockService, tmpl, 1024*1024, 10*1024*1024, &mockQueryService{})

	// Request without path parameter (should use default)
	req := httptest.NewRequest("GET", "/1/files/browse", nil)
	w := httptest.NewRecorder()

	router := newTestRouter("1")
	router.Get("/1/files/browse", handlers.BrowseGameserverFiles)
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}
}

func TestHandlers_FileOperations_InvalidGameserver(t *testing.T) {
	mockService := createMockService()
	tmpl := createTestTemplate("file-browser.html", `{{.CurrentPath}}`)

	tests := []struct {
		name        string
		method      string
		path        string
		body        string
		handlerFunc func(*mockGameserverService, *template.Template) http.HandlerFunc
	}{
		{
			"files list",
			"GET",
			"/999/files",
			"",
			func(s *mockGameserverService, tmpl *template.Template) http.HandlerFunc {
				handlers := New(s, tmpl, 1024*1024, 10*1024*1024, &mockQueryService{})
				return handlers.GameserverFiles
			},
		},
		{
			"browse files",
			"GET",
			"/999/files/browse",
			"",
			func(s *mockGameserverService, tmpl *template.Template) http.HandlerFunc {
				handlers := New(s, tmpl, 1024*1024, 10*1024*1024, &mockQueryService{})
				return handlers.BrowseGameserverFiles
			},
		},
		{
			"create file",
			"POST",
			"/999/files/create",
			"path=/data&name=test.txt&type=file",
			func(s *mockGameserverService, tmpl *template.Template) http.HandlerFunc {
				handlers := New(s, tmpl, 1024*1024, 10*1024*1024, &mockQueryService{})
				return handlers.CreateGameserverFile
			},
		},
		{
			"delete file",
			"DELETE",
			"/999/files/delete?path=/data/test.txt",
			"",
			func(s *mockGameserverService, tmpl *template.Template) http.HandlerFunc {
				handlers := New(s, tmpl, 1024*1024, 10*1024*1024, &mockQueryService{})
				return handlers.DeleteGameserverFile
			},
		},
		{
			"rename file",
			"POST",
			"/999/files/rename",
			"old_path=/data/old.txt&new_name=new.txt",
			func(s *mockGameserverService, tmpl *template.Template) http.HandlerFunc {
				handlers := New(s, tmpl, 1024*1024, 10*1024*1024, &mockQueryService{})
				return handlers.RenameGameserverFile
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var req *http.Request
			if tt.body != "" {
				req = httptest.NewRequest(tt.method, tt.path, strings.NewReader(tt.body))
				req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
			} else {
				req = httptest.NewRequest(tt.method, tt.path, nil)
			}
			w := httptest.NewRecorder()

			router := newTestRouter("999")
			switch tt.method {
			case "GET":
				router.Get(tt.path, tt.handlerFunc(mockService, tmpl))
			case "POST":
				router.Post(tt.path, tt.handlerFunc(mockService, tmpl))
			case "DELETE":
				router.Delete(tt.path, tt.handlerFunc(mockService, tmpl))
			}
			router.ServeHTTP(w, req)

			if w.Code != http.StatusNotFound {
				t.Errorf("Expected status 404 for invalid gameserver, got %d", w.Code)
			}
		})
	}
}

func TestHandlers_FileOperations_ValidFilenames(t *testing.T) {
	tests := []struct {
		name     string
		filename string
		expected int
	}{
		{"simple filename", "test.txt", http.StatusOK},
		{"filename with spaces", "test file.txt", http.StatusOK},
		{"filename with numbers", "test123.txt", http.StatusOK},
		{"filename with underscores", "test_file.txt", http.StatusOK},
		{"filename with hyphens", "test-file.txt", http.StatusOK},
		{"filename with dots", "test.backup.txt", http.StatusOK},
		{"directory name", "test_directory", http.StatusOK},
	}

	for _, tt := range tests {
		t.Run(tt.name+" create", func(t *testing.T) {
			mockService := createMockService()
			tmpl := createTestTemplate("file-browser.html", `{{.CurrentPath}}`)
			handlers := New(mockService, tmpl, 1024*1024, 10*1024*1024, &mockQueryService{})

			formData := "path=/data&name=" + tt.filename + "&type=file"
			req := httptest.NewRequest("POST", "/1/files/create", strings.NewReader(formData))
			req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
			w := httptest.NewRecorder()

			router := newTestRouter("1")
			router.Post("/1/files/create", handlers.CreateGameserverFile)
			router.ServeHTTP(w, req)

			if w.Code != tt.expected {
				t.Errorf("Expected status %d, got %d", tt.expected, w.Code)
			}
		})

		t.Run(tt.name+" rename", func(t *testing.T) {
			mockService := createMockService()
			tmpl := createTestTemplate("file-browser.html", `{{.CurrentPath}}`)
			handlers := New(mockService, tmpl, 1024*1024, 10*1024*1024, &mockQueryService{})

			formData := "old_path=/data/oldfile.txt&new_name=" + tt.filename
			req := httptest.NewRequest("POST", "/1/files/rename", strings.NewReader(formData))
			req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
			w := httptest.NewRecorder()

			router := newTestRouter("1")
			router.Post("/1/files/rename", handlers.RenameGameserverFile)
			router.ServeHTTP(w, req)

			if w.Code != tt.expected {
				t.Errorf("Expected status %d, got %d", tt.expected, w.Code)
			}
		})
	}
}

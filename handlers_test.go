package main

import (
	"html/template"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"

	"0xkowalskidev/gameservers/models"
	"0xkowalskidev/gameservers/handlers"
)

// Initialize handler function variables for testing
func init() {
	handlers.HandleError = HandleError
	handlers.NotFound = NotFound
	handlers.BadRequest = BadRequest
	handlers.InternalError = InternalError
	handlers.ParseForm = ParseForm
	handlers.RequireMethod = RequireMethod
	handlers.LogAndRespond = LogAndRespond
	handlers.Render = Render
}


// Helper function to create test templates with all required templates
func createTestTemplate(contentTemplate string, contentParsing string) *template.Template {
	tmpl := template.Must(template.New(contentTemplate).Parse(contentParsing))
	template.Must(tmpl.New("layout.html").Parse(`{{.Content}}`))
	template.Must(tmpl.New("gameserver-wrapper.html").Parse(`{{.Content}}`))
	template.Must(tmpl.New("gameserver-backups.html").Parse(`{{range .Backups}}{{.Name}}{{end}}`))
	// Only add gameserver-files.html if it's not the content template to avoid conflicts
	if contentTemplate != "gameserver-files.html" {
		template.Must(tmpl.New("gameserver-files.html").Parse(`{{.CurrentPath}}{{range .Files}}{{.Name}}{{end}}`))
	}
	return tmpl
}

type mockGameserverService struct {
	games       []*models.Game
	gameservers []*models.Gameserver
}

func (m *mockGameserverService) CreateGameserver(server *models.Gameserver) error { return nil }
func (m *mockGameserverService) GetGameserver(id string) (*models.Gameserver, error) {
	for _, gs := range m.gameservers {
		if gs.ID == id {
			return gs, nil
		}
	}
	return nil, &models.DatabaseError{Op: "get", Msg: "not found"}
}
func (m *mockGameserverService) UpdateGameserver(server *models.Gameserver) error { return nil }
func (m *mockGameserverService) ListGameservers() ([]*models.Gameserver, error) { return m.gameservers, nil }
func (m *mockGameserverService) StartGameserver(id string) error         { return nil }
func (m *mockGameserverService) StopGameserver(id string) error          { return nil }
func (m *mockGameserverService) RestartGameserver(id string) error       { return nil }
func (m *mockGameserverService) SendGameserverCommand(id string, command string) error { return nil }
func (m *mockGameserverService) DeleteGameserver(id string) error        { return nil }
func (m *mockGameserverService) ListGames() ([]*models.Game, error)             { return m.games, nil }
func (m *mockGameserverService) GetGame(id string) (*models.Game, error)        { return nil, nil }
func (m *mockGameserverService) CreateGame(game *models.Game) error             { return nil }
func (m *mockGameserverService) StreamGameserverLogs(id string) (io.ReadCloser, error) {
	return io.NopCloser(strings.NewReader("log stream")), nil
}
func (m *mockGameserverService) StreamGameserverStats(id string) (io.ReadCloser, error) {
	return io.NopCloser(strings.NewReader(`{"cpu_stats":{"cpu_usage":{"total_usage":100},"system_cpu_usage":200},"precpu_stats":{"cpu_usage":{"total_usage":50},"system_cpu_usage":100},"memory_stats":{"usage":536870912,"limit":1073741824}}`)), nil
}

// Scheduled Task methods
func (m *mockGameserverService) CreateScheduledTask(task *models.ScheduledTask) error { return nil }
func (m *mockGameserverService) GetScheduledTask(id string) (*models.ScheduledTask, error) { 
	return &models.ScheduledTask{ID: id, Name: "Mock Task", Type: models.TaskTypeRestart}, nil 
}
func (m *mockGameserverService) UpdateScheduledTask(task *models.ScheduledTask) error { return nil }
func (m *mockGameserverService) DeleteScheduledTask(id string) error { return nil }
func (m *mockGameserverService) ListScheduledTasksForGameserver(gameserverID string) ([]*models.ScheduledTask, error) {
	return []*models.ScheduledTask{
		{ID: "task-1", GameserverID: gameserverID, Name: "Daily Restart", Type: models.TaskTypeRestart, Status: models.TaskStatusActive},
		{ID: "task-2", GameserverID: gameserverID, Name: "Weekly Backup", Type: models.TaskTypeBackup, Status: models.TaskStatusActive},
	}, nil
}
func (m *mockGameserverService) CreateGameserverBackup(gameserverID string) error { return nil }
func (m *mockGameserverService) RestoreGameserverBackup(gameserverID, backupFilename string) error { return nil }
func (m *mockGameserverService) ListGameserverBackups(gameserverID string) ([]*models.FileInfo, error) {
	return []*models.FileInfo{
		{Name: "backup1.tar.gz", Size: 1024, IsDir: false},
		{Name: "backup2.tar.gz", Size: 2048, IsDir: false},
	}, nil
}

// File manager methods
func (m *mockGameserverService) ListFiles(containerID string, path string) ([]*models.FileInfo, error) { 
	// Return some mock files for backup list testing
	if path == "/data/backups" {
		return []*models.FileInfo{
			{Name: "backup1.tar.gz", Size: 1024, IsDir: false},
			{Name: "backup2.tar.gz", Size: 2048, IsDir: false},
			{Name: "notabackup.txt", Size: 100, IsDir: false}, // This should be filtered out
		}, nil
	}
	return []*models.FileInfo{
		{Name: "server.properties", Size: 1024, IsDir: false},
		{Name: "logs", Size: 0, IsDir: true},
	}, nil 
}
func (m *mockGameserverService) ReadFile(containerID string, path string) ([]byte, error) { return []byte("mock content"), nil }
func (m *mockGameserverService) WriteFile(containerID string, path string, content []byte) error { return nil }
func (m *mockGameserverService) CreateDirectory(containerID string, path string) error { return nil }
func (m *mockGameserverService) DeletePath(containerID string, path string) error { return nil }
func (m *mockGameserverService) DownloadFile(containerID string, path string) (io.ReadCloser, error) { return io.NopCloser(strings.NewReader("mock file")), nil }
func (m *mockGameserverService) RenameFile(containerID string, oldPath string, newPath string) error { return nil }
func (m *mockGameserverService) UploadFile(containerID string, destPath string, reader io.Reader) error { return nil }

func TestHandlers_IndexGameservers(t *testing.T) {
	tmpl := template.Must(template.New("index.html").Parse(`{{range .Gameservers}}{{.Name}}{{end}}`))
	template.Must(tmpl.New("layout.html").Parse(`{{.Content}}`))
	svc := &mockGameserverService{
		gameservers: []*models.Gameserver{{ID: "1", Name: "test"}},
	}
	h := handlers.New(svc, tmpl)

	req := httptest.NewRequest("GET", "/", nil)
	w := httptest.NewRecorder()

	h.IndexGameservers(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}
	if !strings.Contains(w.Body.String(), "test") {
		t.Errorf("expected body to contain 'test'")
	}
}

func TestHandlers_CreateGameserver(t *testing.T) {
	tmpl := template.Must(template.New("test").Parse(`{{.}}`))
	svc := &mockGameserverService{}
	h := handlers.New(svc, tmpl)

	body := strings.NewReader("name=test&game_id=minecraft&port=25565")
	req := httptest.NewRequest("POST", "/", body)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()

	h.CreateGameserver(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}
	if w.Header().Get("HX-Redirect") != "/" {
		t.Errorf("expected HX-Redirect header")
	}
}

func TestHandlers_ShowGameserver(t *testing.T) {
	tmpl := createTestTemplate("gameserver-details.html", `{{.Gameserver.Name}}`)
	svc := &mockGameserverService{
		gameservers: []*models.Gameserver{{ID: "1", Name: "test"}},
	}
	h := handlers.New(svc, tmpl)

	req := httptest.NewRequest("GET", "/1", nil)
	req.Header.Set("HX-Request", "true")
	w := httptest.NewRecorder()
	
	r := chi.NewRouter()
	r.Get("/{id}", h.ShowGameserver)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}
	body := w.Body.String()
	if !strings.Contains(body, "test") {
		t.Errorf("expected body to contain 'test', got: %q", body)
	}
}

// =============================================================================
// Scheduled Task Handler Tests
// =============================================================================

func TestHandlers_ListGameserverTasks(t *testing.T) {
	tmpl := createTestTemplate("gameserver-tasks.html", `{{range .Tasks}}{{.Name}}{{end}}`)
	svc := &mockGameserverService{
		gameservers: []*models.Gameserver{{ID: "1", Name: "test"}},
	}
	h := handlers.New(svc, tmpl)

	req := httptest.NewRequest("GET", "/1/tasks", nil)
	req.Header.Set("HX-Request", "true")
	w := httptest.NewRecorder()

	r := chi.NewRouter()
	r.Get("/{id}/tasks", h.ListGameserverTasks)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}
	if !strings.Contains(w.Body.String(), "Daily Restart") {
		t.Errorf("expected body to contain task names")
	}
}

func TestHandlers_CreateGameserverTask(t *testing.T) {
	tmpl := template.Must(template.New("test").Parse(`{{.}}`))
	svc := &mockGameserverService{
		gameservers: []*models.Gameserver{{ID: "1", Name: "test"}},
	}
	h := handlers.New(svc, tmpl)

	body := strings.NewReader("name=Test Task&type=restart&cron_schedule=0 2 * * *")
	req := httptest.NewRequest("POST", "/1/tasks", body)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()

	r := chi.NewRouter()
	r.Post("/{id}/tasks", h.CreateGameserverTask)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}
	if w.Header().Get("HX-Redirect") != "/1/tasks" {
		t.Errorf("expected HX-Redirect to task list")
	}
}

func TestHandlers_DeleteGameserverTask(t *testing.T) {
	tmpl := template.Must(template.New("test").Parse(`{{.}}`))
	svc := &mockGameserverService{}
	h := handlers.New(svc, tmpl)

	req := httptest.NewRequest("DELETE", "/1/tasks/task-1", nil)
	w := httptest.NewRecorder()

	r := chi.NewRouter()
	r.Delete("/{id}/tasks/{taskId}", h.DeleteGameserverTask)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}
}

func TestHandlers_UpdateGameserverTask(t *testing.T) {
	tmpl := template.Must(template.New("test").Parse(`{{.}}`))
	svc := &mockGameserverService{}
	h := handlers.New(svc, tmpl)

	body := strings.NewReader("name=Updated Task&type=backup&status=active&cron_schedule=0 3 * * *")
	req := httptest.NewRequest("PUT", "/1/tasks/task-1", body)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()

	r := chi.NewRouter()
	r.Put("/{id}/tasks/{taskId}", h.UpdateGameserverTask)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}
	if w.Header().Get("HX-Redirect") != "/1/tasks" {
		t.Errorf("expected HX-Redirect to task list")
	}
}

// =============================================================================
// Backup Handler Tests
// =============================================================================

func TestHandlers_CreateGameserverBackup(t *testing.T) {
	tmpl := template.Must(template.New("test").Parse(`{{.}}`))
	svc := &mockGameserverService{}
	h := handlers.New(svc, tmpl)

	req := httptest.NewRequest("POST", "/1/backup", nil)
	w := httptest.NewRecorder()

	r := chi.NewRouter()
	r.Post("/{id}/backup", h.CreateGameserverBackup)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}
}

func TestHandlers_RestoreGameserverBackup(t *testing.T) {
	tmpl := template.Must(template.New("test").Parse(`{{.}}`))
	svc := &mockGameserverService{
		gameservers: []*models.Gameserver{{ID: "1", Name: "test"}},
	}
	h := handlers.New(svc, tmpl)

	req := httptest.NewRequest("POST", "/1/restore?backup=test-backup.tar.gz", nil)
	w := httptest.NewRecorder()

	r := chi.NewRouter()
	r.Post("/{id}/restore", h.RestoreGameserverBackup)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}
}

func TestHandlers_RestoreGameserverBackup_MissingFilename(t *testing.T) {
	tmpl := template.Must(template.New("test").Parse(`{{.}}`))
	svc := &mockGameserverService{}
	h := handlers.New(svc, tmpl)

	// Request without backup parameter
	req := httptest.NewRequest("POST", "/1/restore", nil)
	w := httptest.NewRecorder()

	r := chi.NewRouter()
	r.Post("/{id}/restore", h.RestoreGameserverBackup)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", w.Code)
	}
}

func TestHandlers_ListGameserverBackups(t *testing.T) {
	tmpl := createTestTemplate("backup-list.html", `{{range .Backups}}{{.Name}}{{end}}`)
	svc := &mockGameserverService{
		gameservers: []*models.Gameserver{{ID: "1", Name: "test", ContainerID: "container-1"}},
	}
	h := handlers.New(svc, tmpl)

	req := httptest.NewRequest("GET", "/1/backups", nil)
	w := httptest.NewRecorder()

	r := chi.NewRouter()
	r.Get("/{id}/backups", h.ListGameserverBackups)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}
}

func TestHandlers_DeleteGameserverBackup(t *testing.T) {
	tmpl := template.Must(template.New("test").Parse(`{{.}}`))
	svc := &mockGameserverService{
		gameservers: []*models.Gameserver{{ID: "1", Name: "test", ContainerID: "container-1"}},
	}
	h := handlers.New(svc, tmpl)

	req := httptest.NewRequest("DELETE", "/1/backups/delete?backup=test-backup.tar.gz", nil)
	w := httptest.NewRecorder()

	r := chi.NewRouter()
	r.Delete("/{id}/backups/delete", h.DeleteGameserverBackup)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}
}

func TestHandlers_DeleteGameserverBackup_MissingFilename(t *testing.T) {
	tmpl := template.Must(template.New("test").Parse(`{{.}}`))
	svc := &mockGameserverService{}
	h := handlers.New(svc, tmpl)

	// Request without backup parameter
	req := httptest.NewRequest("DELETE", "/1/backups/delete", nil)
	w := httptest.NewRecorder()

	r := chi.NewRouter()
	r.Delete("/{id}/backups/delete", h.DeleteGameserverBackup)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", w.Code)
	}
}

// =============================================================================
// File Manager Handler Tests
// =============================================================================

func TestHandlers_GameserverFiles(t *testing.T) {
	tmpl := createTestTemplate("gameserver-files.html", `{{.CurrentPath}}`)
	svc := &mockGameserverService{
		gameservers: []*models.Gameserver{{ID: "1", Name: "test", ContainerID: "container-1"}},
	}
	h := handlers.New(svc, tmpl)

	req := httptest.NewRequest("GET", "/1/files", nil)
	req.Header.Set("HX-Request", "true")
	w := httptest.NewRecorder()

	r := chi.NewRouter()
	r.Get("/{id}/files", h.GameserverFiles)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}
	if !strings.Contains(w.Body.String(), "/data/server") {
		t.Errorf("expected body to contain default path")
	}
}

func TestHandlers_BrowseGameserverFiles(t *testing.T) {
	tmpl := template.Must(template.New("file-browser.html").Parse(`{{.CurrentPath}}`))
	svc := &mockGameserverService{
		gameservers: []*models.Gameserver{{ID: "1", Name: "test", ContainerID: "container-1"}},
	}
	h := handlers.New(svc, tmpl)

	req := httptest.NewRequest("GET", "/1/files/browse?path=/data/server/logs", nil)
	w := httptest.NewRecorder()

	r := chi.NewRouter()
	r.Get("/{id}/files/browse", h.BrowseGameserverFiles)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}
}

func TestHandlers_CreateGameserverFile(t *testing.T) {
	tmpl := template.Must(template.New("file-browser.html").Parse(`{{.CurrentPath}}`))
	svc := &mockGameserverService{
		gameservers: []*models.Gameserver{{ID: "1", Name: "test", ContainerID: "container-1"}},
	}
	h := handlers.New(svc, tmpl)

	body := strings.NewReader("path=/data/server&name=newfile.txt&type=file")
	req := httptest.NewRequest("POST", "/1/files/create", body)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()

	r := chi.NewRouter()
	r.Post("/{id}/files/create", h.CreateGameserverFile)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}
}

func TestHandlers_DeleteGameserverFile(t *testing.T) {
	tmpl := template.Must(template.New("test").Parse(`{{.}}`))
	svc := &mockGameserverService{
		gameservers: []*models.Gameserver{{ID: "1", Name: "test", ContainerID: "container-1"}},
	}
	h := handlers.New(svc, tmpl)

	req := httptest.NewRequest("DELETE", "/1/files/delete?path=/data/server/oldfile.txt", nil)
	w := httptest.NewRecorder()

	r := chi.NewRouter()
	r.Delete("/{id}/files/delete", h.DeleteGameserverFile)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}
}

func TestHandlers_RenameGameserverFile(t *testing.T) {
	tmpl := template.Must(template.New("file-browser.html").Parse(`{{.CurrentPath}}`))
	svc := &mockGameserverService{
		gameservers: []*models.Gameserver{{ID: "1", Name: "test", ContainerID: "container-1"}},
	}
	h := handlers.New(svc, tmpl)

	body := strings.NewReader("old_path=/data/server/oldfile.txt&new_name=newfile.txt")
	req := httptest.NewRequest("POST", "/1/files/rename", body)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()

	r := chi.NewRouter()
	r.Post("/{id}/files/rename", h.RenameGameserverFile)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}
}
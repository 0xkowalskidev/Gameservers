package main

import (
	"html/template"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
)

type mockGameserverService struct {
	games       []*Game
	gameservers []*Gameserver
}

func (m *mockGameserverService) CreateGameserver(server *Gameserver) error { return nil }
func (m *mockGameserverService) GetGameserver(id string) (*Gameserver, error) {
	for _, gs := range m.gameservers {
		if gs.ID == id {
			return gs, nil
		}
	}
	return nil, &DatabaseError{Op: "get", Msg: "not found"}
}
func (m *mockGameserverService) UpdateGameserver(server *Gameserver) error { return nil }
func (m *mockGameserverService) ListGameservers() ([]*Gameserver, error) { return m.gameservers, nil }
func (m *mockGameserverService) StartGameserver(id string) error         { return nil }
func (m *mockGameserverService) StopGameserver(id string) error          { return nil }
func (m *mockGameserverService) RestartGameserver(id string) error       { return nil }
func (m *mockGameserverService) DeleteGameserver(id string) error        { return nil }
func (m *mockGameserverService) ListGames() ([]*Game, error)             { return m.games, nil }
func (m *mockGameserverService) GetGame(id string) (*Game, error)        { return nil, nil }
func (m *mockGameserverService) CreateGame(game *Game) error             { return nil }
func (m *mockGameserverService) StreamGameserverLogs(id string) (io.ReadCloser, error) {
	return io.NopCloser(strings.NewReader("log stream")), nil
}
func (m *mockGameserverService) StreamGameserverStats(id string) (io.ReadCloser, error) {
	return io.NopCloser(strings.NewReader(`{"cpu_stats":{"cpu_usage":{"total_usage":100},"system_cpu_usage":200},"precpu_stats":{"cpu_usage":{"total_usage":50},"system_cpu_usage":100},"memory_stats":{"usage":536870912,"limit":1073741824}}`)), nil
}

// Scheduled Task methods
func (m *mockGameserverService) CreateScheduledTask(task *ScheduledTask) error { return nil }
func (m *mockGameserverService) GetScheduledTask(id string) (*ScheduledTask, error) { 
	return &ScheduledTask{ID: id, Name: "Mock Task", Type: TaskTypeRestart}, nil 
}
func (m *mockGameserverService) UpdateScheduledTask(task *ScheduledTask) error { return nil }
func (m *mockGameserverService) DeleteScheduledTask(id string) error { return nil }
func (m *mockGameserverService) ListScheduledTasksForGameserver(gameserverID string) ([]*ScheduledTask, error) {
	return []*ScheduledTask{
		{ID: "task-1", GameserverID: gameserverID, Name: "Daily Restart", Type: TaskTypeRestart, Status: TaskStatusActive},
		{ID: "task-2", GameserverID: gameserverID, Name: "Weekly Backup", Type: TaskTypeBackup, Status: TaskStatusActive},
	}, nil
}
func (m *mockGameserverService) RestoreGameserverBackup(gameserverID, backupPath string) error { return nil }

func TestHandlers_IndexGameservers(t *testing.T) {
	tmpl := template.Must(template.New("index.html").Parse(`{{range .Gameservers}}{{.Name}}{{end}}`))
	template.Must(tmpl.New("layout.html").Parse(`{{.Content}}`))
	svc := &mockGameserverService{
		gameservers: []*Gameserver{{ID: "1", Name: "test"}},
	}
	h := NewHandlers(svc, tmpl)

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
	h := NewHandlers(svc, tmpl)

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
	tmpl := template.Must(template.New("gameserver-details.html").Parse(`{{.Gameserver.Name}}`))
	template.Must(tmpl.New("layout.html").Parse(`{{.Content}}`))
	svc := &mockGameserverService{
		gameservers: []*Gameserver{{ID: "1", Name: "test"}},
	}
	h := NewHandlers(svc, tmpl)

	req := httptest.NewRequest("GET", "/1", nil)
	w := httptest.NewRecorder()
	
	r := chi.NewRouter()
	r.Get("/{id}", h.ShowGameserver)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}
	if !strings.Contains(w.Body.String(), "test") {
		t.Errorf("expected body to contain 'test'")
	}
}

// =============================================================================
// Scheduled Task Handler Tests
// =============================================================================

func TestHandlers_ListGameserverTasks(t *testing.T) {
	tmpl := template.Must(template.New("gameserver-tasks.html").Parse(`{{range .Tasks}}{{.Name}}{{end}}`))
	template.Must(tmpl.New("layout.html").Parse(`{{.Content}}`))
	svc := &mockGameserverService{
		gameservers: []*Gameserver{{ID: "1", Name: "test"}},
	}
	h := NewHandlers(svc, tmpl)

	req := httptest.NewRequest("GET", "/1/tasks", nil)
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
		gameservers: []*Gameserver{{ID: "1", Name: "test"}},
	}
	h := NewHandlers(svc, tmpl)

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
	h := NewHandlers(svc, tmpl)

	req := httptest.NewRequest("DELETE", "/1/tasks/task-1", nil)
	w := httptest.NewRecorder()

	r := chi.NewRouter()
	r.Delete("/{id}/tasks/{taskId}", h.DeleteGameserverTask)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}
}
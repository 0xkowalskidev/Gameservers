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
func (m *mockGameserverService) ListGameservers() ([]*Gameserver, error) { return m.gameservers, nil }
func (m *mockGameserverService) StartGameserver(id string) error         { return nil }
func (m *mockGameserverService) StopGameserver(id string) error          { return nil }
func (m *mockGameserverService) RestartGameserver(id string) error       { return nil }
func (m *mockGameserverService) DeleteGameserver(id string) error        { return nil }
func (m *mockGameserverService) ListGames() ([]*Game, error)             { return m.games, nil }
func (m *mockGameserverService) GetGame(id string) (*Game, error)        { return nil, nil }
func (m *mockGameserverService) CreateGame(game *Game) error             { return nil }
func (m *mockGameserverService) GetGameserverLogs(id string, lines int) ([]string, error) {
	return []string{"log1", "log2"}, nil
}
func (m *mockGameserverService) GetGameserverStats(id string) (*ContainerStats, error) {
	return &ContainerStats{}, nil
}
func (m *mockGameserverService) StreamGameserverLogs(id string) (io.ReadCloser, error) {
	return io.NopCloser(strings.NewReader("log stream")), nil
}

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
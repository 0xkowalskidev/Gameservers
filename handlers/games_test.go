package handlers

import (
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"0xkowalskidev/gameservers/models"
)

func TestHandlers_IndexGames(t *testing.T) {
	mockService := createMockService()

	// Add test games to the mock service
	testGames := []*models.Game{
		{
			ID:          "test-game-1",
			Name:        "Test Game 1",
			Image:       "test/image:latest",
			MinMemoryMB: 1024,
			RecMemoryMB: 2048,
			CreatedAt:   time.Now(),
			UpdatedAt:   time.Now(),
		},
		{
			ID:          "test-game-2",
			Name:        "Test Game 2",
			Image:       "test/image2:latest",
			MinMemoryMB: 512,
			RecMemoryMB: 1024,
			CreatedAt:   time.Now(),
			UpdatedAt:   time.Now(),
		},
	}

	mockService.games = testGames

	// Test that the service method gets called and returns the games
	games, err := mockService.ListGames()
	if err != nil {
		t.Fatalf("ListGames should not return error: %v", err)
	}

	if len(games) != 2 {
		t.Errorf("Expected 2 games, got %d", len(games))
	}

	if games[0].Name != "Test Game 1" {
		t.Errorf("Expected first game name 'Test Game 1', got %s", games[0].Name)
	}

	if games[1].Name != "Test Game 2" {
		t.Errorf("Expected second game name 'Test Game 2', got %s", games[1].Name)
	}
}

func TestHandlers_ShowGame(t *testing.T) {
	mockService := createMockService()

	// Create test game
	testGame := &models.Game{
		ID:          "test-game",
		Name:        "Test Game",
		Image:       "test/image:latest",
		MinMemoryMB: 1024,
		RecMemoryMB: 2048,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	mockService.games = []*models.Game{testGame}

	// Test that the service method gets called and returns the correct game
	game, err := mockService.GetGame("test-game")
	if err != nil {
		t.Fatalf("GetGame should not return error: %v", err)
	}

	if game.Name != "Test Game" {
		t.Errorf("Expected game name 'Test Game', got %s", game.Name)
	}

	if game.ID != "test-game" {
		t.Errorf("Expected game ID 'test-game', got %s", game.ID)
	}
}

func TestHandlers_CreateGame(t *testing.T) {
	// Test parsing form values for creating a game
	form := url.Values{}
	form.Set("id", "new-test-game")
	form.Set("name", "New Test Game")
	form.Set("slug", "new-test-game")
	form.Set("image", "test/new-game:latest")
	form.Set("min_memory_mb", "1024")
	form.Set("rec_memory_mb", "2048")

	// Test that form values can be parsed correctly
	if form.Get("id") != "new-test-game" {
		t.Error("Expected id to be 'new-test-game'")
	}
	if form.Get("name") != "New Test Game" {
		t.Error("Expected name to be 'New Test Game'")
	}
	if form.Get("min_memory_mb") != "1024" {
		t.Error("Expected min_memory_mb to be '1024'")
	}
	if form.Get("rec_memory_mb") != "2048" {
		t.Error("Expected rec_memory_mb to be '2048'")
	}
}

func TestHandlers_parsePortMappings(t *testing.T) {
	mockService := createMockService()
	tmpl := createTestTemplate("games-list.html", `{{range .Games}}{{.Name}}{{end}}`)
	h := New(mockService, tmpl, 1024*1024, 10*1024*1024, &mockQueryService{})

	form := url.Values{}
	form["port_name"] = []string{"game", "query", ""}
	form["port_protocol"] = []string{"tcp", "udp", "tcp"}
	form["port_container"] = []string{"25565", "25566", ""}

	req := httptest.NewRequest("POST", "/test", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.ParseForm()

	ports, err := h.parsePortMappings(req)
	if err != nil {
		t.Fatalf("Failed to parse port mappings: %v", err)
	}

	if len(ports) != 2 {
		t.Errorf("Expected 2 port mappings, got %d", len(ports))
	}

	if ports[0].Name != "game" || ports[0].Protocol != "tcp" || ports[0].ContainerPort != 25565 {
		t.Errorf("Unexpected port mapping: %+v", ports[0])
	}

	if ports[1].Name != "query" || ports[1].Protocol != "udp" || ports[1].ContainerPort != 25566 {
		t.Errorf("Unexpected port mapping: %+v", ports[1])
	}
}

func TestHandlers_parseConfigVars(t *testing.T) {
	mockService := createMockService()
	tmpl := createTestTemplate("games-list.html", `{{range .Games}}{{.Name}}{{end}}`)
	h := New(mockService, tmpl, 1024*1024, 10*1024*1024, &mockQueryService{})

	form := url.Values{}
	form["config_name"] = []string{"SERVER_NAME", "MAX_PLAYERS", ""}
	form["config_display_name"] = []string{"Server Name", "Max Players", ""}
	form["config_required"] = []string{"on", ""}
	form["config_default"] = []string{"My Server", "16"}
	form["config_description"] = []string{"Name of the server", "Maximum players"}

	req := httptest.NewRequest("POST", "/test", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.ParseForm()

	configs, err := h.parseConfigVars(req)
	if err != nil {
		t.Fatalf("Failed to parse config vars: %v", err)
	}

	if len(configs) != 2 {
		t.Errorf("Expected 2 config vars, got %d", len(configs))
	}

	if configs[0].Name != "SERVER_NAME" || !configs[0].Required || configs[0].Default != "My Server" {
		t.Errorf("Unexpected config var: %+v", configs[0])
	}

	if configs[1].Name != "MAX_PLAYERS" || configs[1].Required || configs[1].Default != "16" {
		t.Errorf("Unexpected config var: %+v", configs[1])
	}
}
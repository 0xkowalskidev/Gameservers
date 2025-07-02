package handlers

import (
	"context"
	"fmt"
	"html/template"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/0xkowalskidev/gameserverquery/protocol"

	"0xkowalskidev/gameservers/models"
)

// Mock error types for testing
type mockError struct {
	Status  int
	Message string
}

func (e mockError) Error() string {
	return e.Message
}

func mockBadRequest(format string, args ...interface{}) error {
	return mockError{Status: 400, Message: fmt.Sprintf(format, args...)}
}

// Helper function to create test templates with all required templates
func createTestTemplate(contentTemplate string, contentParsing string) *template.Template {
	tmpl := template.Must(template.New(contentTemplate).Parse(contentParsing))
	template.Must(tmpl.New("layout.html").Parse(`{{.Content}}`))
	template.Must(tmpl.New("gameserver-wrapper.html").Parse(`{{.Content}}`))
	template.Must(tmpl.New("backup-list.html").Parse(`{{range .Backups}}{{.Name}}{{end}}`))
	template.Must(tmpl.New("gameserver-row.html").Parse(`{{.Name}} - {{.Status}}`))
	// Only add gameserver-files.html if it's not the content template to avoid conflicts
	if contentTemplate != "gameserver-files.html" {
		template.Must(tmpl.New("gameserver-files.html").Parse(`{{.CurrentPath}}{{range .Files}}{{.Name}}{{end}}`))
	}
	// Add gameserver-backups.html if it's not the content template to avoid conflicts
	if contentTemplate != "gameserver-backups.html" {
		template.Must(tmpl.New("gameserver-backups.html").Parse(`{{range .Backups}}{{.Name}}{{end}}`))
	}
	return tmpl
}

type mockGameserverService struct {
	games       []*models.Game
	gameservers []*models.Gameserver
	tasks       []*models.ScheduledTask
	backups     []string
	files       []*models.FileInfo
}

// Mock query service for testing
type mockQueryService struct {
	serverInfo *protocol.ServerInfo
	shouldFail bool
}

func (m *mockQueryService) QueryGameserver(gameserver *models.Gameserver, game *models.Game) (*protocol.ServerInfo, error) {
	if m.shouldFail {
		return nil, nil
	}
	if m.serverInfo != nil {
		return m.serverInfo, nil
	}
	// Default server info
	return &protocol.ServerInfo{
		Online: true,
		Players: protocol.PlayerInfo{
			Current: 5,
			Max:     20,
		},
		Map:  "de_dust2",
		Ping: int(25 * time.Millisecond),
	}, nil
}

func (m *mockGameserverService) CreateGameserver(server *models.Gameserver) error { return nil }

func (m *mockGameserverService) GetGameserver(id string) (*models.Gameserver, error) {
	for _, gs := range m.gameservers {
		if gs.ID == id {
			return gs, nil
		}
	}
	return nil, mockError{Status: 404, Message: "gameserver not found"}
}

func (m *mockGameserverService) UpdateGameserver(server *models.Gameserver) error { return nil }
func (m *mockGameserverService) DeleteGameserver(id string) error                 { return nil }
func (m *mockGameserverService) ListGameservers() ([]*models.Gameserver, error) {
	return m.gameservers, nil
}
func (m *mockGameserverService) StartGameserver(id string) error   { return nil }
func (m *mockGameserverService) StopGameserver(id string) error    { return nil }
func (m *mockGameserverService) RestartGameserver(id string) error { return nil }

func (m *mockGameserverService) ListGames() ([]*models.Game, error) { return m.games, nil }
func (m *mockGameserverService) GetGame(id string) (*models.Game, error) {
	for _, game := range m.games {
		if game.ID == id {
			return game, nil
		}
	}
	return nil, mockError{Status: 404, Message: "game not found"}
}
func (m *mockGameserverService) CreateGame(game *models.Game) error                    { return nil }
func (m *mockGameserverService) SendGameserverCommand(id string, command string) error { return nil }
func (m *mockGameserverService) StreamGameserverLogs(id string) (io.ReadCloser, error) {
	return io.NopCloser(strings.NewReader("log content")), nil
}
func (m *mockGameserverService) StreamGameserverStats(id string) (io.ReadCloser, error) {
	return io.NopCloser(strings.NewReader("stats content")), nil
}

func (m *mockGameserverService) CreateScheduledTask(task *models.ScheduledTask) error { return nil }
func (m *mockGameserverService) GetScheduledTask(id string) (*models.ScheduledTask, error) {
	for _, task := range m.tasks {
		if task.ID == id {
			return task, nil
		}
	}
	return nil, mockError{Status: 404, Message: "task not found"}
}
func (m *mockGameserverService) UpdateScheduledTask(task *models.ScheduledTask) error { return nil }
func (m *mockGameserverService) DeleteScheduledTask(id string) error {
	// Check if task exists in our mock data
	for _, task := range m.tasks {
		if task.ID == id {
			return nil // Task found, delete successful
		}
	}
	return mockError{Status: 404, Message: "task not found"}
}
func (m *mockGameserverService) ListScheduledTasksForGameserver(gameserverID string) ([]*models.ScheduledTask, error) {
	return m.tasks, nil
}

func (m *mockGameserverService) CreateGameserverBackup(gameserverID string) error { return nil }
func (m *mockGameserverService) RestoreGameserverBackup(gameserverID, backupFilename string) error {
	return nil
}
func (m *mockGameserverService) ListGameserverBackups(gameserverID string) ([]*models.FileInfo, error) {
	// Convert string backups to FileInfo for compatibility
	backupFiles := make([]*models.FileInfo, len(m.backups))
	for i, backup := range m.backups {
		backupFiles[i] = &models.FileInfo{Name: backup, Path: "/backups/" + backup, IsDir: false, Size: 1024}
	}
	return backupFiles, nil
}
func (m *mockGameserverService) DeleteGameserverBackup(gameserverID, backupFilename string) error {
	return nil
}

// File operations for the GameserverServiceInterface
func (m *mockGameserverService) ListFiles(containerID, path string) ([]*models.FileInfo, error) {
	return m.files, nil
}
func (m *mockGameserverService) ReadFile(containerID, path string) ([]byte, error) {
	return []byte("file content"), nil
}
func (m *mockGameserverService) WriteFile(containerID, path string, content []byte) error {
	return nil
}
func (m *mockGameserverService) CreateDirectory(containerID, path string) error { return nil }
func (m *mockGameserverService) DeletePath(containerID, path string) error      { return nil }
func (m *mockGameserverService) RenameFile(containerID, oldPath, newPath string) error {
	return nil
}
func (m *mockGameserverService) DownloadFile(containerID, path string) (io.ReadCloser, error) {
	return io.NopCloser(strings.NewReader("file content")), nil
}
func (m *mockGameserverService) UploadFile(containerID, destPath string, reader io.Reader) error {
	return nil
}

// Legacy methods for backward compatibility - some handlers might still use these
func (m *mockGameserverService) ListGameserverFiles(gameserverID, path string) ([]*models.FileInfo, error) {
	return m.files, nil
}
func (m *mockGameserverService) ReadGameserverFile(gameserverID, path string) ([]byte, error) {
	return []byte("file content"), nil
}
func (m *mockGameserverService) WriteGameserverFile(gameserverID, path string, content []byte) error {
	return nil
}
func (m *mockGameserverService) CreateGameserverDirectory(gameserverID, path string) error {
	return nil
}
func (m *mockGameserverService) DeleteGameserverPath(gameserverID, path string) error { return nil }
func (m *mockGameserverService) RenameGameserverFile(gameserverID, oldPath, newPath string) error {
	return nil
}
func (m *mockGameserverService) DownloadGameserverFile(gameserverID, path string) (io.ReadCloser, error) {
	return io.NopCloser(strings.NewReader("file content")), nil
}

// Mock error handler functions
func mockHandleError(w http.ResponseWriter, err error, context string) {
	if httpErr, ok := err.(mockError); ok {
		http.Error(w, httpErr.Message, httpErr.Status)
	} else {
		http.Error(w, "Internal Server Error", 500)
	}
}

func mockNotFound(resource string) error {
	return mockError{Status: 404, Message: resource + " not found"}
}

func mockInternalError(err error, message string) error {
	return mockError{Status: 500, Message: message}
}

func mockParseForm(r *http.Request) error {
	return r.ParseForm()
}

func mockRequireMethod(r *http.Request, method string) error {
	if r.Method != method {
		return mockError{Status: 405, Message: "Method Not Allowed"}
	}
	return nil
}

func mockLogAndRespond(w http.ResponseWriter, status int, message string, args ...interface{}) {
	w.WriteHeader(status)
	w.Write([]byte(message))
}

func mockRender(w http.ResponseWriter, r *http.Request, tmpl *template.Template, templateName string, data interface{}) {
	if err := tmpl.ExecuteTemplate(w, templateName, data); err != nil {
		http.Error(w, "Template Error", 500)
	}
}

// Initialize handler function variables for testing
func init() {
	HandleError = mockHandleError
	NotFound = mockNotFound
	BadRequest = mockBadRequest
	InternalError = mockInternalError
	ParseForm = mockParseForm
	RequireMethod = mockRequireMethod
	LogAndRespond = mockLogAndRespond
	Render = mockRender
}

// Helper function to create a new test router with gameserver ID
func newTestRouter(gameserverID string) *chi.Mux {
	r := chi.NewRouter()

	// Add middleware to inject the gameserver ID into all requests
	r.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			// Create new route context with the gameserver ID
			rctx := chi.NewRouteContext()
			rctx.URLParams.Add("id", gameserverID)
			ctx := context.WithValue(req.Context(), chi.RouteCtxKey, rctx)
			req = req.WithContext(ctx)
			next.ServeHTTP(w, req)
		})
	})

	return r
}

// Helper function to create a new test router with multiple URL parameters
func newTestRouterWithParams(params map[string]string) *chi.Mux {
	r := chi.NewRouter()

	// Add middleware to inject the URL parameters into all requests
	r.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			// Create new route context with the URL parameters
			rctx := chi.NewRouteContext()
			for key, value := range params {
				rctx.URLParams.Add(key, value)
			}
			ctx := context.WithValue(req.Context(), chi.RouteCtxKey, rctx)
			req = req.WithContext(ctx)
			next.ServeHTTP(w, req)
		})
	})

	return r
}

// Helper to create default mock data
func createMockService() *mockGameserverService {
	return &mockGameserverService{
		games: []*models.Game{
			{ID: "minecraft", Name: "Minecraft"},
			{ID: "valheim", Name: "Valheim"},
		},
		gameservers: []*models.Gameserver{
			{ID: "1", Name: "Test Server", GameID: "minecraft", Status: models.StatusStopped},
		},
		tasks: []*models.ScheduledTask{
			{ID: "task-1", GameserverID: "1", Name: "Test Task", Type: models.TaskTypeRestart},
		},
		backups: []string{"backup1.tar.gz", "backup2.tar.gz"},
		files: []*models.FileInfo{
			{Name: "server.properties", Path: "/data/server.properties", IsDir: false, Size: 1024},
			{Name: "world", Path: "/data/world", IsDir: true, Size: 0},
		},
	}
}

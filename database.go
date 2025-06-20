package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

type DatabaseError struct {
	Operation string
	Message   string
	Err       error
}

func (e *DatabaseError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("database %s failed: %s: %v", e.Operation, e.Message, e.Err)
	}
	return fmt.Sprintf("database %s failed: %s", e.Operation, e.Message)
}

type DatabaseManager struct {
	db *sql.DB
}

func NewDatabaseManager(dbPath string) (*DatabaseManager, error) {
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return nil, &DatabaseError{"connect", "failed to open database", err}
	}
	if err := db.Ping(); err != nil {
		return nil, &DatabaseError{"connect", "failed to ping database", err}
	}

	dm := &DatabaseManager{db: db}
	if err := dm.migrate(); err != nil {
		return nil, err
	}
	return dm, nil
}

func (dm *DatabaseManager) Close() error { return dm.db.Close() }

func (dm *DatabaseManager) migrate() error {
	schema := `
	CREATE TABLE IF NOT EXISTS gameservers (
		id TEXT PRIMARY KEY, name TEXT NOT NULL UNIQUE, game_type TEXT NOT NULL,
		image TEXT NOT NULL, container_id TEXT, status TEXT NOT NULL DEFAULT 'stopped',
		port INTEGER NOT NULL, environment TEXT, volumes TEXT,
		created_at DATETIME NOT NULL, updated_at DATETIME NOT NULL
	);
	CREATE INDEX IF NOT EXISTS idx_gameservers_status ON gameservers(status);`

	_, err := dm.db.Exec(schema)
	if err != nil {
		return &DatabaseError{"migrate", "failed to create schema", err}
	}
	return nil
}

func (dm *DatabaseManager) CreateGameServer(server *GameServer) error {
	envJSON, _ := json.Marshal(server.Environment)
	volumesJSON, _ := json.Marshal(server.Volumes)

	_, err := dm.db.Exec(`INSERT INTO gameservers (id, name, game_type, image, container_id, status, port, environment, volumes, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		server.ID, server.Name, server.GameType, server.Image, server.ContainerID, server.Status, server.Port, string(envJSON), string(volumesJSON), server.CreatedAt, server.UpdatedAt)

	if err != nil {
		return &DatabaseError{"create", fmt.Sprintf("failed to insert gameserver %s", server.Name), err}
	}
	return nil
}

func (dm *DatabaseManager) scanGameServer(row interface{ Scan(...interface{}) error }) (*GameServer, error) {
	var server GameServer
	var envJSON, volumesJSON string

	err := row.Scan(&server.ID, &server.Name, &server.GameType, &server.Image, &server.ContainerID, &server.Status, &server.Port, &envJSON, &volumesJSON, &server.CreatedAt, &server.UpdatedAt)
	if err != nil {
		return nil, err
	}

	json.Unmarshal([]byte(envJSON), &server.Environment)
	json.Unmarshal([]byte(volumesJSON), &server.Volumes)
	return &server, nil
}

func (dm *DatabaseManager) GetGameServer(id string) (*GameServer, error) {
	row := dm.db.QueryRow(`SELECT id, name, game_type, image, container_id, status, port, environment, volumes, created_at, updated_at FROM gameservers WHERE id = ?`, id)
	server, err := dm.scanGameServer(row)
	if err == sql.ErrNoRows {
		return nil, &DatabaseError{"get", fmt.Sprintf("gameserver %s not found", id), nil}
	}
	if err != nil {
		return nil, &DatabaseError{"get", fmt.Sprintf("failed to query gameserver %s", id), err}
	}
	return server, nil
}

func (dm *DatabaseManager) UpdateGameServer(server *GameServer) error {
	envJSON, _ := json.Marshal(server.Environment)
	volumesJSON, _ := json.Marshal(server.Volumes)
	server.UpdatedAt = time.Now()

	result, err := dm.db.Exec(`UPDATE gameservers SET name = ?, game_type = ?, image = ?, container_id = ?, status = ?, port = ?, environment = ?, volumes = ?, updated_at = ? WHERE id = ?`,
		server.Name, server.GameType, server.Image, server.ContainerID, server.Status, server.Port, string(envJSON), string(volumesJSON), server.UpdatedAt, server.ID)

	if err != nil {
		return &DatabaseError{"update", fmt.Sprintf("failed to update gameserver %s", server.ID), err}
	}

	if rowsAffected, _ := result.RowsAffected(); rowsAffected == 0 {
		return &DatabaseError{"update", fmt.Sprintf("gameserver %s not found", server.ID), nil}
	}
	return nil
}

func (dm *DatabaseManager) DeleteGameServer(id string) error {
	result, err := dm.db.Exec(`DELETE FROM gameservers WHERE id = ?`, id)
	if err != nil {
		return &DatabaseError{"delete", fmt.Sprintf("failed to delete gameserver %s", id), err}
	}
	if rowsAffected, _ := result.RowsAffected(); rowsAffected == 0 {
		return &DatabaseError{"delete", fmt.Sprintf("gameserver %s not found", id), nil}
	}
	return nil
}

func (dm *DatabaseManager) ListGameServers() ([]*GameServer, error) {
	rows, err := dm.db.Query(`SELECT id, name, game_type, image, container_id, status, port, environment, volumes, created_at, updated_at FROM gameservers ORDER BY created_at DESC`)
	if err != nil {
		return nil, &DatabaseError{"list", "failed to query gameservers", err}
	}
	defer rows.Close()

	var servers []*GameServer
	for rows.Next() {
		server, err := dm.scanGameServer(rows)
		if err != nil {
			return nil, &DatabaseError{"list", "failed to scan gameserver row", err}
		}
		servers = append(servers, server)
	}
	return servers, rows.Err()
}

func (dm *DatabaseManager) GetGameServerByContainerID(containerID string) (*GameServer, error) {
	row := dm.db.QueryRow(`SELECT id, name, game_type, image, container_id, status, port, environment, volumes, created_at, updated_at FROM gameservers WHERE container_id = ?`, containerID)
	server, err := dm.scanGameServer(row)
	if err == sql.ErrNoRows {
		return nil, &DatabaseError{"get_by_container", fmt.Sprintf("gameserver with container %s not found", containerID), nil}
	}
	if err != nil {
		return nil, &DatabaseError{"get_by_container", fmt.Sprintf("failed to query gameserver by container %s", containerID), err}
	}
	return server, nil
}

type GameServerService struct {
	db     *DatabaseManager
	docker DockerManagerInterface
}

func NewGameServerService(db *DatabaseManager, docker DockerManagerInterface) *GameServerService {
	return &GameServerService{db: db, docker: docker}
}

func (gss *GameServerService) CreateGameServer(server *GameServer) error {
	now := time.Now()
	server.CreatedAt, server.UpdatedAt, server.Status = now, now, StatusStopped

	if err := gss.db.CreateGameServer(server); err != nil {
		return err
	}
	if err := gss.docker.CreateContainer(server); err != nil {
		gss.db.DeleteGameServer(server.ID)
		return err
	}
	return gss.db.UpdateGameServer(server)
}

func (gss *GameServerService) execDockerOp(id string, op func(string) error, status GameServerStatus) error {
	server, err := gss.db.GetGameServer(id)
	if err != nil {
		return err
	}
	if err := op(server.ContainerID); err != nil {
		return err
	}
	server.Status, server.UpdatedAt = status, time.Now()
	return gss.db.UpdateGameServer(server)
}

func (gss *GameServerService) StartGameServer(id string) error {
	return gss.execDockerOp(id, gss.docker.StartContainer, StatusStarting)
}

func (gss *GameServerService) StopGameServer(id string) error {
	return gss.execDockerOp(id, gss.docker.StopContainer, StatusStopping)
}

func (gss *GameServerService) RestartGameServer(id string) error {
	return gss.execDockerOp(id, gss.docker.RestartContainer, StatusStarting)
}

func (gss *GameServerService) DeleteGameServer(id string) error {
	server, err := gss.db.GetGameServer(id)
	if err != nil {
		return err
	}
	if server.ContainerID != "" {
		gss.docker.RemoveContainer(server.ContainerID)
	}
	return gss.db.DeleteGameServer(id)
}

func (gss *GameServerService) syncStatus(server *GameServer) {
	if server.ContainerID != "" {
		if dockerStatus, err := gss.docker.GetContainerStatus(server.ContainerID); err == nil && server.Status != dockerStatus {
			server.Status, server.UpdatedAt = dockerStatus, time.Now()
			gss.db.UpdateGameServer(server)
		}
	}
}

func (gss *GameServerService) GetGameServer(id string) (*GameServer, error) {
	server, err := gss.db.GetGameServer(id)
	if err != nil {
		return nil, err
	}
	gss.syncStatus(server)
	return server, nil
}

func (gss *GameServerService) ListGameServers() ([]*GameServer, error) {
	servers, err := gss.db.ListGameServers()
	if err != nil {
		return nil, err
	}
	for _, server := range servers {
		gss.syncStatus(server)
	}
	return servers, nil
}

func (gss *GameServerService) GetGameServerLogs(id string, lines int) ([]string, error) {
	server, err := gss.db.GetGameServer(id)
	if err != nil {
		return nil, err
	}
	if server.ContainerID == "" {
		return []string{}, nil
	}
	return gss.docker.GetContainerLogs(server.ContainerID, lines)
}

func (gss *GameServerService) GetGameServerStats(id string) (*ContainerStats, error) {
	server, err := gss.db.GetGameServer(id)
	if err != nil {
		return nil, err
	}
	if server.ContainerID == "" {
		return nil, &DatabaseError{"stats", "container not created yet", nil}
	}
	return gss.docker.GetContainerStats(server.ContainerID)
}
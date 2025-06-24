package database

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"0xkowalskidev/gameservers/models"
)

// CreateGameserver inserts a new gameserver into the database
func (dm *DatabaseManager) CreateGameserver(server *models.Gameserver) error {
	envJSON, _ := json.Marshal(server.Environment)
	volumesJSON, _ := json.Marshal(server.Volumes)
	portMappingsJSON, _ := json.Marshal(server.PortMappings)

	_, err := dm.db.Exec(`INSERT INTO gameservers (id, name, game_id, container_id, status, port_mappings, memory_mb, cpu_cores, max_backups, environment, volumes, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		server.ID, server.Name, server.GameID, server.ContainerID, server.Status, string(portMappingsJSON), server.MemoryMB, server.CPUCores, server.MaxBackups, string(envJSON), string(volumesJSON), server.CreatedAt, server.UpdatedAt)

	if err != nil {
		return &models.DatabaseError{Op: "create_gameserver", Msg: fmt.Sprintf("failed to insert gameserver %s", server.Name), Err: err}
	}

	return nil
}

// GetGameserver retrieves a gameserver by ID
func (dm *DatabaseManager) GetGameserver(id string) (*models.Gameserver, error) {
	row := dm.db.QueryRow(`SELECT id, name, game_id, container_id, status, port_mappings, memory_mb, cpu_cores, max_backups, environment, volumes, created_at, updated_at FROM gameservers WHERE id = ?`, id)
	server, err := dm.scanGameserver(row)
	if err == sql.ErrNoRows {
		return nil, &models.DatabaseError{Op: "get_gameserver", Msg: fmt.Sprintf("gameserver %s not found", id), Err: nil}
	}
	if err != nil {
		return nil, &models.DatabaseError{Op: "get_gameserver", Msg: fmt.Sprintf("failed to query gameserver %s", id), Err: err}
	}
	return server, nil
}

// UpdateGameserver updates an existing gameserver
func (dm *DatabaseManager) UpdateGameserver(server *models.Gameserver) error {
	envJSON, _ := json.Marshal(server.Environment)
	volumesJSON, _ := json.Marshal(server.Volumes)
	portMappingsJSON, _ := json.Marshal(server.PortMappings)
	server.UpdatedAt = time.Now()

	result, err := dm.db.Exec(`UPDATE gameservers SET name = ?, game_id = ?, container_id = ?, status = ?, port_mappings = ?, memory_mb = ?, cpu_cores = ?, max_backups = ?, environment = ?, volumes = ?, updated_at = ? WHERE id = ?`,
		server.Name, server.GameID, server.ContainerID, server.Status, string(portMappingsJSON), server.MemoryMB, server.CPUCores, server.MaxBackups, string(envJSON), string(volumesJSON), server.UpdatedAt, server.ID)

	if err != nil {
		return &models.DatabaseError{Op: "update_gameserver", Msg: fmt.Sprintf("failed to update gameserver %s", server.ID), Err: err}
	}

	if rowsAffected, _ := result.RowsAffected(); rowsAffected == 0 {
		return &models.DatabaseError{Op: "update_gameserver", Msg: fmt.Sprintf("gameserver %s not found", server.ID), Err: nil}
	}
	return nil
}

// DeleteGameserver deletes a gameserver by ID
func (dm *DatabaseManager) DeleteGameserver(id string) error {
	result, err := dm.db.Exec(`DELETE FROM gameservers WHERE id = ?`, id)
	if err != nil {
		return &models.DatabaseError{Op: "delete_gameserver", Msg: fmt.Sprintf("failed to delete gameserver %s", id), Err: err}
	}
	if rowsAffected, _ := result.RowsAffected(); rowsAffected == 0 {
		return &models.DatabaseError{Op: "delete_gameserver", Msg: fmt.Sprintf("gameserver %s not found", id), Err: nil}
	}
	return nil
}

// ListGameservers retrieves all gameservers
func (dm *DatabaseManager) ListGameservers() ([]*models.Gameserver, error) {
	rows, err := dm.db.Query(`SELECT id, name, game_id, container_id, status, port_mappings, memory_mb, cpu_cores, max_backups, environment, volumes, created_at, updated_at FROM gameservers ORDER BY created_at DESC`)
	if err != nil {
		return nil, &models.DatabaseError{Op: "list_gameservers", Msg: "failed to query gameservers", Err: err}
	}
	defer rows.Close()

	var servers []*models.Gameserver
	for rows.Next() {
		server, err := dm.scanGameserver(rows)
		if err != nil {
			return nil, &models.DatabaseError{Op: "list_gameservers", Msg: "failed to scan gameserver row", Err: err}
		}
		servers = append(servers, server)
	}
	return servers, rows.Err()
}

// GetGameserverByContainerID retrieves a gameserver by container ID
func (dm *DatabaseManager) GetGameserverByContainerID(containerID string) (*models.Gameserver, error) {
	row := dm.db.QueryRow(`SELECT id, name, game_id, container_id, status, port_mappings, memory_mb, cpu_cores, max_backups, environment, volumes, created_at, updated_at FROM gameservers WHERE container_id = ?`, containerID)
	server, err := dm.scanGameserver(row)
	if err == sql.ErrNoRows {
		return nil, &models.DatabaseError{Op: "get_gameserver_by_container", Msg: fmt.Sprintf("gameserver with container %s not found", containerID), Err: nil}
	}
	if err != nil {
		return nil, &models.DatabaseError{Op: "get_gameserver_by_container", Msg: fmt.Sprintf("failed to query gameserver by container %s", containerID), Err: err}
	}
	return server, nil
}

// scanGameserver scans a database row into a Gameserver struct
func (dm *DatabaseManager) scanGameserver(row interface{ Scan(...interface{}) error }) (*models.Gameserver, error) {
	var server models.Gameserver
	var envJSON, volumesJSON, portMappingsJSON string

	err := row.Scan(&server.ID, &server.Name, &server.GameID, &server.ContainerID, &server.Status, &portMappingsJSON, &server.MemoryMB, &server.CPUCores, &server.MaxBackups, &envJSON, &volumesJSON, &server.CreatedAt, &server.UpdatedAt)
	if err != nil {
		return nil, err
	}

	json.Unmarshal([]byte(envJSON), &server.Environment)
	json.Unmarshal([]byte(volumesJSON), &server.Volumes)
	json.Unmarshal([]byte(portMappingsJSON), &server.PortMappings)
	return &server, nil
}
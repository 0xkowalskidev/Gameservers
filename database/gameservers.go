package database

import (
	"fmt"

	"gorm.io/gorm"

	"0xkowalskidev/gameservers/models"
)

// databaseError represents a database operation error (simplified local version)
type databaseError struct {
	Op  string
	Msg string
	Err error
}

func (e *databaseError) Error() string {
	if e.Err != nil {
		return e.Op + ": " + e.Msg + ": " + e.Err.Error()
	}
	return e.Op + ": " + e.Msg
}

// CreateGameserver inserts a new gameserver into the database
func (dm *DatabaseManager) CreateGameserver(server *models.Gameserver) error {
	if err := dm.db.Create(server).Error; err != nil {
		return &databaseError{Op: "create_gameserver", Msg: fmt.Sprintf("failed to insert gameserver %s", server.Name), Err: err}
	}
	return nil
}

// GetGameserver retrieves a gameserver by ID
func (dm *DatabaseManager) GetGameserver(id string) (*models.Gameserver, error) {
	var server models.Gameserver
	if err := dm.db.First(&server, "id = ?", id).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, &databaseError{Op: "get_gameserver", Msg: fmt.Sprintf("gameserver %s not found", id), Err: nil}
		}
		return nil, &databaseError{Op: "get_gameserver", Msg: fmt.Sprintf("failed to query gameserver %s", id), Err: err}
	}
	return &server, nil
}

// UpdateGameserver updates an existing gameserver
func (dm *DatabaseManager) UpdateGameserver(server *models.Gameserver) error {
	result := dm.db.Save(server)
	if result.Error != nil {
		return &databaseError{Op: "update_gameserver", Msg: fmt.Sprintf("failed to update gameserver %s", server.ID), Err: result.Error}
	}
	if result.RowsAffected == 0 {
		return &databaseError{Op: "update_gameserver", Msg: fmt.Sprintf("gameserver %s not found", server.ID), Err: nil}
	}
	return nil
}

// DeleteGameserver deletes a gameserver by ID
func (dm *DatabaseManager) DeleteGameserver(id string) error {
	result := dm.db.Delete(&models.Gameserver{}, "id = ?", id)
	if result.Error != nil {
		return &databaseError{Op: "delete_gameserver", Msg: fmt.Sprintf("failed to delete gameserver %s", id), Err: result.Error}
	}
	if result.RowsAffected == 0 {
		return &databaseError{Op: "delete_gameserver", Msg: fmt.Sprintf("gameserver %s not found", id), Err: nil}
	}
	return nil
}

// ListGameservers retrieves all gameservers
func (dm *DatabaseManager) ListGameservers() ([]*models.Gameserver, error) {
	var servers []*models.Gameserver
	if err := dm.db.Order("created_at DESC").Find(&servers).Error; err != nil {
		return nil, &databaseError{Op: "list_gameservers", Msg: "failed to query gameservers", Err: err}
	}
	return servers, nil
}

// GetGameserverByContainerID retrieves a gameserver by container ID
func (dm *DatabaseManager) GetGameserverByContainerID(containerID string) (*models.Gameserver, error) {
	var server models.Gameserver
	if err := dm.db.Where("container_id = ?", containerID).First(&server).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, &databaseError{Op: "get_gameserver_by_container", Msg: fmt.Sprintf("gameserver with container %s not found", containerID), Err: nil}
		}
		return nil, &databaseError{Op: "get_gameserver_by_container", Msg: fmt.Sprintf("failed to query gameserver by container %s", containerID), Err: err}
	}
	return &server, nil
}

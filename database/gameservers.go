package database

import (
	"fmt"

	"gorm.io/gorm"

	"0xkowalskidev/gameservers/models"
)

// CreateGameserver inserts a new gameserver into the database
func (dm *DatabaseManager) CreateGameserver(server *models.Gameserver) error {
	if err := dm.db.Create(server).Error; err != nil {
		return &models.DatabaseError{Op: "create_gameserver", Msg: fmt.Sprintf("failed to insert gameserver %s", server.Name), Err: err}
	}
	return nil
}

// GetGameserver retrieves a gameserver by ID
func (dm *DatabaseManager) GetGameserver(id string) (*models.Gameserver, error) {
	var server models.Gameserver
	if err := dm.db.First(&server, "id = ?", id).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, &models.DatabaseError{Op: "get_gameserver", Msg: fmt.Sprintf("gameserver %s not found", id), Err: nil}
		}
		return nil, &models.DatabaseError{Op: "get_gameserver", Msg: fmt.Sprintf("failed to query gameserver %s", id), Err: err}
	}
	return &server, nil
}

// UpdateGameserver updates an existing gameserver
func (dm *DatabaseManager) UpdateGameserver(server *models.Gameserver) error {
	result := dm.db.Save(server)
	if result.Error != nil {
		return &models.DatabaseError{Op: "update_gameserver", Msg: fmt.Sprintf("failed to update gameserver %s", server.ID), Err: result.Error}
	}
	if result.RowsAffected == 0 {
		return &models.DatabaseError{Op: "update_gameserver", Msg: fmt.Sprintf("gameserver %s not found", server.ID), Err: nil}
	}
	return nil
}

// DeleteGameserver deletes a gameserver by ID
func (dm *DatabaseManager) DeleteGameserver(id string) error {
	result := dm.db.Unscoped().Delete(&models.Gameserver{}, "id = ?", id)
	if result.Error != nil {
		return &models.DatabaseError{Op: "delete_gameserver", Msg: fmt.Sprintf("failed to delete gameserver %s", id), Err: result.Error}
	}
	if result.RowsAffected == 0 {
		return &models.DatabaseError{Op: "delete_gameserver", Msg: fmt.Sprintf("gameserver %s not found", id), Err: nil}
	}
	return nil
}

// ListGameservers retrieves all gameservers
func (dm *DatabaseManager) ListGameservers() ([]*models.Gameserver, error) {
	var servers []*models.Gameserver
	if err := dm.db.Order("created_at DESC").Find(&servers).Error; err != nil {
		return nil, &models.DatabaseError{Op: "list_gameservers", Msg: "failed to query gameservers", Err: err}
	}
	return servers, nil
}

// GetGameserverByContainerID retrieves a gameserver by container ID
func (dm *DatabaseManager) GetGameserverByContainerID(containerID string) (*models.Gameserver, error) {
	var server models.Gameserver
	if err := dm.db.Where("container_id = ?", containerID).First(&server).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, &models.DatabaseError{Op: "get_gameserver_by_container", Msg: fmt.Sprintf("gameserver with container %s not found", containerID), Err: nil}
		}
		return nil, &models.DatabaseError{Op: "get_gameserver_by_container", Msg: fmt.Sprintf("failed to query gameserver by container %s", containerID), Err: err}
	}
	return &server, nil
}

// CountGameserversByGameID counts gameservers using a specific game
func (dm *DatabaseManager) CountGameserversByGameID(gameID string) (int64, error) {
	var count int64
	if err := dm.db.Model(&models.Gameserver{}).Where("game_id = ?", gameID).Count(&count).Error; err != nil {
		return 0, &models.DatabaseError{Op: "count_gameservers_by_game", Msg: fmt.Sprintf("failed to count gameservers for game %s", gameID), Err: err}
	}
	return count, nil
}

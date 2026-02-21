package database

import (
	"fmt"

	"0xkowalskidev/gameservers/models"
)

// GetModsForGame retrieves all mods available for a specific game
func (dm *DatabaseManager) GetModsForGame(gameID string) ([]*models.Mod, error) {
	var mods []*models.Mod
	if err := dm.db.Where("game_id = ?", gameID).Order("name").Find(&mods).Error; err != nil {
		return nil, &models.DatabaseError{Op: "get_mods_for_game", Msg: fmt.Sprintf("failed to get mods for game %s", gameID), Err: err}
	}
	return mods, nil
}

// GetMod retrieves a mod by ID
func (dm *DatabaseManager) GetMod(id string) (*models.Mod, error) {
	var mod models.Mod
	if err := dm.db.First(&mod, "id = ?", id).Error; err != nil {
		return nil, &models.DatabaseError{Op: "get_mod", Msg: fmt.Sprintf("failed to get mod %s", id), Err: err}
	}
	return &mod, nil
}

// ListMods retrieves all mods
func (dm *DatabaseManager) ListMods() ([]*models.Mod, error) {
	var mods []*models.Mod
	if err := dm.db.Order("game_id, name").Find(&mods).Error; err != nil {
		return nil, &models.DatabaseError{Op: "list_mods", Msg: "failed to query mods", Err: err}
	}
	return mods, nil
}

// CreateMod creates a new mod
func (dm *DatabaseManager) CreateMod(mod *models.Mod) error {
	if err := dm.db.Create(mod).Error; err != nil {
		return &models.DatabaseError{Op: "create_mod", Msg: fmt.Sprintf("failed to create mod %s", mod.ID), Err: err}
	}
	return nil
}

// UpdateMod updates an existing mod
func (dm *DatabaseManager) UpdateMod(mod *models.Mod) error {
	if err := dm.db.Save(mod).Error; err != nil {
		return &models.DatabaseError{Op: "update_mod", Msg: fmt.Sprintf("failed to update mod %s", mod.ID), Err: err}
	}
	return nil
}

// DeleteMod deletes a mod by ID
func (dm *DatabaseManager) DeleteMod(id string) error {
	if err := dm.db.Delete(&models.Mod{}, "id = ?", id).Error; err != nil {
		return &models.DatabaseError{Op: "delete_mod", Msg: fmt.Sprintf("failed to delete mod %s", id), Err: err}
	}
	return nil
}

// DeleteModsForGame deletes all mods for a game
func (dm *DatabaseManager) DeleteModsForGame(gameID string) error {
	if err := dm.db.Delete(&models.Mod{}, "game_id = ?", gameID).Error; err != nil {
		return &models.DatabaseError{Op: "delete_mods_for_game", Msg: fmt.Sprintf("failed to delete mods for game %s", gameID), Err: err}
	}
	return nil
}

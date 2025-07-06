package handlers

import (
	"fmt"
	"net/http"

	. "0xkowalskidev/gameservers/errors"
	"0xkowalskidev/gameservers/models"
	"0xkowalskidev/gameservers/services"
	"github.com/go-chi/chi/v5"
	"github.com/rs/zerolog/log"
)

// BackupHandlers handles backup-related HTTP requests
type BackupHandlers struct {
	*BaseHandlers
	gameserverService services.GameserverServiceInterface
	backupService     models.BackupServiceInterface
	fileService       models.FileServiceInterface
}

// NewBackupHandlers creates new backup handlers
func NewBackupHandlers(base *BaseHandlers, gameserverService services.GameserverServiceInterface, backupService models.BackupServiceInterface, fileService models.FileServiceInterface) *BackupHandlers {
	return &BackupHandlers{
		BaseHandlers:      base,
		gameserverService: gameserverService,
		backupService:     backupService,
		fileService:       fileService,
	}
}

// RestoreGameserverBackup restores a gameserver from a backup
func (h *BackupHandlers) RestoreGameserverBackup(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	backupFilename := r.URL.Query().Get("backup")
	if backupFilename == "" {
		h.HandleError(w, r, BadRequest("backup parameter required"))
		return
	}

	gameserver, err := h.gameserverService.GetGameserver(id)
	if err != nil {
		h.HandleError(w, r, NotFound("Gameserver"))
		return
	}

	log.Info().Str("gameserver_id", id).Str("backup_filename", backupFilename).Msg("Restoring backup")

	if err := h.backupService.RestoreGameserverBackup(gameserver.ID, backupFilename); err != nil {
		h.HandleError(w, r, InternalError(err, "Failed to restore backup"))
		return
	}

	w.WriteHeader(http.StatusOK)
}

// CreateGameserverBackup creates a new backup
func (h *BackupHandlers) CreateGameserverBackup(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	log.Info().Str("gameserver_id", id).Msg("Creating backup")

	if err := h.backupService.CreateGameserverBackup(id); err != nil {
		h.HandleError(w, r, InternalError(err, "Failed to create backup"))
		return
	}

	w.WriteHeader(http.StatusOK)
}

// ListGameserverBackups displays all backups for a gameserver
func (h *BackupHandlers) ListGameserverBackups(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	gameserver, err := h.gameserverService.GetGameserver(id)
	if err != nil {
		h.HandleError(w, r, NotFound("Gameserver"))
		return
	}

	// Get backup files
	backups, err := h.backupService.ListGameserverBackups(id)
	if err != nil {
		h.HandleError(w, r, InternalError(err, "Failed to list backup files"))
		return
	}

	// Calculate remaining backups
	var remainingBackups int
	if gameserver.MaxBackups > 0 {
		remainingBackups = gameserver.MaxBackups - len(backups)
		if remainingBackups < 0 {
			remainingBackups = 0
		}
	}

	data := map[string]interface{}{
		"Gameserver":       gameserver,
		"Backups":          backups,
		"RemainingBackups": remainingBackups,
	}

	// Check if request is targeting a specific element
	target := r.Header.Get("HX-Target")
	templateName := "gameserver-backups.html"
	if target == "#backup-list" || r.URL.Query().Get("list") == "true" {
		templateName = "backup-list.html"
	}
	h.Render(w, r, templateName, data)
}

// DeleteGameserverBackup deletes a backup file
func (h *BackupHandlers) DeleteGameserverBackup(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	backupFilename := r.URL.Query().Get("backup")
	if backupFilename == "" {
		h.HandleError(w, r, BadRequest("backup parameter required"))
		return
	}

	gameserver, err := h.gameserverService.GetGameserver(id)
	if err != nil {
		h.HandleError(w, r, NotFound("Gameserver"))
		return
	}

	log.Info().Str("gameserver_id", id).Str("backup_filename", backupFilename).Msg("Deleting backup")

	// Delete the backup file from /data/backups
	backupPath := fmt.Sprintf("/data/backups/%s", backupFilename)
	if err := h.fileService.DeletePath(gameserver.ContainerID, backupPath); err != nil {
		h.HandleError(w, r, InternalError(err, "Failed to delete backup"))
		return
	}

	w.WriteHeader(http.StatusOK)
}

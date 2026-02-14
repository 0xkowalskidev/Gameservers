package handlers

import (
	"fmt"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/rs/zerolog/log"
)

// RestoreGameserverBackup restores a gameserver from a backup
func (h *Handlers) RestoreGameserverBackup(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	backupFilename, err := h.requireQueryParam(r, "backup")
	if err != nil {
		HandleError(w, err, "restore_backup")
		return
	}

	gameserver, ok := h.getGameserver(w, id)
	if !ok {
		return
	}

	log.Info().Str("gameserver_id", id).Str("backup_filename", backupFilename).Msg("Restoring backup")

	if err := h.service.RestoreGameserverBackup(gameserver.ID, backupFilename); err != nil {
		HandleError(w, InternalError(err, "Failed to restore backup"), "restore_backup")
		return
	}

	w.WriteHeader(http.StatusOK)
}

// CreateGameserverBackup creates a new backup
func (h *Handlers) CreateGameserverBackup(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	log.Info().Str("gameserver_id", id).Msg("Creating backup")

	if err := h.service.CreateGameserverBackup(id); err != nil {
		HandleError(w, InternalError(err, "Failed to create backup"), "create_backup")
		return
	}

	w.WriteHeader(http.StatusOK)
}

// ListGameserverBackups displays all backups for a gameserver
func (h *Handlers) ListGameserverBackups(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	gameserver, ok := h.getGameserver(w, id)
	if !ok {
		return
	}

	// Get backup files
	backups, err := h.service.ListGameserverBackups(id)
	if err != nil {
		HandleError(w, InternalError(err, "Failed to list backup files"), "list_backups")
		return
	}

	data := map[string]interface{}{
		"Gameserver":   gameserver,
		"Backups":      backups,
		"GameserverID": id,
		"BackupCount":  len(backups),
		"MaxBackups":   gameserver.MaxBackups,
	}

	// Special case: if targeting #backup-list specifically, return just the list
	target := r.Header.Get("HX-Target")
	if target == "backup-list" || r.URL.Query().Get("list") == "true" {
		if err := h.tmpl.ExecuteTemplate(w, "backup-list.html", data); err != nil {
			HandleError(w, InternalError(err, "Failed to render backup list"), "list_backups")
		}
		return
	}

	h.renderGameserver(w, r, gameserver, "backups", "gameserver-backups.html", data)
}

// DeleteGameserverBackup deletes a backup file
func (h *Handlers) DeleteGameserverBackup(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	backupFilename, err := h.requireQueryParam(r, "backup")
	if err != nil {
		HandleError(w, err, "delete_backup")
		return
	}

	gameserver, ok := h.getGameserver(w, id)
	if !ok {
		return
	}

	log.Info().Str("gameserver_id", id).Str("backup_filename", backupFilename).Msg("Deleting backup")

	// Delete the backup file from /data/backups
	backupPath := fmt.Sprintf("/data/backups/%s", backupFilename)
	if err := h.docker.DeletePath(gameserver.ContainerID, backupPath); err != nil {
		HandleError(w, InternalError(err, "Failed to delete backup"), "delete_backup")
		return
	}

	// Return updated backup list for HTMX swap
	backups, err := h.service.ListGameserverBackups(id)
	if err != nil {
		HandleError(w, InternalError(err, "Failed to list backup files"), "delete_backup")
		return
	}

	data := map[string]interface{}{
		"Gameserver":   gameserver,
		"Backups":      backups,
		"GameserverID": id,
		"BackupCount":  len(backups),
		"MaxBackups":   gameserver.MaxBackups,
	}

	if err := h.tmpl.ExecuteTemplate(w, "backup-list.html", data); err != nil {
		HandleError(w, InternalError(err, "Failed to render backup list"), "delete_backup")
	}
}

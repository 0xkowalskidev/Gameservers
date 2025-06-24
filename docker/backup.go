package docker

import (
	"fmt"
	"strings"
	"time"

	"github.com/rs/zerolog/log"
)

// CreateBackup creates a backup of gameserver files
func (d *DockerManager) CreateBackup(containerID, gameserverName string) error {
	// Generate timestamped backup filename
	timestamp := time.Now().Format("2006-01-02_15-04-05")
	backupFilename := fmt.Sprintf("backup-%s.tar.gz", timestamp)
	
	log.Info().Str("container_id", containerID).Str("backup_file", backupFilename).Msg("Creating backup")
	
	// First ensure the backups directory exists
	if err := d.execCommandSimple(containerID, []string{"mkdir", "-p", "/data/backups"}, "create_backup_dir"); err != nil {
		return err
	}
	
	// Create backup using tar inside the existing container
	cmd := []string{"tar", "-czf", fmt.Sprintf("/data/backups/%s", backupFilename), 
		"-C", "/data/server", "."}
	
	if err := d.execCommandSimple(containerID, cmd, "create_backup"); err != nil {
		return err
	}
	
	log.Info().Str("container_id", containerID).Str("backup_file", backupFilename).Msg("Backup created successfully")
	return nil
}

// CleanupOldBackups removes old backup files based on maxBackups limit
func (d *DockerManager) CleanupOldBackups(containerID string, maxBackups int) error {
	if maxBackups <= 0 {
		// Unlimited backups, no cleanup needed
		return nil
	}
	
	log.Info().Str("container_id", containerID).Int("max_backups", maxBackups).Msg("Cleaning up old backups")
	
	// List all backup files sorted by modification time (newest first)
	cmd := []string{"sh", "-c", "find /data/backups -name '*.tar.gz' -type f -printf '%T@ %p\\n' | sort -nr | cut -d' ' -f2-"}
	output, err := d.ExecCommand(containerID, cmd)
	if err != nil {
		return &DockerError{
			Op:  "list_backups",
			Msg: fmt.Sprintf("failed to list backups for cleanup in container %s", containerID),
			Err: err,
		}
	}
	
	backupFiles := strings.Split(strings.TrimSpace(string(output)), "\n")
	if len(backupFiles) <= maxBackups {
		// No cleanup needed
		log.Info().Str("container_id", containerID).Int("backup_count", len(backupFiles)).Int("max_backups", maxBackups).Msg("No backup cleanup needed")
		return nil
	}
	
	// Delete the oldest backups (files beyond maxBackups limit)
	filesToDelete := backupFiles[maxBackups:]
	for _, file := range filesToDelete {
		if strings.TrimSpace(file) == "" {
			continue
		}
		
		log.Info().Str("container_id", containerID).Str("backup_file", file).Msg("Deleting old backup")
		_, err := d.ExecCommand(containerID, []string{"rm", "-f", file})
		if err != nil {
			log.Error().Err(err).Str("container_id", containerID).Str("backup_file", file).Msg("Failed to delete old backup")
			// Continue with other files even if one fails
		}
	}
	
	log.Info().Str("container_id", containerID).Int("deleted_count", len(filesToDelete)).Msg("Backup cleanup completed")
	return nil
}

// RestoreBackup restores a backup to the gameserver
func (d *DockerManager) RestoreBackup(containerID, backupFilename string) error {
	log.Info().Str("container_id", containerID).Str("backup_file", backupFilename).Msg("Restoring backup")
	
	// Create temporary directory for backups during restore
	if err := d.execCommandSimple(containerID, []string{"mkdir", "-p", "/tmp/backups"}, "create_temp_dir"); err != nil {
		return err
	}
	
	// Clear server directory
	if err := d.execCommandSimple(containerID, []string{"sh", "-c", "find /data/server -mindepth 1 -delete"}, "clear_server_dir"); err != nil {
		return err
	}
	
	// Extract the backup
	backupPath := fmt.Sprintf("/data/backups/%s", backupFilename)
	if err := d.execCommandSimple(containerID, []string{"tar", "-xzf", backupPath, "-C", "/data/server"}, "extract_backup"); err != nil {
		return err
	}
	
	// Clean up temporary directory
	_, err := d.ExecCommand(containerID, []string{"rm", "-rf", "/tmp/backups"})
	if err != nil {
		log.Warn().Err(err).Msg("Failed to clean up temporary backup directory")
	}
	
	log.Info().Str("container_id", containerID).Str("backup_file", backupFilename).Msg("Backup restored successfully")
	return nil
}
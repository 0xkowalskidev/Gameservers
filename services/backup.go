package services

import (
	"strings"

	"0xkowalskidev/gameservers/models"
)

// BackupService handles backup operations
type BackupService struct {
	docker models.DockerManagerInterface
	db     models.DatabaseInterface
}

// NewBackupService creates a new backup service
func NewBackupService(docker models.DockerManagerInterface, db models.DatabaseInterface) models.BackupServiceInterface {
	return &BackupService{
		docker: docker,
		db:     db,
	}
}

// CreateGameserverBackup creates a backup for a gameserver
func (s *BackupService) CreateGameserverBackup(gameserverID string) error {
	gameserver, err := s.db.GetGameserver(gameserverID)
	if err != nil {
		return err
	}
	return s.docker.CreateBackup(gameserver.ContainerID, gameserver.Name)
}

// RestoreGameserverBackup restores a gameserver from backup
func (s *BackupService) RestoreGameserverBackup(gameserverID, backupFilename string) error {
	gameserver, err := s.db.GetGameserver(gameserverID)
	if err != nil {
		return err
	}
	return s.docker.RestoreBackup(gameserver.ContainerID, backupFilename)
}

// ListGameserverBackups lists all backups for a gameserver
func (s *BackupService) ListGameserverBackups(gameserverID string) ([]*models.FileInfo, error) {
	gameserver, err := s.db.GetGameserver(gameserverID)
	if err != nil {
		return nil, err
	}

	// List files in /data/backups and filter for .tar.gz files
	files, err := s.docker.ListFiles(gameserver.ContainerID, "/data/backups")
	if err != nil {
		return nil, err
	}

	// Filter for backup files
	var backups []*models.FileInfo
	for _, file := range files {
		if !file.IsDir && strings.HasSuffix(strings.ToLower(file.Name), ".tar.gz") {
			backups = append(backups, file)
		}
	}

	return backups, nil
}
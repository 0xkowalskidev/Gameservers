package services

import (
	"io"

	"0xkowalskidev/gameservers/models"
)

// FileService handles file operations
type FileService struct {
	docker models.DockerManagerInterface
}

// NewFileService creates a new file service
func NewFileService(docker models.DockerManagerInterface) models.FileServiceInterface {
	return &FileService{
		docker: docker,
	}
}

// ListFiles lists files in a container directory
func (s *FileService) ListFiles(containerID string, path string) ([]*models.FileInfo, error) {
	return s.docker.ListFiles(containerID, path)
}

// ReadFile reads a file from a container
func (s *FileService) ReadFile(containerID string, path string) ([]byte, error) {
	return s.docker.ReadFile(containerID, path)
}

// WriteFile writes a file to a container
func (s *FileService) WriteFile(containerID string, path string, content []byte) error {
	return s.docker.WriteFile(containerID, path, content)
}

// CreateDirectory creates a directory in a container
func (s *FileService) CreateDirectory(containerID string, path string) error {
	return s.docker.CreateDirectory(containerID, path)
}

// DeletePath deletes a file or directory in a container
func (s *FileService) DeletePath(containerID string, path string) error {
	return s.docker.DeletePath(containerID, path)
}

// DownloadFile downloads a file from a container
func (s *FileService) DownloadFile(containerID string, path string) (io.ReadCloser, error) {
	return s.docker.DownloadFile(containerID, path)
}

// RenameFile renames a file in a container
func (s *FileService) RenameFile(containerID string, oldPath string, newPath string) error {
	return s.docker.RenameFile(containerID, oldPath, newPath)
}

// UploadFile uploads a file to a container
func (s *FileService) UploadFile(containerID string, destPath string, reader io.Reader) error {
	return s.docker.UploadFile(containerID, destPath, reader)
}
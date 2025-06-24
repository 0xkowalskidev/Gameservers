package handlers

import (
	"archive/tar"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/rs/zerolog/log"
)

// GameserverFiles displays the file manager interface
func (h *Handlers) GameserverFiles(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	gameserver, ok := h.getGameserver(w, id)
	if !ok {
		return
	}

	// Get root directory listing
	files, err := h.service.ListFiles(gameserver.ContainerID, "/data/server")
	if err != nil {
		log.Error().Err(err).Str("gameserver_id", id).Msg("Failed to list files")
	}

	data := map[string]interface{}{"Files": files, "CurrentPath": "/data/server"}
	h.renderGameserverPageOrPartial(w, r, gameserver, "files", "gameserver-files.html", data)
}

// BrowseGameserverFiles returns file listing for a specific path (HTMX)
func (h *Handlers) BrowseGameserverFiles(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	path := r.URL.Query().Get("path")
	if path == "" {
		path = "/data/server"
	}
	path = sanitizePath(path)

	gameserver, ok := h.getGameserver(w, id)
	if !ok {
		return
	}

	files, err := h.service.ListFiles(gameserver.ContainerID, path)
	if err != nil {
		HandleError(w, InternalError(err, "Failed to list files"), "browse_files")
		return
	}

	data := map[string]interface{}{"Files": files, "CurrentPath": path}
	data["Gameserver"] = gameserver
	if err := h.tmpl.ExecuteTemplate(w, "file-browser.html", data); err != nil {
		HandleError(w, InternalError(err, "Failed to render file browser"), "browse_files")
	}
}

// GameserverFileContent returns file content for editing (JSON API)
func (h *Handlers) GameserverFileContent(w http.ResponseWriter, r *http.Request) {
	// Set content type early to ensure consistent responses
	w.Header().Set("Content-Type", "application/json")

	// Get gameserver ID
	id := chi.URLParam(r, "id")
	if id == "" {
		json.NewEncoder(w).Encode(map[string]interface{}{
			"Supported": false,
			"Error":     "Missing gameserver ID",
		})
		return
	}

	// Get file path
	path := r.URL.Query().Get("path")
	if path == "" {
		json.NewEncoder(w).Encode(map[string]interface{}{
			"Supported": false,
			"Error":     "Missing file path",
		})
		return
	}

	// Sanitize path
	path = sanitizePath(path)

	// Check if file is editable
	if !isEditableFile(path) {
		json.NewEncoder(w).Encode(map[string]interface{}{
			"Path":      path,
			"Content":   "",
			"Supported": false,
		})
		return
	}

	// Get gameserver
	gameserver, err := h.service.GetGameserver(id)
	if err != nil {
		log.Error().Err(err).Str("id", id).Msg("Failed to get gameserver")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"Supported": false,
			"Error":     "Gameserver not found",
		})
		return
	}

	// Use a safer approach to read the file
	// Instead of using ExecCommand with cat, use docker cp to copy the file out
	reader, err := h.service.DownloadFile(gameserver.ContainerID, path)
	if err != nil {
		log.Error().Err(err).Str("path", path).Msg("Failed to download file for reading")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"Path":      path,
			"Content":   "",
			"Supported": false,
			"Error":     "Failed to read file",
		})
		return
	}
	defer reader.Close()

	// Extract file from tar archive
	tarReader := tar.NewReader(reader)
	header, err := tarReader.Next()
	if err != nil {
		log.Error().Err(err).Str("path", path).Msg("Failed to read tar header")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"Path":      path,
			"Content":   "",
			"Supported": false,
			"Error":     "Failed to read file archive",
		})
		return
	}

	// Read file content with size limit
	if header.Size > h.maxFileEditSize {
		json.NewEncoder(w).Encode(map[string]interface{}{
			"Path":      path,
			"Content":   "",
			"Supported": false,
			"Error":     fmt.Sprintf("File too large to edit (max %s)", formatFileSize(h.maxFileEditSize)),
		})
		return
	}

	// Read content
	content := make([]byte, header.Size)
	_, err = io.ReadFull(tarReader, content)
	if err != nil {
		log.Error().Err(err).Str("path", path).Msg("Failed to read file content")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"Path":      path,
			"Content":   "",
			"Supported": false,
			"Error":     "Failed to read file content",
		})
		return
	}

	// Success response
	json.NewEncoder(w).Encode(map[string]interface{}{
		"Path":      path,
		"Content":   string(content),
		"Supported": true,
	})
}

// SaveGameserverFile saves file content (JSON API)
func (h *Handlers) SaveGameserverFile(w http.ResponseWriter, r *http.Request) {
	// Set content type early
	w.Header().Set("Content-Type", "application/json")

	// Get gameserver ID
	id := chi.URLParam(r, "id")
	if id == "" {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{
			"status": "error",
			"error":  "Missing gameserver ID",
		})
		return
	}

	// Parse form data
	if err := r.ParseForm(); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{
			"status": "error",
			"error":  "Invalid form data",
		})
		return
	}

	// Get path and content
	path := r.FormValue("path")
	if path == "" {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{
			"status": "error",
			"error":  "Missing file path",
		})
		return
	}

	content := r.FormValue("content")

	// Sanitize path
	path = sanitizePath(path)

	// Verify it's an editable file
	if !isEditableFile(path) {
		w.WriteHeader(http.StatusForbidden)
		json.NewEncoder(w).Encode(map[string]string{
			"status": "error",
			"error":  "File type not editable",
		})
		return
	}

	// Get gameserver
	gameserver, err := h.service.GetGameserver(id)
	if err != nil {
		log.Error().Err(err).Str("id", id).Msg("Failed to get gameserver")
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(map[string]string{
			"status": "error",
			"error":  "Gameserver not found",
		})
		return
	}

	// Size limit check
	contentBytes := []byte(content)
	if int64(len(contentBytes)) > h.maxFileEditSize {
		w.WriteHeader(http.StatusRequestEntityTooLarge)
		json.NewEncoder(w).Encode(map[string]string{
			"status": "error",
			"error":  fmt.Sprintf("File content too large (max %s)", formatFileSize(h.maxFileEditSize)),
		})
		return
	}

	// Write file
	if err := h.service.WriteFile(gameserver.ContainerID, path, contentBytes); err != nil {
		log.Error().Err(err).Str("path", path).Msg("Failed to write file")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{
			"status": "error",
			"error":  "Failed to save file",
		})
		return
	}

	// Success response
	json.NewEncoder(w).Encode(map[string]string{
		"status": "saved",
	})
}

// DownloadGameserverFile downloads a file
func (h *Handlers) DownloadGameserverFile(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	path, err := h.requireQueryParam(r, "path")
	if err != nil {
		HandleError(w, err, "download_file")
		return
	}

	// Sanitize path
	path = sanitizePath(path)

	gameserver, ok := h.getGameserver(w, id)
	if !ok {
		return
	}

	// Use DownloadFile which supports both server and backup paths
	log.Info().Str("path", path).Str("container_id", gameserver.ContainerID).Msg("Attempting to download file")
	reader, err := h.service.DownloadFile(gameserver.ContainerID, path)
	if err != nil {
		log.Error().Err(err).Str("path", path).Str("container_id", gameserver.ContainerID).Msg("Download file failed")
		HandleError(w, InternalError(err, "Failed to download file"), "download_file")
		return
	}
	defer reader.Close()

	// Extract filename from path
	filename := filepath.Base(path)

	// The reader contains a tar archive, extract the file
	tarReader := tar.NewReader(reader)

	// Read the first (and should be only) file from the tar
	header, err := tarReader.Next()
	if err != nil {
		HandleError(w, InternalError(err, "Failed to read file from archive"), "download_file")
		return
	}

	// Set headers for download
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", filename))
	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Content-Length", strconv.FormatInt(header.Size, 10))

	// Stream the actual file content (not the tar archive)
	if _, err := io.Copy(w, tarReader); err != nil {
		log.Error().Err(err).Str("path", path).Msg("Failed to stream file content")
	}
}

// CreateGameserverFile creates a new file or directory
func (h *Handlers) CreateGameserverFile(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if err := h.validateFormFields(r, "path", "name"); err != nil {
		HandleError(w, err, "create_file")
		return
	}

	path := r.FormValue("path")
	name := r.FormValue("name")
	isDir := r.FormValue("type") == "directory"

	// Sanitize inputs
	path = sanitizePath(path)
	fullPath := filepath.Join(path, name)

	gameserver, ok := h.getGameserver(w, id)
	if !ok {
		return
	}

	var err error
	if isDir {
		err = h.service.CreateDirectory(gameserver.ContainerID, fullPath)
	} else {
		// Create empty file
		err = h.service.WriteFile(gameserver.ContainerID, fullPath, []byte(""))
	}

	if err != nil {
		HandleError(w, InternalError(err, "Failed to create file/directory"), "create_file")
		return
	}

	// Return updated file listing
	h.BrowseGameserverFiles(w, r)
}

// DeleteGameserverFile deletes a file or directory
func (h *Handlers) DeleteGameserverFile(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	path, err := h.requireQueryParam(r, "path")
	if err != nil {
		HandleError(w, err, "delete_file")
		return
	}

	// Sanitize path
	path = sanitizePath(path)

	gameserver, ok := h.getGameserver(w, id)
	if !ok {
		return
	}

	if err := h.service.DeletePath(gameserver.ContainerID, path); err != nil {
		HandleError(w, InternalError(err, "Failed to delete file/directory"), "delete_file")
		return
	}

	w.WriteHeader(http.StatusOK)
}

// RenameGameserverFile renames a file or directory
func (h *Handlers) RenameGameserverFile(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if err := h.validateFormFields(r, "old_path", "new_name"); err != nil {
		HandleError(w, err, "rename_file")
		return
	}

	oldPath := r.FormValue("old_path")
	newName := r.FormValue("new_name")

	// Sanitize paths
	oldPath = sanitizePath(oldPath)
	newPath := sanitizePath(filepath.Join(filepath.Dir(oldPath), newName))

	gameserver, ok := h.getGameserver(w, id)
	if !ok {
		return
	}

	if err := h.service.RenameFile(gameserver.ContainerID, oldPath, newPath); err != nil {
		HandleError(w, InternalError(err, "Failed to rename file"), "rename_file")
		return
	}

	// Return updated file listing
	h.BrowseGameserverFiles(w, r)
}

// UploadGameserverFile handles file uploads
func (h *Handlers) UploadGameserverFile(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	// Parse multipart form with configurable limit
	if err := r.ParseMultipartForm(h.maxUploadSize); err != nil {
		HandleError(w, BadRequest("Invalid upload format"), "upload_file")
		return
	}

	// Get the destination path
	destPath := r.FormValue("path")
	if destPath == "" {
		destPath = "/data/server"
	}
	destPath = sanitizePath(destPath)

	// Get the uploaded file
	file, header, err := r.FormFile("file")
	if err != nil {
		HandleError(w, BadRequest("No file provided"), "upload_file")
		return
	}
	defer file.Close()

	// Validate file size
	if header.Size > h.maxUploadSize {
		HandleError(w, BadRequest("File too large (max %s)", formatFileSize(h.maxUploadSize)), "upload_file")
		return
	}

	gameserver, ok := h.getGameserver(w, id)
	if !ok {
		return
	}

	// Create a tar archive for the file
	buf := new(bytes.Buffer)
	tw := tar.NewWriter(buf)

	// Add file to tar archive
	hdr := &tar.Header{
		Name: header.Filename,
		Mode: 0644,
		Size: header.Size,
	}

	if err := tw.WriteHeader(hdr); err != nil {
		HandleError(w, InternalError(err, "Failed to create archive"), "upload_file")
		return
	}

	if _, err := io.Copy(tw, file); err != nil {
		HandleError(w, InternalError(err, "Failed to write file"), "upload_file")
		return
	}

	if err := tw.Close(); err != nil {
		HandleError(w, InternalError(err, "Failed to close archive"), "upload_file")
		return
	}

	// Upload file to container
	if err := h.service.UploadFile(gameserver.ContainerID, destPath, bytes.NewReader(buf.Bytes())); err != nil {
		HandleError(w, InternalError(err, "Failed to upload file"), "upload_file")
		return
	}

	// Return updated file listing
	h.BrowseGameserverFiles(w, r)
}

// Helper functions

func sanitizePath(path string) string {
	// Clean the path
	path = filepath.Clean(path)

	// If path is empty or just "/", use server directory
	if path == "" || path == "/" {
		return "/data/server"
	}

	// Ensure path is absolute
	if !filepath.IsAbs(path) {
		path = "/" + path
	}

	// Handle backup paths - they should remain as-is if they're already valid backup paths
	if strings.HasPrefix(path, "/data/backups/") {
		// For backup paths, just clean and return
		return filepath.Clean(path)
	}

	// Handle server paths
	const serverDir = "/data/server"
	if strings.HasPrefix(path, serverDir) {
		// Already a valid server path, just clean and return
		return filepath.Clean(path)
	}

	// If user is trying to access parent directories, return server root
	if strings.HasPrefix(path, "/..") || path == ".." {
		return serverDir
	}

	// Otherwise, treat as relative to server directory
	path = filepath.Join(serverDir, path)

	// Clean again to resolve any .. sequences
	path = filepath.Clean(path)

	// Final check - ensure we're still within /data/server
	if !strings.HasPrefix(path, serverDir) {
		return serverDir
	}

	return path
}

func isEditableFile(filename string) bool {
	// Get file extension
	ext := strings.ToLower(filepath.Ext(filename))

	// Whitelist of editable file extensions
	editableExtensions := map[string]bool{
		".txt":          true,
		".json":         true,
		".xml":          true,
		".yaml":         true,
		".yml":          true,
		".toml":         true,
		".ini":          true,
		".conf":         true,
		".config":       true,
		".cfg":          true,
		".properties":   true,
		".log":          true,
		".md":           true,
		".js":           true,
		".ts":           true,
		".html":         true,
		".htm":          true,
		".css":          true,
		".scss":         true,
		".less":         true,
		".sql":          true,
		".sh":           true,
		".bash":         true,
		".bat":          true,
		".cmd":          true,
		".ps1":          true,
		".py":           true,
		".go":           true,
		".java":         true,
		".c":            true,
		".cpp":          true,
		".h":            true,
		".hpp":          true,
		".cs":           true,
		".php":          true,
		".rb":           true,
		".pl":           true,
		".r":            true,
		".lua":          true,
		".dockerfile":   true,
		".dockerignore": true,
		".gitignore":    true,
		".env":          true,
		".example":      true,
		"":              true, // Files without extension (like README, LICENSE)
	}

	// Special cases for files without extension that are typically text
	if ext == "" {
		baseName := strings.ToLower(filepath.Base(filename))
		textFiles := map[string]bool{
			"readme":       true,
			"license":      true,
			"changelog":    true,
			"authors":      true,
			"contributors": true,
			"copying":      true,
			"install":      true,
			"news":         true,
			"todo":         true,
			"makefile":     true,
			"dockerfile":   true,
			"vagrantfile":  true,
		}

		if textFiles[baseName] {
			return true
		}
	}

	return editableExtensions[ext]
}

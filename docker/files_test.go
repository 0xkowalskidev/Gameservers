package docker

import (
	"fmt"
	"io"
	"strings"
	"testing"
)

func TestMockDockerManager_ListFiles(t *testing.T) {
	tests := []struct {
		name        string
		containerID string
		path        string
		shouldFail  bool
		expectError bool
		minFiles    int
	}{
		{
			name:        "list server files",
			containerID: "container-123",
			path:        "/data/server",
			shouldFail:  false,
			expectError: false,
			minFiles:    2, // server.properties and world directory
		},
		{
			name:        "list backup files",
			containerID: "container-123",
			path:        "/data/backups",
			shouldFail:  false,
			expectError: false,
			minFiles:    0, // Initially no backups
		},
		{
			name:        "list files failure",
			containerID: "container-123",
			path:        "/data/server",
			shouldFail:  true,
			expectError: true,
			minFiles:    0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := NewMockDockerManager()
			if tt.shouldFail {
				mock.SetShouldFail("list_files", true)
			}

			// Add some backups if testing backup path
			if strings.Contains(tt.path, "backup") {
				mock.CreateBackup(tt.containerID, "test-server")
			}

			files, err := mock.ListFiles(tt.containerID, tt.path)

			if tt.expectError {
				if err == nil {
					t.Error("Expected error but got none")
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
				}

				if len(files) < tt.minFiles {
					t.Errorf("Expected at least %d files, got %d", tt.minFiles, len(files))
				}

				// Verify file structure
				for _, file := range files {
					if file.Name == "" {
						t.Error("File name should not be empty")
					}
					if file.Path == "" {
						t.Error("File path should not be empty")
					}
					if file.Modified == "" {
						t.Error("File modified time should not be empty")
					}
				}
			}
		})
	}
}

func TestMockDockerManager_ReadFile(t *testing.T) {
	mock := NewMockDockerManager()
	containerID := "container-123"
	filePath := "/data/server/server.properties"

	// Test reading non-existent file (should return default content)
	content, err := mock.ReadFile(containerID, filePath)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	if string(content) != "mock file content" {
		t.Errorf("Expected default content, got: %s", string(content))
	}

	// Write a file and then read it
	testContent := []byte("server-name=My Server\ndifficulty=normal\n")
	err = mock.WriteFile(containerID, filePath, testContent)
	if err != nil {
		t.Fatalf("Failed to write file: %v", err)
	}

	content, err = mock.ReadFile(containerID, filePath)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	if string(content) != string(testContent) {
		t.Errorf("Expected %s, got %s", string(testContent), string(content))
	}

	// Test read failure
	mock.SetShouldFail("read_file", true)
	_, err = mock.ReadFile(containerID, filePath)
	if err == nil {
		t.Error("Expected error but got none")
	}
}

func TestMockDockerManager_WriteFile(t *testing.T) {
	mock := NewMockDockerManager()
	containerID := "container-123"
	filePath := "/data/server/config.yml"
	content := []byte("gamemode: survival\nspawn-protection: 16\n")

	// Test successful write
	err := mock.WriteFile(containerID, filePath, content)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	// Verify content was written
	readContent, err := mock.ReadFile(containerID, filePath)
	if err != nil {
		t.Errorf("Failed to read back content: %v", err)
	}
	if string(readContent) != string(content) {
		t.Errorf("Content mismatch. Expected: %s, Got: %s", string(content), string(readContent))
	}

	// Test write failure
	mock.SetShouldFail("write_file", true)
	err = mock.WriteFile(containerID, filePath, content)
	if err == nil {
		t.Error("Expected error but got none")
	}
}

func TestMockDockerManager_CreateDirectory(t *testing.T) {
	mock := NewMockDockerManager()
	containerID := "container-123"
	dirPath := "/data/server/plugins"

	// Test successful directory creation
	err := mock.CreateDirectory(containerID, dirPath)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	// Test create failure
	mock.SetShouldFail("create_directory", true)
	err = mock.CreateDirectory(containerID, "/data/server/mods")
	if err == nil {
		t.Error("Expected error but got none")
	}
}

func TestMockDockerManager_DeletePath(t *testing.T) {
	mock := NewMockDockerManager()
	containerID := "container-123"
	filePath := "/data/server/test.txt"

	// Create a file first
	err := mock.WriteFile(containerID, filePath, []byte("test content"))
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// Test successful deletion
	err = mock.DeletePath(containerID, filePath)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	// Test delete failure
	mock.SetShouldFail("delete_path", true)
	err = mock.DeletePath(containerID, "/data/server/another.txt")
	if err == nil {
		t.Error("Expected error but got none")
	}
}

func TestMockDockerManager_RenameFile(t *testing.T) {
	mock := NewMockDockerManager()
	containerID := "container-123"
	oldPath := "/data/server/old.txt"
	newPath := "/data/server/new.txt"
	content := []byte("file content")

	// Create original file
	err := mock.WriteFile(containerID, oldPath, content)
	if err != nil {
		t.Fatalf("Failed to create original file: %v", err)
	}

	// Test successful rename
	err = mock.RenameFile(containerID, oldPath, newPath)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	// Verify file was moved
	newContent, err := mock.ReadFile(containerID, newPath)
	if err != nil {
		t.Errorf("Failed to read renamed file: %v", err)
	}
	if string(newContent) != string(content) {
		t.Errorf("Content mismatch after rename")
	}

	// Test rename failure
	mock.SetShouldFail("rename_file", true)
	err = mock.RenameFile(containerID, newPath, "/data/server/final.txt")
	if err == nil {
		t.Error("Expected error but got none")
	}
}

func TestMockDockerManager_DownloadFile(t *testing.T) {
	mock := NewMockDockerManager()
	containerID := "container-123"
	filePath := "/data/server/world/level.dat"
	content := []byte("world data")

	// Create file to download
	err := mock.WriteFile(containerID, filePath, content)
	if err != nil {
		t.Fatalf("Failed to create file for download: %v", err)
	}

	// Test successful download
	reader, err := mock.DownloadFile(containerID, filePath)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	defer reader.Close()

	downloadedContent, err := io.ReadAll(reader)
	if err != nil {
		t.Errorf("Failed to read downloaded content: %v", err)
	}
	if string(downloadedContent) != string(content) {
		t.Errorf("Downloaded content mismatch")
	}

	// Test download failure
	mock.SetShouldFail("download_file", true)
	_, err = mock.DownloadFile(containerID, filePath)
	if err == nil {
		t.Error("Expected error but got none")
	}
}

func TestMockDockerManager_UploadFile(t *testing.T) {
	mock := NewMockDockerManager()
	containerID := "container-123"
	destPath := "/data/server/uploaded.txt"
	content := "uploaded content"

	// Test successful upload
	reader := strings.NewReader(content)
	err := mock.UploadFile(containerID, destPath, reader)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	// Verify file was uploaded
	uploadedContent, err := mock.ReadFile(containerID, destPath)
	if err != nil {
		t.Errorf("Failed to read uploaded file: %v", err)
	}
	if string(uploadedContent) != content {
		t.Errorf("Uploaded content mismatch")
	}

	// Test upload failure
	mock.SetShouldFail("upload_file", true)
	reader = strings.NewReader("fail content")
	err = mock.UploadFile(containerID, "/data/server/fail.txt", reader)
	if err == nil {
		t.Error("Expected error but got none")
	}
}

func TestMockDockerManager_SendCommand(t *testing.T) {
	mock := NewMockDockerManager()
	containerID := "container-123"
	command := "say Hello World"

	// Test successful command
	err := mock.SendCommand(containerID, command)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	// Test command failure
	mock.SetShouldFail("send_command", true)
	err = mock.SendCommand(containerID, "say Goodbye")
	if err == nil {
		t.Error("Expected error but got none")
	}
}

func TestMockDockerManager_ExecCommand(t *testing.T) {
	mock := NewMockDockerManager()
	containerID := "container-123"
	cmd := []string{"ls", "-la", "/data/server"}

	// Test successful exec
	output, err := mock.ExecCommand(containerID, cmd)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	if output != "mock command output" {
		t.Errorf("Expected mock output, got: %s", output)
	}

	// Test exec failure
	mock.SetShouldFail("exec_command", true)
	_, err = mock.ExecCommand(containerID, []string{"fail", "command"})
	if err == nil {
		t.Error("Expected error but got none")
	}
}

func TestFileOperationsWorkflow(t *testing.T) {
	mock := NewMockDockerManager()
	containerID := "workflow-container"

	// 1. Create directory structure
	err := mock.CreateDirectory(containerID, "/data/server/plugins")
	if err != nil {
		t.Fatalf("Failed to create plugins directory: %v", err)
	}

	err = mock.CreateDirectory(containerID, "/data/server/world")
	if err != nil {
		t.Fatalf("Failed to create world directory: %v", err)
	}

	// 2. Write configuration files
	serverProps := []byte("server-name=Test Server\nmotd=Welcome!\ndifficulty=normal\n")
	err = mock.WriteFile(containerID, "/data/server/server.properties", serverProps)
	if err != nil {
		t.Fatalf("Failed to write server.properties: %v", err)
	}

	pluginConfig := []byte("plugin:\n  enabled: true\n  debug: false\n")
	err = mock.WriteFile(containerID, "/data/server/plugins/config.yml", pluginConfig)
	if err != nil {
		t.Fatalf("Failed to write plugin config: %v", err)
	}

	// 3. List files to verify structure
	files, err := mock.ListFiles(containerID, "/data/server")
	if err != nil {
		t.Fatalf("Failed to list files: %v", err)
	}
	if len(files) < 2 {
		t.Errorf("Expected at least 2 files, got %d", len(files))
	}

	// 4. Read back configuration
	readProps, err := mock.ReadFile(containerID, "/data/server/server.properties")
	if err != nil {
		t.Fatalf("Failed to read server.properties: %v", err)
	}
	if string(readProps) != string(serverProps) {
		t.Error("Server properties content mismatch")
	}

	// 5. Rename a file
	err = mock.RenameFile(containerID, "/data/server/server.properties", "/data/server/server.properties.backup")
	if err != nil {
		t.Fatalf("Failed to rename file: %v", err)
	}

	// 6. Verify rename worked
	_, err = mock.ReadFile(containerID, "/data/server/server.properties.backup")
	if err != nil {
		t.Error("Failed to read renamed file")
	}

	// 7. Upload a new file
	newContent := "updated-server-name=New Test Server\n"
	reader := strings.NewReader(newContent)
	err = mock.UploadFile(containerID, "/data/server/server.properties", reader)
	if err != nil {
		t.Fatalf("Failed to upload new file: %v", err)
	}

	// 8. Download the file to verify
	downloadReader, err := mock.DownloadFile(containerID, "/data/server/server.properties")
	if err != nil {
		t.Fatalf("Failed to download file: %v", err)
	}
	defer downloadReader.Close()

	downloadedContent, err := io.ReadAll(downloadReader)
	if err != nil {
		t.Fatalf("Failed to read downloaded content: %v", err)
	}
	if string(downloadedContent) != newContent {
		t.Error("Downloaded content doesn't match uploaded content")
	}

	// 9. Delete a file
	err = mock.DeletePath(containerID, "/data/server/server.properties.backup")
	if err != nil {
		t.Fatalf("Failed to delete backup file: %v", err)
	}

	// 10. Execute a command
	output, err := mock.ExecCommand(containerID, []string{"echo", "workflow complete"})
	if err != nil {
		t.Fatalf("Failed to execute command: %v", err)
	}
	if output != "mock command output" {
		t.Errorf("Unexpected command output: %s", output)
	}
}

func TestPathValidation(t *testing.T) {
	// Note: In the real implementation, path validation would be tested
	// This is a placeholder for testing the path validation logic
	tests := []struct {
		name      string
		path      string
		valid     bool
		operation string
	}{
		{
			name:      "valid server path",
			path:      "/data/server/config.yml",
			valid:     true,
			operation: "write",
		},
		{
			name:      "valid backup path",
			path:      "/data/backups/backup.tar.gz",
			valid:     true,
			operation: "read",
		},
		{
			name:      "invalid path - outside data",
			path:      "/etc/passwd",
			valid:     false,
			operation: "read",
		},
		{
			name:      "invalid path - parent traversal",
			path:      "/data/server/../../../etc/passwd",
			valid:     false,
			operation: "read",
		},
	}

	// This would test the actual path validation in the real implementation
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// In real implementation, test validatePath function
			// For now, just document the expected behavior
			t.Logf("Path: %s, Expected valid: %v, Operation: %s", tt.path, tt.valid, tt.operation)
		})
	}
}

func TestLargeFileHandling(t *testing.T) {
	mock := NewMockDockerManager()
	containerID := "large-file-container"

	// Test with different file sizes
	sizes := []int{1024, 10240, 102400} // 1KB, 10KB, 100KB

	for _, size := range sizes {
		t.Run(fmt.Sprintf("file_size_%d", size), func(t *testing.T) {
			// Create content of specified size
			content := make([]byte, size)
			for i := range content {
				content[i] = byte(i % 256)
			}

			filePath := fmt.Sprintf("/data/server/large_%d.bin", size)

			// Write large file
			err := mock.WriteFile(containerID, filePath, content)
			if err != nil {
				t.Errorf("Failed to write large file (%d bytes): %v", size, err)
			}

			// Read it back
			readContent, err := mock.ReadFile(containerID, filePath)
			if err != nil {
				t.Errorf("Failed to read large file (%d bytes): %v", size, err)
			}

			if len(readContent) != len(content) {
				t.Errorf("Size mismatch: expected %d bytes, got %d bytes", len(content), len(readContent))
			}

			// Download it
			reader, err := mock.DownloadFile(containerID, filePath)
			if err != nil {
				t.Errorf("Failed to download large file (%d bytes): %v", size, err)
			}
			defer reader.Close()

			downloadedContent, err := io.ReadAll(reader)
			if err != nil {
				t.Errorf("Failed to read downloaded large file (%d bytes): %v", size, err)
			}

			if len(downloadedContent) != len(content) {
				t.Errorf("Download size mismatch: expected %d bytes, got %d bytes", len(content), len(downloadedContent))
			}
		})
	}
}

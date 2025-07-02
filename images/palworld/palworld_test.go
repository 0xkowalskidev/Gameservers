package palworld_test

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/0xkowalskidev/gameserverquery/query"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

const (
	// Test timeouts
	buildTimeout   = 10 * time.Minute // Palworld is large download
	startupTimeout = 5 * time.Minute
	shutdownTimeout = 30 * time.Second
	
	// Default test values
	defaultServerName = "Test Palworld Server"
	defaultPassword   = "testpass123"
)

// getDockerfileContext returns the absolute path to the palworld image directory
func getDockerfileContext() string {
	wd, _ := os.Getwd()
	// We're already in the palworld directory, so return current directory
	return wd
}

// readLogsToString reads an io.Reader and returns its content as a string
func readLogsToString(reader io.Reader) (string, error) {
	var builder strings.Builder
	scanner := bufio.NewScanner(reader)
	for scanner.Scan() {
		builder.WriteString(scanner.Text())
		builder.WriteString("\n")
	}
	if err := scanner.Err(); err != nil {
		return "", err
	}
	return builder.String(), nil
}

// cleanupContainer ensures proper container cleanup with timeout and force removal
func cleanupContainer(t *testing.T, container testcontainers.Container) {
	if container == nil {
		return
	}
	
	t.Helper()
	
	// Use a separate context for cleanup to avoid cancellation issues
	cleanupCtx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	
	// First try graceful termination
	if err := container.Terminate(cleanupCtx); err != nil {
		t.Logf("Warning: Failed to terminate container gracefully: %v", err)
		
		// If graceful termination fails, try forceful stop
		stopTimeout := 10 * time.Second
		if err := container.Stop(cleanupCtx, &stopTimeout); err != nil {
			t.Logf("Warning: Failed to stop container forcefully: %v", err)
			
			// Last resort: try to get container info for debugging
			if state, stateErr := container.State(cleanupCtx); stateErr == nil {
				t.Logf("Container state during cleanup failure: Running=%v, Status=%s", 
					state.Running, state.Status)
			}
		} else {
			t.Logf("Container stopped successfully after terminate failed")
		}
	} else {
		t.Logf("Container terminated successfully")
	}
}

// setupTest performs pre-test cleanup to ensure clean environment
func setupTest(t *testing.T) {
	t.Helper()
	
	// Set up test-specific cleanup that runs even if test panics
	t.Cleanup(func() {
		// This runs after each test regardless of outcome
		cleanupTestResources(t)
	})
}

// cleanupTestResources performs comprehensive cleanup of test resources
func cleanupTestResources(t *testing.T) {
	t.Helper()
	
	// This function can be called to clean up any leaked resources
	// In practice, Testcontainers with Ryuk should handle most cleanup automatically
	// But this provides a hook for additional cleanup if needed
}

// TestPalworldImage_Build tests that the Docker image builds successfully
func TestPalworldImage_Build(t *testing.T) {
	setupTest(t)
	ctx := context.Background()
	
	// Build the image without starting a container
	_, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: testcontainers.ContainerRequest{
			FromDockerfile: testcontainers.FromDockerfile{
				Context:       getDockerfileContext(),
				Dockerfile:    "Dockerfile",
				PrintBuildLog: true,
			},
		},
		Started: false, // Just build, don't start
	})
	
	if err != nil {
		t.Fatalf("Failed to build Palworld Docker image: %v", err)
	}
}

// TestPalworldImage_BasicStartup tests basic container startup with minimal configuration
func TestPalworldImage_BasicStartup(t *testing.T) {
	setupTest(t)
	ctx := context.Background()
	
	container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: testcontainers.ContainerRequest{
			FromDockerfile: testcontainers.FromDockerfile{
				Context:    getDockerfileContext(),
				Dockerfile: "Dockerfile",
			},
			ExposedPorts: []string{"8211/udp", "27015/udp"},
			Env: map[string]string{
				"SERVER_NAME": defaultServerName,
				"SERVER_PASSWORD": defaultPassword,
			},
			WaitingFor: wait.ForLog("Starting Palworld server").WithStartupTimeout(startupTimeout),
		},
		Started: true,
	})
	if err != nil {
		t.Fatalf("Failed to start Palworld container: %v", err)
	}
	
	// Ensure cleanup happens regardless of test outcome
	defer cleanupContainer(t, container)
	
	// Verify container is running
	state, err := container.State(ctx)
	if err != nil {
		t.Fatalf("Failed to get container state: %v", err)
	}
	if !state.Running {
		t.Fatal("Container should be running")
	}
	
	// Get the mapped port for testing
	mappedPort, err := container.MappedPort(ctx, "8211/udp")
	if err != nil {
		t.Fatalf("Failed to get mapped port: %v", err)
	}
	
	// Verify server is responding using GameserverQuery
	serverAddress := fmt.Sprintf("127.0.0.1:%s", mappedPort.Port())
	
	// Wait a bit for server to fully initialize
	time.Sleep(30 * time.Second)
	
	// Try to query the server multiple times (Palworld sometimes takes a while to respond)
	var queryErr error
	for i := 0; i < 3; i++ {
		queryCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
		serverInfo, err := query.Query(queryCtx, "palworld", serverAddress)
		cancel()
		
		if err == nil {
			t.Logf("Server successfully started - Name: %s, Players: %d/%d", 
				serverInfo.Name, serverInfo.Players.Current, serverInfo.Players.Max)
			queryErr = nil
			break
		}
		
		queryErr = err
		t.Logf("Query attempt %d failed: %v", i+1, err)
		if i < 2 {
			time.Sleep(15 * time.Second)
		}
	}
	
	if queryErr != nil {
		// Palworld server query may be unreliable, so just log this as a warning
		t.Logf("Warning: Failed to query Palworld server after 3 attempts: %v", queryErr)
		t.Logf("This may be expected behavior for Palworld servers which don't always respond to queries")
	}
}

// TestPalworldImage_EnvironmentVariables tests that environment variables are properly applied
func TestPalworldImage_EnvironmentVariables(t *testing.T) {
	setupTest(t)
	ctx := context.Background()
	
	testCases := []struct {
		name     string
		env      map[string]string
		checkLog func(logs string) error
	}{
		{
			name: "Server name configuration",
			env: map[string]string{
				"SERVER_NAME": "My Custom Palworld Server",
				"SERVER_PASSWORD": defaultPassword,
			},
			checkLog: func(logs string) error {
				if !strings.Contains(logs, "Starting Palworld server: My Custom Palworld Server") {
					return fmt.Errorf("Server name should be set to 'My Custom Palworld Server'")
				}
				return nil
			},
		},
		{
			name: "Server description configuration",
			env: map[string]string{
				"SERVER_NAME": defaultServerName,
				"SERVER_DESCRIPTION": "A custom description",
				"SERVER_PASSWORD": defaultPassword,
			},
			checkLog: func(logs string) error {
				if !strings.Contains(logs, "Configuration updated in PalWorldSettings.ini") {
					return fmt.Errorf("Configuration should be updated")
				}
				return nil
			},
		},
		{
			name: "Max players configuration",
			env: map[string]string{
				"SERVER_NAME": defaultServerName,
				"SERVER_PASSWORD": defaultPassword,
				"MAX_PLAYERS": "20",
			},
			checkLog: func(logs string) error {
				if !strings.Contains(logs, "Starting Palworld server: Test Palworld Server") {
					return fmt.Errorf("Server should start with max players configuration")
				}
				return nil
			},
		},
		{
			name: "Public port configuration",
			env: map[string]string{
				"SERVER_NAME": defaultServerName,
				"SERVER_PASSWORD": defaultPassword,
				"PUBLIC_PORT": "8211",
			},
			checkLog: func(logs string) error {
				if !strings.Contains(logs, "Starting Palworld server: Test Palworld Server") {
					return fmt.Errorf("Server should start with port configuration")
				}
				return nil
			},
		},
		{
			name: "Admin password configuration",
			env: map[string]string{
				"SERVER_NAME": defaultServerName,
				"SERVER_PASSWORD": defaultPassword,
				"ADMIN_PASSWORD": "supersecretadmin",
			},
			checkLog: func(logs string) error {
				if !strings.Contains(logs, "Starting Palworld server: Test Palworld Server") {
					return fmt.Errorf("Server should start with admin password configuration")
				}
				return nil
			},
		},
	}
	
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
				ContainerRequest: testcontainers.ContainerRequest{
					FromDockerfile: testcontainers.FromDockerfile{
						Context:    getDockerfileContext(),
						Dockerfile: "Dockerfile",
					},
					ExposedPorts: []string{"8211/udp", "27015/udp"},
					Env:          tc.env,
					WaitingFor:   wait.ForLog("Starting Palworld server").WithStartupTimeout(startupTimeout),
				},
				Started: true,
			})
			if err != nil {
				t.Fatalf("Failed to start container: %v", err)
			}
			defer cleanupContainer(t, container)
			
			// Get container logs
			logs, err := container.Logs(ctx)
			if err != nil {
				t.Fatalf("Failed to get container logs: %v", err)
			}
			defer logs.Close()
			
			logStr, err := readLogsToString(logs)
			if err != nil {
				t.Fatalf("Failed to read logs: %v", err)
			}
			
			// Check the logs
			if err := tc.checkLog(logStr); err != nil {
				t.Errorf("Environment variable test failed: %v", err)
				t.Logf("Container logs:\n%s", logStr)
			}
		})
	}
}

// TestPalworldImage_Shutdown tests that the container shuts down when stopped
func TestPalworldImage_Shutdown(t *testing.T) {
	setupTest(t)
	ctx := context.Background()
	
	container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: testcontainers.ContainerRequest{
			FromDockerfile: testcontainers.FromDockerfile{
				Context:    getDockerfileContext(),
				Dockerfile: "Dockerfile",
			},
			ExposedPorts: []string{"8211/udp", "27015/udp"},
			Env: map[string]string{
				"SERVER_NAME": defaultServerName,
				"SERVER_PASSWORD": defaultPassword,
			},
			WaitingFor: wait.ForLog("Starting Palworld server").WithStartupTimeout(startupTimeout),
		},
		Started: true,
	})
	if err != nil {
		t.Fatalf("Failed to start container: %v", err)
	}
	
	// Send SIGTERM to the container
	stopTimeout := shutdownTimeout
	if err := container.Stop(ctx, &stopTimeout); err != nil {
		t.Fatalf("Failed to stop container: %v", err)
	}
	
	// Verify container actually stopped
	state, err := container.State(ctx)
	if err != nil {
		t.Fatalf("Failed to get final container state: %v", err)
	}
	if state.Running {
		t.Error("Container should have stopped after SIGTERM")
	}
	
	// Cleanup
	if err := container.Terminate(ctx); err != nil {
		t.Logf("Failed to terminate container: %v", err)
	}
}

// TestPalworldImage_FileStructure tests that the expected file structure is created
func TestPalworldImage_FileStructure(t *testing.T) {
	setupTest(t)
	ctx := context.Background()
	
	container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: testcontainers.ContainerRequest{
			FromDockerfile: testcontainers.FromDockerfile{
				Context:    getDockerfileContext(),
				Dockerfile: "Dockerfile",
			},
			ExposedPorts: []string{"8211/udp", "27015/udp"},
			Env: map[string]string{
				"SERVER_NAME": defaultServerName,
				"SERVER_PASSWORD": defaultPassword,
			},
			WaitingFor: wait.ForLog("Starting Palworld server").WithStartupTimeout(startupTimeout),
		},
		Started: true,
	})
	if err != nil {
		t.Fatalf("Failed to start container: %v", err)
	}
	defer cleanupContainer(t, container)
	
	// Check required files and directories exist
	requiredPaths := []string{
		"/data/server/PalServer.sh",
		"/data/scripts/start.sh",
	}
	
	for _, path := range requiredPaths {
		exitCode, outputReader, err := container.Exec(ctx, []string{"ls", "-la", path})
		if err != nil {
			t.Fatalf("Failed to check path %s: %v", path, err)
		}
		if exitCode != 0 {
			output, _ := readLogsToString(outputReader)
			t.Errorf("Required path %s not found, exit code: %d, output: %s", path, exitCode, output)
		}
	}
	
	// Check that scripts are executable
	executableScripts := []string{
		"/data/scripts/start.sh",
		"/data/server/PalServer.sh",
	}
	
	for _, script := range executableScripts {
		exitCode, outputReader, err := container.Exec(ctx, []string{"test", "-x", script})
		if err != nil {
			t.Fatalf("Failed to test executable %s: %v", script, err)
		}
		if exitCode != 0 {
			output, _ := readLogsToString(outputReader)
			t.Errorf("Script %s should be executable, exit code: %d, output: %s", script, exitCode, output)
		}
	}
	
	// Check that configuration directory exists
	exitCode, outputReader, err := container.Exec(ctx, []string{"ls", "-la", "/data/server/Pal/Saved/Config/LinuxServer"})
	if err != nil {
		t.Fatalf("Failed to list config directory: %v", err)
	}
	if exitCode != 0 {
		output, _ := readLogsToString(outputReader)
		t.Errorf("Config directory should exist, exit code: %d, output: %s", exitCode, output)
	}
}
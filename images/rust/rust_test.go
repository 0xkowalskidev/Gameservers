package rust_test

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

const (
	// Test timeouts - Rust takes much longer to start than other games
	buildTimeout     = 15 * time.Minute // Rust server download is large
	startupTimeout   = 10 * time.Minute // World generation can take a very long time
	shutdownTimeout  = 2 * time.Minute  // Rust servers take time to save and shutdown
)

// getDockerfileContext returns the absolute path to the rust image directory
func getDockerfileContext() string {
	wd, _ := os.Getwd()
	// We're already in the rust directory, so return current directory
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
	cleanupCtx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	
	// First try graceful termination
	if err := container.Terminate(cleanupCtx); err != nil {
		t.Logf("Warning: Failed to terminate container gracefully: %v", err)
		
		// If graceful termination fails, try forceful stop
		stopTimeout := 60 * time.Second
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

// TestRustImage_Build tests that the Docker image builds successfully
func TestRustImage_Build(t *testing.T) {
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
		t.Fatalf("Failed to build Rust Docker image: %v", err)
	}
}

// TestRustImage_BasicStartup tests basic container startup with minimal configuration
// Note: This test may take 5-10 minutes due to Rust's world generation
func TestRustImage_BasicStartup(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping long-running Rust startup test in short mode")
	}
	
	setupTest(t)
	ctx := context.Background()
	
	container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: testcontainers.ContainerRequest{
			FromDockerfile: testcontainers.FromDockerfile{
				Context:    getDockerfileContext(),
				Dockerfile: "Dockerfile",
			},
			ExposedPorts: []string{"28015/tcp"},
			Env: map[string]string{
				"WORLDSIZE": "1000", // Smaller world for faster testing
				"SEED":      "12345",
			},
			// Wait for server to be ready for players (this indicates full startup)
			WaitingFor: wait.ForLog("Server startup complete").WithStartupTimeout(startupTimeout),
		},
		Started: true,
	})
	if err != nil {
		t.Fatalf("Failed to start Rust container: %v", err)
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
	
	// Get the mapped port for verification
	mappedPort, err := container.MappedPort(ctx, "28015")
	if err != nil {
		t.Fatalf("Failed to get mapped port: %v", err)
	}
	
	t.Logf("Rust server started successfully on port %s", mappedPort.Port())
}

// TestRustImage_EnvironmentVariables tests that environment variables are properly applied
func TestRustImage_EnvironmentVariables(t *testing.T) {
	setupTest(t)
	ctx := context.Background()
	
	testCases := []struct {
		name     string
		env      map[string]string
		checkLog func(logs string) error
	}{
		{
			name: "Server name",
			env: map[string]string{
				"NAME":      "Test Rust Server",
				"WORLDSIZE": "1000", // Small world for faster testing
			},
			checkLog: func(logs string) error {
				if !strings.Contains(logs, "+server.hostname \"Test Rust Server\"") {
					return fmt.Errorf("server name should be set in launch command")
				}
				return nil
			},
		},
		{
			name: "World size",
			env: map[string]string{
				"WORLDSIZE": "2000",
			},
			checkLog: func(logs string) error {
				if !strings.Contains(logs, "+server.worldsize 2000") {
					return fmt.Errorf("world size should be set to 2000")
				}
				return nil
			},
		},
		{
			name: "Max players",
			env: map[string]string{
				"MAXPLAYERS": "100",
				"WORLDSIZE":  "1000",
			},
			checkLog: func(logs string) error {
				if !strings.Contains(logs, "+server.maxplayers 100") {
					return fmt.Errorf("max players should be set to 100")
				}
				return nil
			},
		},
		{
			name: "Server seed",
			env: map[string]string{
				"SEED":      "99999",
				"WORLDSIZE": "1000",
			},
			checkLog: func(logs string) error {
				if !strings.Contains(logs, "+server.seed 99999") {
					return fmt.Errorf("server seed should be set to 99999")
				}
				return nil
			},
		},
		{
			name: "RCON configuration",
			env: map[string]string{
				"RCON_PASSWORD": "testpassword123",
				"RCON_PORT":     "28017",
				"WORLDSIZE":     "1000",
			},
			checkLog: func(logs string) error {
				if !strings.Contains(logs, "+rcon.password \"testpassword123\"") {
					return fmt.Errorf("RCON password should be set")
				}
				if !strings.Contains(logs, "+rcon.port 28017") {
					return fmt.Errorf("RCON port should be set to 28017")
				}
				if !strings.Contains(logs, "+rcon.web 1") {
					return fmt.Errorf("RCON web should be enabled")
				}
				return nil
			},
		},
	}
	
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Start container but don't wait for full startup - just check launch command
			container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
				ContainerRequest: testcontainers.ContainerRequest{
					FromDockerfile: testcontainers.FromDockerfile{
						Context:    getDockerfileContext(),
						Dockerfile: "Dockerfile",
					},
					ExposedPorts: []string{"28015/tcp"},
					Env:          tc.env,
					// Only wait for launch command to appear, not full startup
					WaitingFor: wait.ForLog("Launching Rust server with the following command:").WithStartupTimeout(2*time.Minute),
				},
				Started: true,
			})
			if err != nil {
				t.Fatalf("Failed to start container: %v", err)
			}
			defer cleanupContainer(t, container)
			
			// Get logs to check launch command
			logs, err := container.Logs(ctx)
			if err != nil {
				t.Fatalf("Failed to get container logs: %v", err)
			}
			defer logs.Close()
			
			logStr, err := readLogsToString(logs)
			if err != nil {
				t.Fatalf("Failed to read logs: %v", err)
			}
			
			if err := tc.checkLog(logStr); err != nil {
				t.Errorf("Environment variable test failed: %v", err)
				t.Logf("Container logs:\n%s", logStr)
			}
		})
	}
}

// TestRustImage_CommandInterface tests the RCON command system
func TestRustImage_CommandInterface(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping long-running Rust command interface test in short mode")
	}
	
	setupTest(t)
	ctx := context.Background()
	
	container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: testcontainers.ContainerRequest{
			FromDockerfile: testcontainers.FromDockerfile{
				Context:    getDockerfileContext(),
				Dockerfile: "Dockerfile",
			},
			ExposedPorts: []string{"28015/tcp", "28016/tcp"},
			Env: map[string]string{
				"RCON_PASSWORD": "testpass123",
				"WORLDSIZE":     "1000", // Smaller world for faster testing
			},
			WaitingFor: wait.ForLog("Server startup complete").WithStartupTimeout(startupTimeout),
		},
		Started: true,
	})
	if err != nil {
		t.Fatalf("Failed to start container: %v", err)
	}
	defer cleanupContainer(t, container)
	
	// Test sending a command through RCON
	exitCode, outputReader, err := container.Exec(ctx, []string{
		"bash", "-c", "RCON_PASSWORD=testpass123 /data/scripts/send-command.sh 'status'",
	})
	if err != nil {
		t.Fatalf("Failed to send RCON command: %v", err)
	}
	if exitCode != 0 {
		output, _ := readLogsToString(outputReader)
		t.Fatalf("Failed to send RCON command, exit code: %d, output: %s", exitCode, output)
	}
	
	// Read command output
	output, err := readLogsToString(outputReader)
	if err != nil {
		t.Fatalf("Failed to read command output: %v", err)
	}
	
	// RCON status command should return server information
	if !strings.Contains(output, "hostname") || !strings.Contains(output, "players") {
		t.Errorf("RCON status command should return server information, got: %s", output)
	}
}

// TestRustImage_PortConfiguration tests that ports are configured correctly
func TestRustImage_PortConfiguration(t *testing.T) {
	setupTest(t)
	ctx := context.Background()
	
	container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: testcontainers.ContainerRequest{
			FromDockerfile: testcontainers.FromDockerfile{
				Context:    getDockerfileContext(),
				Dockerfile: "Dockerfile",
			},
			ExposedPorts: []string{"28015/tcp", "28016/tcp"},
			Env: map[string]string{
				"RCON_PASSWORD": "testpass123",
				"WORLDSIZE":     "1000",
			},
			WaitingFor: wait.ForLog("Launching Rust server with the following command:").WithStartupTimeout(2*time.Minute),
		},
		Started: true,
	})
	if err != nil {
		t.Fatalf("Failed to start container: %v", err)
	}
	defer cleanupContainer(t, container)
	
	// Get logs to verify port configuration
	logs, err := container.Logs(ctx)
	if err != nil {
		t.Fatalf("Failed to get container logs: %v", err)
	}
	defer logs.Close()
	
	logStr, err := readLogsToString(logs)
	if err != nil {
		t.Fatalf("Failed to read logs: %v", err)
	}
	
	// Check that server is configured with correct ports
	if !strings.Contains(logStr, "+server.port 28015") {
		t.Error("Server should be configured with port 28015")
	}
	
	if !strings.Contains(logStr, "+rcon.port 28016") {
		t.Error("RCON should be configured with port 28016")
	}
}

// TestRustImage_GracefulShutdown tests that the container shuts down gracefully
func TestRustImage_GracefulShutdown(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping long-running Rust shutdown test in short mode")
	}
	
	setupTest(t)
	ctx := context.Background()
	
	container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: testcontainers.ContainerRequest{
			FromDockerfile: testcontainers.FromDockerfile{
				Context:    getDockerfileContext(),
				Dockerfile: "Dockerfile",
			},
			ExposedPorts: []string{"28015/tcp"},
			Env: map[string]string{
				"WORLDSIZE": "1000", // Small world for faster testing
			},
			WaitingFor: wait.ForLog("Server startup complete").WithStartupTimeout(startupTimeout),
		},
		Started: true,
	})
	if err != nil {
		t.Fatalf("Failed to start container: %v", err)
	}
	
	// Send SIGTERM to the container (graceful shutdown)
	stopTimeout := shutdownTimeout
	if err := container.Stop(ctx, &stopTimeout); err != nil {
		t.Fatalf("Failed to stop container gracefully: %v", err)
	}
	
	// Get final logs to check for graceful shutdown messages
	logs, err := container.Logs(ctx)
	if err != nil {
		t.Fatalf("Failed to get logs: %v", err)
	}
	defer logs.Close()
	
	// Read logs into string
	logStr, err := readLogsToString(logs)
	if err != nil {
		t.Fatalf("Failed to read logs: %v", err)
	}
	
	// Check for graceful shutdown indicators
	gracefulShutdownPatterns := []string{
		"Received SIGTERM",
		"stopping Rust server gracefully",
		"Rust server stopped",
	}
	
	found := false
	for _, pattern := range gracefulShutdownPatterns {
		if strings.Contains(logStr, pattern) {
			found = true
			break
		}
	}
	
	if !found {
		t.Error("Expected at least one graceful shutdown message in logs")
		t.Logf("Container logs:\n%s", logStr)
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

// TestRustImage_FileStructure tests that the expected file structure is created
func TestRustImage_FileStructure(t *testing.T) {
	setupTest(t)
	ctx := context.Background()
	
	container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: testcontainers.ContainerRequest{
			FromDockerfile: testcontainers.FromDockerfile{
				Context:    getDockerfileContext(),
				Dockerfile: "Dockerfile",
			},
			ExposedPorts: []string{"28015/tcp"},
			Env: map[string]string{
				"WORLDSIZE": "1000",
			},
			WaitingFor: wait.ForLog("Launching Rust server with the following command:").WithStartupTimeout(2*time.Minute),
		},
		Started: true,
	})
	if err != nil {
		t.Fatalf("Failed to start container: %v", err)
	}
	defer cleanupContainer(t, container)
	
	// Check required files and directories exist
	requiredPaths := []string{
		"/data/server/RustDedicated",
		"/data/scripts/start.sh",
		"/data/scripts/send-command.sh",
		"/data/steamcmd/steamcmd.sh",
		"/data/backups",
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
		"/data/scripts/send-command.sh",
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
	
	// Verify rcon-cli is available
	exitCode, outputReader, err := container.Exec(ctx, []string{"which", "rcon-cli"})
	if err != nil {
		t.Fatalf("Failed to check for rcon-cli: %v", err)
	}
	if exitCode != 0 {
		output, _ := readLogsToString(outputReader)
		t.Errorf("rcon-cli should be available, exit code: %d, output: %s", exitCode, output)
	}
}
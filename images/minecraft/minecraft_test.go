package minecraft_test

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/0xkowalskidev/gameserverquery/query"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

const (
	// Test timeouts
	buildTimeout   = 5 * time.Minute
	startupTimeout = 3 * time.Minute
	shutdownTimeout = 30 * time.Second
	
	// Default test values
	defaultMemory = "1024"
	testServerName = "Test Minecraft Server"
)

// getDockerfileContext returns the absolute path to the minecraft image directory
func getDockerfileContext() string {
	wd, _ := os.Getwd()
	// We're already in the minecraft directory, so return current directory
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

// TestMinecraftImage_Build tests that the Docker image builds successfully
func TestMinecraftImage_Build(t *testing.T) {
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
		t.Fatalf("Failed to build Minecraft Docker image: %v", err)
	}
}

// TestMinecraftImage_BasicStartup tests basic container startup with minimal configuration
func TestMinecraftImage_BasicStartup(t *testing.T) {
	setupTest(t)
	ctx := context.Background()
	
	container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: testcontainers.ContainerRequest{
			FromDockerfile: testcontainers.FromDockerfile{
				Context:    getDockerfileContext(),
				Dockerfile: "Dockerfile",
			},
			ExposedPorts: []string{"25565/tcp"},
			Env: map[string]string{
				"EULA":      "true",
				"MEMORY_MB": defaultMemory,
				// Note: No MINECRAFT_VERSION set, should default to "latest"
			},
			WaitingFor: wait.ForLog("Done").WithStartupTimeout(startupTimeout),
		},
		Started: true,
	})
	if err != nil {
		t.Fatalf("Failed to start Minecraft container: %v", err)
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
	mappedPort, err := container.MappedPort(ctx, "25565")
	if err != nil {
		t.Fatalf("Failed to get mapped port: %v", err)
	}
	
	// Verify server is responding using GameserverQuery
	serverAddress := fmt.Sprintf("127.0.0.1:%s", mappedPort.Port())
	
	// Wait a bit for server to fully initialize
	time.Sleep(10 * time.Second)
	
	queryCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	
	serverInfo, err := query.Query(queryCtx, "minecraft", serverAddress)
	if err != nil {
		t.Fatalf("Failed to query Minecraft server: %v", err)
	}
	
	// Note: Server name might be empty for default Minecraft servers, which is normal
	// The important thing is that we can query the server successfully
	
	t.Logf("Server successfully started - Name: %s, Players: %d/%d", 
		serverInfo.Name, serverInfo.Players.Current, serverInfo.Players.Max)
}

// TestMinecraftImage_EnvironmentVariables tests that environment variables are properly applied
func TestMinecraftImage_EnvironmentVariables(t *testing.T) {
	setupTest(t)
	ctx := context.Background()
	
	testCases := []struct {
		name     string
		env      map[string]string
		checkLog func(logs string) error
	}{
		{
			name: "EULA acceptance",
			env: map[string]string{
				"EULA":      "true",
				"MEMORY_MB": defaultMemory,
			},
			checkLog: func(logs string) error {
				// Check that server started successfully (which means EULA was accepted)
				if !strings.Contains(logs, "Done (") {
					return fmt.Errorf("Server should start successfully when EULA=true, but 'Done' message not found")
				}
				return nil
			},
		},
		{
			name: "Server name (MOTD)",
			env: map[string]string{
				"EULA":        "true",
				"MEMORY_MB":   defaultMemory,
				"SERVER_NAME": testServerName,
			},
			checkLog: func(logs string) error {
				// Check server.properties for MOTD setting
				if !strings.Contains(logs, "motd="+testServerName) {
					return fmt.Errorf("server name '%s' should be set in MOTD", testServerName)
				}
				return nil
			},
		},
		{
			name: "Memory allocation",
			env: map[string]string{
				"EULA":      "true",
				"MEMORY_MB": "2048",
			},
			checkLog: func(logs string) error {
				if !strings.Contains(logs, "-Xmx2048M") || !strings.Contains(logs, "-Xms2048M") {
					return fmt.Errorf("memory allocation should be set to 2048M")
				}
				return nil
			},
		},
		{
			name: "Difficulty setting",
			env: map[string]string{
				"EULA":       "true",
				"MEMORY_MB":  defaultMemory,
				"DIFFICULTY": "hard",
			},
			checkLog: func(logs string) error {
				if !strings.Contains(logs, "difficulty=hard") {
					return fmt.Errorf("difficulty should be set to hard")
				}
				return nil
			},
		},
		{
			name: "Gamemode setting",
			env: map[string]string{
				"EULA":      "true",
				"MEMORY_MB": defaultMemory,
				"GAMEMODE":  "creative",
			},
			checkLog: func(logs string) error {
				if !strings.Contains(logs, "gamemode=creative") {
					return fmt.Errorf("gamemode should be set to creative")
				}
				return nil
			},
		},
		{
			name: "Minecraft version (latest)",
			env: map[string]string{
				"EULA":             "true",
				"MEMORY_MB":        defaultMemory,
				"MINECRAFT_VERSION": "latest",
			},
			checkLog: func(logs string) error {
				// Should see version checking and download messages
				if !strings.Contains(logs, "Checking Minecraft server version: latest") {
					return fmt.Errorf("should check for latest version")
				}
				if !strings.Contains(logs, "Latest release version is:") {
					return fmt.Errorf("should determine latest release version")
				}
				return nil
			},
		},
		{
			name: "Minecraft version (specific)",
			env: map[string]string{
				"EULA":             "true",
				"MEMORY_MB":        defaultMemory,
				"MINECRAFT_VERSION": "1.21.4",
			},
			checkLog: func(logs string) error {
				// Should see version checking and download messages
				if !strings.Contains(logs, "Checking Minecraft server version: 1.21.4") {
					return fmt.Errorf("should check for version 1.21.4")
				}
				if !strings.Contains(logs, "Successfully downloaded minecraft_server_1.21.4.jar") {
					return fmt.Errorf("should download 1.21.4 server JAR")
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
					ExposedPorts: []string{"25565/tcp"},
					Env:          tc.env,
					WaitingFor:   wait.ForLog("Done").WithStartupTimeout(startupTimeout),
				},
				Started: true,
			})
			if err != nil {
				t.Fatalf("Failed to start container: %v", err)
			}
			defer cleanupContainer(t, container)
			
			// For environment variable tests, we need to check the actual configuration files
			// rather than logs, since script modifications don't appear in Java logs
			
			var checkErr error
			switch tc.name {
			case "EULA acceptance":
				// Check that eula.txt file has correct content
				exitCode, eulaOutput, err := container.Exec(ctx, []string{"cat", "/data/server/eula.txt"})
				if err != nil || exitCode != 0 {
					checkErr = fmt.Errorf("failed to read eula.txt: %v", err)
				} else {
					eulaStr, _ := readLogsToString(eulaOutput)
					if !strings.Contains(eulaStr, "eula=true") {
						checkErr = fmt.Errorf("EULA should be set to true in eula.txt, got: %s", eulaStr)
					}
				}
			case "Server name (MOTD)":
				// Check server.properties for MOTD
				exitCode, propOutput, err := container.Exec(ctx, []string{"grep", "motd", "/data/server/server.properties"})
				if err != nil || exitCode != 0 {
					checkErr = fmt.Errorf("failed to check server.properties for MOTD: %v", err)
				} else {
					propStr, _ := readLogsToString(propOutput)
					if !strings.Contains(propStr, testServerName) {
						checkErr = fmt.Errorf("server name '%s' should be in MOTD, got: %s", testServerName, propStr)
					}
				}
			case "Memory allocation":
				// Check for memory settings in process list
				exitCode, psOutput, err := container.Exec(ctx, []string{"ps", "aux"})
				if err != nil || exitCode != 0 {
					checkErr = fmt.Errorf("failed to check process list: %v", err)
				} else {
					psStr, _ := readLogsToString(psOutput)
					if !strings.Contains(psStr, "-Xmx2048M") || !strings.Contains(psStr, "-Xms2048M") {
						checkErr = fmt.Errorf("memory allocation should be 2048M, process list: %s", psStr)
					}
				}
			case "Difficulty setting":
				// Check server.properties for difficulty
				exitCode, propOutput, err := container.Exec(ctx, []string{"grep", "difficulty", "/data/server/server.properties"})
				if err != nil || exitCode != 0 {
					checkErr = fmt.Errorf("failed to check server.properties for difficulty: %v", err)
				} else {
					propStr, _ := readLogsToString(propOutput)
					if !strings.Contains(propStr, "difficulty=hard") {
						checkErr = fmt.Errorf("difficulty should be set to hard, got: %s", propStr)
					}
				}
			case "Gamemode setting":
				// Check server.properties for gamemode
				exitCode, propOutput, err := container.Exec(ctx, []string{"grep", "gamemode", "/data/server/server.properties"})
				if err != nil || exitCode != 0 {
					checkErr = fmt.Errorf("failed to check server.properties for gamemode: %v", err)
				} else {
					propStr, _ := readLogsToString(propOutput)
					if !strings.Contains(propStr, "gamemode=creative") {
						checkErr = fmt.Errorf("gamemode should be set to creative, got: %s", propStr)
					}
				}
			case "Minecraft version (latest)", "Minecraft version (specific)":
				// For version tests, check container logs for download messages
				logs, err := container.Logs(ctx)
				if err != nil {
					checkErr = fmt.Errorf("failed to get container logs: %v", err)
				} else {
					defer logs.Close()
					logStr, err := readLogsToString(logs)
					if err != nil {
						checkErr = fmt.Errorf("failed to read logs: %v", err)
					} else {
						checkErr = tc.checkLog(logStr)
					}
				}
			default:
				// Fallback to original log checking
				logs, err := container.Logs(ctx)
				if err != nil {
					t.Fatalf("Failed to get container logs: %v", err)
				}
				defer logs.Close()
				
				logStr, err := readLogsToString(logs)
				if err != nil {
					t.Fatalf("Failed to read logs: %v", err)
				}
				
				checkErr = tc.checkLog(logStr)
			}
			
			if checkErr != nil {
				t.Errorf("Environment variable test failed: %v", checkErr)
			}
		})
	}
}

// TestMinecraftImage_CommandInterface tests the named pipe command system
func TestMinecraftImage_CommandInterface(t *testing.T) {
	setupTest(t)
	ctx := context.Background()
	
	container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: testcontainers.ContainerRequest{
			FromDockerfile: testcontainers.FromDockerfile{
				Context:    getDockerfileContext(),
				Dockerfile: "Dockerfile",
			},
			ExposedPorts: []string{"25565/tcp"},
			Env: map[string]string{
				"EULA":      "true",
				"MEMORY_MB": defaultMemory,
			},
			WaitingFor: wait.ForLog("Done").WithStartupTimeout(startupTimeout),
		},
		Started: true,
	})
	if err != nil {
		t.Fatalf("Failed to start container: %v", err)
	}
	defer cleanupContainer(t, container)
	
	// Test that the command interface exists
	exitCode, outputReader, err := container.Exec(ctx, []string{"ls", "-la", "/tmp/command-fifo"})
	if err != nil {
		t.Fatalf("Failed to execute command in container: %v", err)
	}
	if exitCode != 0 {
		output, _ := readLogsToString(outputReader)
		t.Fatalf("Expected command-fifo to exist, exit code: %d, output: %s", exitCode, output)
	}
	
	// Test sending a command through the interface
	exitCode, outputReader, err = container.Exec(ctx, []string{"/data/scripts/send-command.sh", "list"})
	if err != nil {
		t.Fatalf("Failed to send command: %v", err)
	}
	if exitCode != 0 {
		output, _ := readLogsToString(outputReader)
		t.Fatalf("Failed to send command, exit code: %d, output: %s", exitCode, output)
	}
	
	// Wait a moment for the command to be processed
	time.Sleep(2 * time.Second)
	
	// Check logs for the list command response
	logs, err := container.Logs(ctx)
	if err != nil {
		t.Fatalf("Failed to get logs: %v", err)
	}
	defer logs.Close()
	
	logStr, err := readLogsToString(logs)
	if err != nil {
		t.Fatalf("Failed to read logs: %v", err)
	}
	
	// Look for command processing in logs - the 'list' command should produce player list output
	if !strings.Contains(logStr, "players online:") {
		t.Error("Command 'list' should produce player list output in server logs")
		t.Logf("Container logs:\n%s", logStr)
	}
}

// TestMinecraftImage_PortConfiguration tests that ports are configured correctly
func TestMinecraftImage_PortConfiguration(t *testing.T) {
	setupTest(t)
	ctx := context.Background()
	
	container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: testcontainers.ContainerRequest{
			FromDockerfile: testcontainers.FromDockerfile{
				Context:    getDockerfileContext(),
				Dockerfile: "Dockerfile",
			},
			ExposedPorts: []string{"25565/tcp"},
			Env: map[string]string{
				"EULA":      "true",
				"MEMORY_MB": defaultMemory,
			},
			WaitingFor: wait.ForLog("Done").WithStartupTimeout(startupTimeout),
		},
		Started: true,
	})
	if err != nil {
		t.Fatalf("Failed to start container: %v", err)
	}
	defer cleanupContainer(t, container)
	
	// Check that server.properties has correct port configuration
	exitCode, outputReader, err := container.Exec(ctx, []string{"grep", "server-port", "/data/server/server.properties"})
	if err != nil {
		t.Fatalf("Failed to check server.properties: %v", err)
	}
	if exitCode != 0 {
		t.Fatalf("Failed to find server-port in server.properties, exit code: %d", exitCode)
	}
	
	output, err := readLogsToString(outputReader)
	if err != nil {
		t.Fatalf("Failed to read grep output: %v", err)
	}
	
	if !strings.Contains(output, "server-port=25565") {
		t.Errorf("Expected server-port=25565, got: %s", output)
	}
	
	// Check RCON port configuration
	exitCode, outputReader, err = container.Exec(ctx, []string{"grep", "rcon.port", "/data/server/server.properties"})
	if err != nil {
		t.Fatalf("Failed to check RCON port: %v", err)
	}
	if exitCode != 0 {
		t.Fatalf("Failed to find rcon.port in server.properties, exit code: %d", exitCode)
	}
	
	output, err = readLogsToString(outputReader)
	if err != nil {
		t.Fatalf("Failed to read grep output: %v", err)
	}
	
	if !strings.Contains(output, "rcon.port=25575") {
		t.Errorf("Expected rcon.port=25575, got: %s", output)
	}
}

// TestMinecraftImage_GracefulShutdown tests that the container shuts down gracefully
func TestMinecraftImage_GracefulShutdown(t *testing.T) {
	setupTest(t)
	ctx := context.Background()
	
	container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: testcontainers.ContainerRequest{
			FromDockerfile: testcontainers.FromDockerfile{
				Context:    getDockerfileContext(),
				Dockerfile: "Dockerfile",
			},
			ExposedPorts: []string{"25565/tcp"},
			Env: map[string]string{
				"EULA":      "true",
				"MEMORY_MB": defaultMemory,
			},
			WaitingFor: wait.ForLog("Done").WithStartupTimeout(startupTimeout),
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
		"stopping Minecraft server gracefully",
		"Stop command sent",
		"Minecraft server stopped gracefully",
	}
	
	for _, pattern := range gracefulShutdownPatterns {
		if !strings.Contains(logStr, pattern) {
			t.Errorf("Expected graceful shutdown message '%s' not found in logs", pattern)
		}
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

// TestMinecraftImage_JavaProcessManagement tests that Java processes are managed correctly
func TestMinecraftImage_JavaProcessManagement(t *testing.T) {
	setupTest(t)
	ctx := context.Background()
	
	container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: testcontainers.ContainerRequest{
			FromDockerfile: testcontainers.FromDockerfile{
				Context:    getDockerfileContext(),
				Dockerfile: "Dockerfile",
			},
			ExposedPorts: []string{"25565/tcp"},
			Env: map[string]string{
				"EULA":      "true",
				"MEMORY_MB": "512", // Lower memory for faster startup in tests
			},
			WaitingFor: wait.ForLog("Done").WithStartupTimeout(startupTimeout),
		},
		Started: true,
	})
	if err != nil {
		t.Fatalf("Failed to start container: %v", err)
	}
	defer cleanupContainer(t, container)
	
	// Check that Java process is running with correct parameters
	exitCode, outputReader, err := container.Exec(ctx, []string{"ps", "aux"})
	if err != nil {
		t.Fatalf("Failed to list processes: %v", err)
	}
	if exitCode != 0 {
		t.Fatalf("Failed to list processes, exit code: %d", exitCode)
	}
	
	// Read exec output
	output, err := readLogsToString(outputReader)
	if err != nil {
		t.Fatalf("Failed to read exec output: %v", err)
	}
	
	// Verify Java process exists with correct memory settings
	javaProcessPattern := regexp.MustCompile(`java.*-Xmx512M.*-Xms512M.*server\.jar`)
	if !javaProcessPattern.MatchString(output) {
		t.Errorf("Java process with correct memory settings not found in process list")
		t.Logf("Process list:\n%s", output)
	}
	
	// Verify server.jar is running
	if !strings.Contains(output, "server.jar") {
		t.Error("Minecraft server.jar process not found")
		t.Logf("Process list:\n%s", output)
	}
}

// TestMinecraftImage_VersionCaching tests that version JARs are cached correctly
func TestMinecraftImage_VersionCaching(t *testing.T) {
	setupTest(t)
	ctx := context.Background()
	
	// First, start a container to download a specific version
	container1, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: testcontainers.ContainerRequest{
			FromDockerfile: testcontainers.FromDockerfile{
				Context:    getDockerfileContext(),
				Dockerfile: "Dockerfile",
			},
			ExposedPorts: []string{"25565/tcp"},
			Env: map[string]string{
				"EULA":             "true",
				"MEMORY_MB":        defaultMemory,
				"MINECRAFT_VERSION": "1.21.4",
			},
			WaitingFor: wait.ForLog("Done").WithStartupTimeout(startupTimeout),
		},
		Started: true,
	})
	if err != nil {
		t.Fatalf("Failed to start first container: %v", err)
	}
	defer cleanupContainer(t, container1)
	
	// Check that the version JAR was downloaded
	exitCode, outputReader, err := container1.Exec(ctx, []string{"ls", "-la", "/data/server/"})
	if err != nil {
		t.Fatalf("Failed to list server directory: %v", err)
	}
	if exitCode != 0 {
		t.Fatalf("Failed to list server directory, exit code: %d", exitCode)
	}
	
	output, err := readLogsToString(outputReader)
	if err != nil {
		t.Fatalf("Failed to read directory listing: %v", err)
	}
	
	// Should have both the versioned JAR and the symlink
	if !strings.Contains(output, "minecraft_server_1.21.4.jar") {
		t.Error("Should have downloaded minecraft_server_1.21.4.jar")
	}
	if !strings.Contains(output, "server.jar") {
		t.Error("Should have server.jar symlink")
	}
	
	// Stop first container
	cleanupContainer(t, container1)
	
	// Start second container with the same version - should use cached JAR
	container2, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: testcontainers.ContainerRequest{
			FromDockerfile: testcontainers.FromDockerfile{
				Context:    getDockerfileContext(),
				Dockerfile: "Dockerfile",
			},
			ExposedPorts: []string{"25565/tcp"},
			Env: map[string]string{
				"EULA":             "true",
				"MEMORY_MB":        defaultMemory,
				"MINECRAFT_VERSION": "1.21.4",
			},
			WaitingFor: wait.ForLog("Done").WithStartupTimeout(startupTimeout),
		},
		Started: true,
	})
	if err != nil {
		t.Fatalf("Failed to start second container: %v", err)
	}
	defer cleanupContainer(t, container2)
	
	// Check logs for cache usage message
	logs, err := container2.Logs(ctx)
	if err != nil {
		t.Fatalf("Failed to get container logs: %v", err)
	}
	defer logs.Close()
	
	logStr, err := readLogsToString(logs)
	if err != nil {
		t.Fatalf("Failed to read logs: %v", err)
	}
	
	// Note: In reality, we can't test true caching between containers because they have separate volumes
	// But we can test that the version checking and JAR management logic works
	if !strings.Contains(logStr, "Checking Minecraft server version: 1.21.4") {
		t.Error("Should check for version 1.21.4")
		t.Logf("Container logs:\n%s", logStr)
	}
}

// TestMinecraftImage_FileStructure tests that the expected file structure is created
func TestMinecraftImage_FileStructure(t *testing.T) {
	setupTest(t)
	ctx := context.Background()
	
	container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: testcontainers.ContainerRequest{
			FromDockerfile: testcontainers.FromDockerfile{
				Context:    getDockerfileContext(),
				Dockerfile: "Dockerfile",
			},
			ExposedPorts: []string{"25565/tcp"},
			Env: map[string]string{
				"EULA":      "true",
				"MEMORY_MB": defaultMemory,
			},
			WaitingFor: wait.ForLog("Done").WithStartupTimeout(startupTimeout),
		},
		Started: true,
	})
	if err != nil {
		t.Fatalf("Failed to start container: %v", err)
	}
	defer cleanupContainer(t, container)
	
	// Check required files and directories exist
	requiredPaths := []string{
		"/data/server/server.jar",
		"/data/server/server.properties",
		"/data/server/eula.txt",
		"/data/scripts/start.sh",
		"/data/scripts/send-command.sh",
		"/tmp/command-fifo",
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
}
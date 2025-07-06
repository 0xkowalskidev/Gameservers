# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

# Gameserver Management Control Panel

## Overview
A minimal, Docker-based gameserver management control panel built with Go, HTMX, and Tailwind CSS. Similar to Pterodactyl but focusing on simplicity and locality of behavior.

## Core Architecture

### Technology Stack
- **Backend**: Go with Chi router and zerolog for structured logging
- **Frontend**: HTMX + Tailwind CSS (embedded static files)
- **Container Runtime**: Docker API with automatic image pulling
- **Database**: SQLite with database/sql
- **Package Manager**: Nix flake

### Design Principles
1. **Single Binary**: Everything compiled into one executable
2. **Clean Architecture**: Well-organized packages (database/, docker/, handlers/, models/, services/)
3. **Interface-Driven Design**: Heavy use of interfaces for testability and modularity
4. **Template Files**: HTML templates in templates/ directory
5. **Simple State Management**: SQLite for persistence, in-memory for runtime
6. **No Authentication**: Direct access (handle externally if needed)
7. **Comprehensive Testing**: Nearly every Go file has corresponding tests

## Features

### Phase 1: Core Functionality
- [x] Docker container management (start/stop/restart)
- [x] Real-time console output (via SSE log streaming)
- [x] Resource monitoring via SSE (CPU, RAM)
- [x] Gameserver CRUD

### Phase 2: Enhanced Features
- [x] Advanced file manager with editor
- [x] Backup/restore functionality
- [x] Schedule tasks (restarts, backups)
- [x] Console integration (unified command interface)

### Phase 3: Future File Manager Enhancements
- [ ] File upload functionality (drag & drop support)
- [ ] Multi-select for batch file operations
- [ ] File preview for logs and text files
- [ ] Zip archive operations (create/extract)
- [ ] Move/cut files and directories between folders

## Current TODOs
- None at the moment! 🎉
- Smart pull strategy implemented - Docker now checks remote image digests and only pulls when there's a newer version available

## Recent Improvements
- **✅ Port Assignment System**: Implemented our own port allocator that reserves ports for gameservers before Docker container creation. This ensures consistent port assignments across server restarts and allows us to "reserve" ports even when containers are stopped/deleted. Ports are assigned during gameserver creation and persisted in the database.

## Recent Improvements (cont'd)
- **✅ Port Allocator Refactoring**: The PortAllocator has been properly extracted into `models/port.go` with comprehensive unit tests, following clean separation of concerns.

## File Structure
```
/
├── main.go              # Entry point, Chi router setup
├── errors.go            # HTTP error handling utilities
├── errors_test.go       # Tests for error handling
├── database/            # Database layer
│   ├── manager.go       # Database connection management
│   ├── games.go         # Game CRUD operations
│   ├── gameservers.go   # Gameserver CRUD operations
│   ├── tasks.go         # Scheduled tasks CRUD
│   ├── service.go       # Database service implementation
│   └── *_test.go        # Comprehensive test coverage
├── docker/              # Docker API interactions
│   ├── client.go        # Docker client initialization
│   ├── containers.go    # Container management
│   ├── files.go         # File operations via Docker
│   ├── images.go        # Image management
│   ├── volumes.go       # Volume management
│   ├── backup.go        # Backup/restore functionality
│   └── *_test.go        # Test files for each module
├── handlers/            # HTTP request handlers
│   ├── common.go        # Common handler utilities
│   ├── gameserver.go    # Gameserver CRUD handlers
│   ├── console.go       # Console streaming handlers (SSE)
│   ├── files.go         # File manager handlers
│   ├── backup.go        # Backup handlers
│   ├── tasks.go         # Task scheduling handlers
│   └── *_test.go        # Handler tests
├── models/              # Data models and interfaces
│   ├── interfaces.go    # Service interfaces
│   ├── gameserver.go    # Gameserver model
│   ├── game.go          # Game model
│   ├── port.go          # Port allocation logic
│   ├── task.go          # Scheduled task model
│   ├── file.go          # File info model
│   ├── volume.go        # Volume info model
│   ├── errors.go        # Model-specific errors
│   └── utils.go         # Utility functions
├── services/            # Business logic layer
│   ├── gameserver.go    # Gameserver service
│   ├── scheduler.go     # Task scheduler implementation
│   ├── cron.go          # Cron expression parser
│   ├── interfaces.go    # Service interfaces
│   ├── requests.go      # Request/response DTOs
│   ├── errors.go        # Service errors
│   └── *_test.go        # Service tests
├── templates/           # HTML templates (HTMX)
│   ├── layout.html      # Base layout
│   ├── index.html       # Dashboard (not dashboard_page.html)
│   ├── components.html  # Reusable components
│   └── [many more templates for each feature]
├── static/              # Static assets (embedded)
│   ├── htmx.js         # HTMX library
│   └── tailwind.css    # Tailwind CSS
├── images/              # Docker images for game servers
│   ├── minecraft/       # Minecraft server image
│   ├── garrysmod/       # Garry's Mod server image
│   └── terraria/        # Terraria server image
├── .github/workflows/   # CI/CD
│   └── build-images.yml # Docker image builds
├── flake.nix           # Nix development environment
├── flake.lock          # Nix lock file
├── go.mod              # Go dependencies
└── go.sum              # Go dependency checksums
```

## API Design (HTMX-focused)

### Implemented Routes
- `GET /` - List gameservers (dashboard)
- `POST /` - Create gameserver
- `GET /new` - Create gameserver form
- `GET /{id}` - Show gameserver details
- `POST /{id}/start` - Start gameserver
- `POST /{id}/stop` - Stop gameserver  
- `POST /{id}/restart` - Restart gameserver
- `DELETE /{id}` - Delete gameserver
- `GET /{id}/console` - Console output (SSE stream)
- `POST /{id}/console` - Send console command
- `GET /{id}/files` - File manager
- `POST /{id}/files` - File operations
- `GET /{id}/backup` - List backups
- `POST /{id}/backup` - Create backup
- `POST /{id}/restore` - Restore from backup
- `GET /tasks` - List scheduled tasks
- `POST /tasks` - Create scheduled task

### RESTful Handler Naming
- `IndexGameservers()`, `NewGameserver()`, `CreateGameserver()`
- `ShowGameserver()`, `DestroyGameserver()`
- Console, files, backup, and task handlers follow similar patterns

## Gameserver Images

### Structure
Each gameserver has its own directory under `gameservers/` containing:
- `Dockerfile` - Image definition
- Startup script - start.sh
- Other standardised files needed for gameserver management

### Image Naming Convention
- Registry: `ghcr.io/0xkowalskidev/gameservers`
- Format: `ghcr.io/0xkowalskidev/gameservers/GAME:VERSION`
- Example: `ghcr.io/0xkowalskidev/gameservers/minecraft:1.20.4`

## Testing Strategy

### Principles
- **Minimal**: Only test critical paths and complex logic
- **Maintainable**: Tests should be easy to update as code changes
- **High Impact**: Focus on areas that would cause significant issues if broken

### Test Structure
- Each file has an associated `_test.go` file (except main.go/other untestable files)
- Example: `docker.go` → `docker_test.go`
- Use table-driven tests for multiple scenarios
- Mock external dependencies (Docker API, SQLite)

### What NOT to Test
- Simple getters/setters
- Direct Docker API calls (trust the Docker client library)
- HTML template rendering (visual testing)
- Third-party library internals

## Development Scripts (via Nix)
```bash
# Enter development shell
nix develop

# Run development server with hot reload and Tailwind compilation
nix run .#dev
# or just 'dev' in the shell

# Run tests with richgo (colored output)
nix run .#test
# dont use 'test' in the shell as its overwritten by internals
```
## Notes
- Use Chi middleware for request logging
- Templates use Go html/template package
- SSE connections auto-reconnect on failure
- Console streaming uses SSE (Server-Sent Events), not WebSocket
- Consider using Alpine.js for light interactivity
- File operations use Docker cp API

## Memories
- Call it Gameserver or Gameservers, not GameServer or Server
- Never use go test ./... as it will activate the images tests which take 20 minutes+

## Codebase Quality Assessment

### Strengths
- **Excellent Test Coverage**: Nearly every Go file has comprehensive tests
- **Clean Architecture**: Clear separation of concerns with dedicated packages
- **Interface-Driven Design**: Enables easy testing and modularity
- **Consistent Error Handling**: Centralized HTTP-aware error types
- **Embedded Assets**: Single binary deployment with embedded templates/static files
- **Structured Logging**: Consistent use of zerolog throughout

### Suggested Improvements

1. **Configuration Management**
   - Add `config/config.go` for environment-based configuration
   - Support env vars for database path, port, Docker socket
   - Add configuration validation on startup

2. **API Documentation**
   - Consider OpenAPI/Swagger for HTMX endpoints
   - Document expected request/response formats
   - Add inline godoc comments for exported functions

3. **Database Migrations**
   - Implement versioned migration system (e.g., golang-migrate)
   - Track applied migrations in database
   - Support rollback functionality

4. **Observability**
   - Add `/metrics` endpoint for Prometheus
   - Include `/health` and `/ready` endpoints
   - Add request tracing with correlation IDs

5. **Security Enhancements**
   - Implement rate limiting middleware
   - Add CORS configuration options
   - Sanitize console command inputs
   - Consider adding optional authentication layer

6. **Developer Experience**
   - Add Makefile for common tasks
   - Include docker-compose.yml for local development
   - Set up pre-commit hooks for code quality
   - Add README.md with setup instructions

7. **WebSocket Implementation**
   - Consider implementing proper WebSocket for bidirectional console
   - Current SSE works well for logs but limits interactivity

8. **Backup Strategy**
   - Add scheduled automatic backups
   - Implement backup retention policies
   - Support remote backup destinations (S3, etc.)

### Architecture Notes
The codebase follows a well-structured layered architecture:
- **HTTP Layer** (main.go, handlers/): Request routing and handling
- **Service Layer** (services/): Business logic orchestration
- **Data Access** (database/): SQLite operations and migrations
- **Docker Integration** (docker/): Container management
- **Models** (models/): Shared data structures and interfaces

This architecture promotes testability, maintainability, and clear separation of concerns.
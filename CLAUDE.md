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
- **Database**: SQLite with GORM
- **Package Manager**: Nix flake

### Design Principles
1. **Single Binary**: Everything compiled into one executable with embedded templates/static files
2. **Clean Architecture**: Well-organized packages with clear separation of concerns
3. **Interface-Driven Design**: Heavy use of interfaces for testability and modularity
4. **Simple State Management**: SQLite for persistence, in-memory for runtime
5. **No Authentication**: Direct access (handle externally if needed)
6. **Comprehensive Testing**: Nearly every Go file has corresponding tests

### Layered Architecture
The codebase follows a simplified 2-layer architecture:

```
┌─────────────────────────────────────┐
│  HTTP Layer (main.go, handlers/)    │  ← Chi routes, request handling, workflows
├─────────────────────────────────────┤
│  Repository Layer (database/)       │  ← Data access + Docker orchestration
├─────────────────────────────────────┤
│  Docker Layer (docker/)             │  ← Container, volume, image management
├─────────────────────────────────────┤
│  Models (models/)                   │  ← Shared structs, utilities
└─────────────────────────────────────┘
```

**Key Points:**
- **Single Repository Layer**: `database/repository.go` handles both data access and Docker orchestration
- **Concrete Types**: Direct dependency injection without unnecessary interfaces
- **Dependency Flow**: HTTP → GameserverRepository → Docker Manager
- **Port Allocation**: Simplified port allocator in `models/port.go` uses static range (49152-65535)

## Development Commands

### Nix Environment
```bash
# Enter development shell (loads all tools)
nix develop

# Run dev server with hot reload + Tailwind compilation
nix run .#dev
# or just 'dev' inside nix shell

# Run application tests (excludes slow image tests)
nix run .#test

# Run Docker image integration tests (slow, requires Docker)
nix run .#test-images

# Run all tests (application + images)
nix run .#test-all
```

### Manual Commands
```bash
# Run specific package tests
go test ./handlers/...
go test ./database/...

# Run single test
go test -run TestCreateGameserver ./handlers/

# Build binary
go build -o gameservers .

# Run server
go run .

# Set debug logging
DEBUG=1 go run .
```

## Configuration

All configuration is done via environment variables with sensible defaults:

```bash
# Server
GAMESERVER_HOST=localhost                    # default: localhost
GAMESERVER_PORT=3000                        # default: 3000
GAMESERVER_PUBLIC_ADDRESS=play.example.com  # default: localhost (public IP/domain for connection details)
GAMESERVER_SHUTDOWN_TIMEOUT=30s             # default: 30s

# Database
GAMESERVER_DATABASE_PATH=gameservers.db     # default: gameservers.db

# Docker
GAMESERVER_DOCKER_SOCKET=                   # default: empty (uses Docker default)
GAMESERVER_CONTAINER_NAMESPACE=gameservers  # default: gameservers
GAMESERVER_CONTAINER_STOP_TIMEOUT=30s       # default: 30s

# File Operations
GAMESERVER_MAX_FILE_EDIT_SIZE=10485760      # default: 10MB
GAMESERVER_MAX_UPLOAD_SIZE=104857600        # default: 100MB
```

## Gameserver Docker Images

### Structure
Each gameserver type has a directory under `images/` containing:
- `Dockerfile` - Image definition
- `start.sh` - Startup script
- `send-command.sh` - Command injection script
- `*_test.go` - Integration tests (slow, require Docker)
- Game-specific config files

### Registry Convention
- Registry: `registry.0xkowalski.dev/gameservers`
- Format: `registry.0xkowalski.dev/gameservers/GAME:VERSION`
- Example: `registry.0xkowalski.dev/gameservers/minecraft:1.20.4`

### Available Games
Current images: minecraft, terraria, garrysmod, palworld, rust, valheim, ark-survival-evolved

### Planned Games
Games to add (in priority order):
1. 7daystodie - 7 Days to Die (SteamCMD)
2. cs2 - Counter-Strike 2 (SteamCMD)
3. projectzomboid - Project Zomboid (SteamCMD)
4. factorio - Factorio (native Linux headless, no Steam required)
5. satisfactory - Satisfactory (SteamCMD)
6. dontstarvetogether - Don't Starve Together (SteamCMD)
7. left4dead2 - Left 4 Dead 2 (SteamCMD, Source engine)
8. conanexiles - Conan Exiles (SteamCMD)

### Adding New Games

See `images/README.md` for the full contributor guide. Quick reference:

**SteamCMD games** (most common):
- Base: `steamcmd/steamcmd:debian-bookworm`
- Use `rcon-cli` for command interface if game supports RCON
- Run as non-root `steam` user

**Non-SteamCMD games**:
- Base: `ubuntu:22.04` (Java), `mono:*` (.NET), `alpine` (native binaries)
- Use named pipe (FIFO) for command interface
- Download server at build time or startup depending on versioning needs

**Required patterns**:
- Graceful shutdown via SIGTERM trap in start.sh
- Standard directory structure: `/data/server`, `/data/backups`, `/data/scripts`
- `send-command.sh` for games that support external commands

## Key Implementation Details

### Port Allocation System
- Custom allocator in `models/port.go` reserves ports during gameserver creation
- Ports persist in database, survive container restarts/recreation
- Avoids Docker's dynamic port assignment issues

### Template Rendering (main.go)
- HTMX requests (`HX-Request: true` header) get partial templates
- Full page requests get wrapped in `layout.html`
- Custom template functions: `formatFileSize`, `dict`, `slice`, `gt`, `mul`, `div`, etc.

### Docker Integration
- Smart image pulling: checks remote digest, only pulls if newer version exists
- Volume-based persistence: each gameserver gets its own named volume
- Backup/restore: tar-based snapshots to `/data/backups/`
- File operations: uses Docker API (`docker cp` equivalent)

### Task Scheduler
- Cron-like scheduling in `services/scheduler.go`
- Supports: restart, backup, stop, start actions
- Runs in background goroutine, checks every minute
- Cron expression parser in `services/cron.go`

### Error Handling
- `models/errors.go`: Domain-specific errors (DatabaseError, ValidationError)
- `services/errors.go`: HTTP-aware errors with status codes
- `errors.go` (root): HTTP error handlers used by handlers package

## Testing Strategy

**Note**: All tests have been removed from the codebase (except image integration tests in `/images`). Verification is done manually or through smoke testing.

## Important Notes

### Terminology
- Call it "Gameserver" or "Gameservers", not "GameServer" or "Server"

### HTMX vs Alpine.js Usage
- **HTMX**: Navigation (`hx-get` with `hx-push-url`) and RESTful CRUD operations (form submissions, deletions)
- **Alpine.js**: UI state management, SSE streaming (logs/stats), server actions (start/stop/restart), status polling
- SSE streaming uses native EventSource via Alpine components (not htmx-sse extension)
- Status/query polling uses Alpine fetch + setInterval
- SSE endpoints: `/{id}/stats`, `/{id}/logs` - both return JSON data for Alpine consumption

### Database
- Uses GORM for ORM operations
- Auto-migration in `database/manager.go`
- Models in `models/` package have GORM tags
- Repository pattern in `database/repository.go` for data access

### File Operations
- File manager: browse, edit, download, upload, rename, delete
- Edit size limit: 10MB (configurable)
- Upload size limit: 100MB (configurable)
- Uses Docker API for all file operations (not host filesystem)

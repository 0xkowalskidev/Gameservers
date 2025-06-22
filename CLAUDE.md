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
2. **Locality of Behavior**: Keep related code together
3. **Minimal File Structure**: Avoid excessive directory nesting
4. **Template Files**: HTML templates in templates/ directory
5. **Simple State Management**: SQLite for persistence, in-memory for runtime
6. **No Authentication**: Direct access (handle externally if needed)

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

## Future Improvements
- **Port Allocator Refactoring**: Consider moving the PortAllocator from models.go into its own dedicated file (e.g., `port_allocator.go`) with comprehensive unit tests. Currently it's embedded in models.go but could benefit from standalone testing and cleaner separation of concerns.

## File Structure
```
/
├── main.go           # Entry point, Chi router setup
├── docker.go         # Docker API interactions
├── database.go       # SQLite database layer, migrations, and GameServer CRUD operations
├── handlers.go      # HTTP handlers (HTMX endpoints)
├── websocket.go      # Console streaming
├── templates/        # HTML templates
│   ├── layout.html   # Base layout with HTMX
│   ├── dashboard_page.html    # Dashboard
├── static/           # CSS/JS assets
├── images/      # Game server Docker images
│   ├── minecraft/    # Minecraft server
│   │   ├── Dockerfile
│   │   ├── start.sh  # Startup script
│   ├── cs2/          # Counter-Strike 2
│   │   ├── Dockerfile
│   │   └── start.sh
│   ├── valheim/      # Valheim server
│   └── ...           # Other game servers
├── .github/
│   └── workflows/
│       └── build-images.yml  # Docker image builds
├── flake.nix         # Nix development environment
├── flake.lock        # Nix lock file
└── go.mod            # Go dependencies
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

### RESTful Handler Naming
- `IndexGameservers()`, `NewGameserver()`, `CreateGameserver()`
- `ShowGameserver()`, `DestroyGameserver()`

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
- WebSocket has heartbeat for connection health
- Consider using Alpine.js for light interactivity
- File operations use Docker cp API

## Memories
- Call it Gameserver or Gameservers, not GameServer or Server

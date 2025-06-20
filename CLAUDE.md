# Gameserver Management Control Panel

## Overview
A minimal, Docker-based gameserver management control panel built with Go, HTMX, and Tailwind CSS. Similar to Pterodactyl but focusing on simplicity and locality of behavior.

## Core Architecture

### Technology Stack
- **Backend**: Go with Chi router
- **Frontend**: HTMX + Tailwind CSS
- **Container Runtime**: Docker API
- **Database**: SQLite 
- **Real-time**: WebSockets (console/File management) + SSE (stats/logs)
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
- [ ] Docker container management (start/stop/restart)
- [ ] Real-time console output via WebSockets
- [ ] Resource monitoring via SSE (CPU, RAM, disk)
- [ ] Gameserver CRUD

### Phase 2: Enhanced Features
- [ ] Advanced file manager with editor
- [ ] Backup/restore functionality
- [ ] Schedule tasks (restarts, backups)
- [ ] Console with Log viewer with SSE streaming + RCON/other with sending command when possible

## Current TODOs
- [ ] Implement SQLite database layer with GameServer CRUD operations
- [ ] Create database schema and migration for GameServer table
- [ ] Integrate database with Docker manager for persistent gameserver storage
- [ ] Add HTTP handlers for gameserver CRUD operations

## File Structure
```
/
├── main.go           # Entry point, Chi router setup
├── docker.go         # Docker API interactions
├── database.go       # SQLite database layer, migrations, and GameServer CRUD operations
├── handler.go       # HTTP handlers (HTMX endpoints)
├── websocket.go      # Console streaming
├── templates/        # HTML templates
│   ├── layout.html   # Base layout with HTMX
│   ├── dashboard_page.html    # Dashboard
├── static/           # CSS/JS assets
├── gameservers/      # Game server Docker images
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

- `GET /` - Dashboard (server list)
- `GET /{id}` - Server details
- `GET /{id}/console` - Console view
- `GET /{id}/files` - File manager
- `GET /new` - Create server form
- etc

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
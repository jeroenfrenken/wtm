# Configuration Reference

This document describes all configuration options for worktree-manager.

## Configuration File

The configuration file must be named `wtm.yaml` and placed in your project root. The tool searches for this file starting from the current directory and walking up to the filesystem root.

## Root Options

| Option | Type | Default | Description |
|--------|------|---------|-------------|
| `project` | string | **required** | Project name, used in domain generation |
| `domain` | string | `"local"` | Base domain for local routing |
| `proxy_port` | int | `8081` | Port for the reverse proxy |
| `ui_port` | int | `9000` | Port for the web UI |
| `port_range_start` | int | `3001` | Starting port for services |
| `env_files` | []string | `[".env"]` | Environment files to load |
| `hooks` | object | | Project-level lifecycle hooks |
| `services` | object | | Service definitions |
| `tasks` | object | | One-off task definitions |
| `terminal` | object | | Terminal emulator configuration |

## Port Allocation

Ports are allocated using this formula:
```
port = port_range_start + (worktree_index * 100) + service_port_offset
```

For example, with `port_range_start: 3001`:
- Worktree 0 (main): api=3001, web=3002
- Worktree 1 (feature-a): api=3101, web=3102
- Worktree 2 (feature-b): api=3201, web=3202

## Project Hooks

```yaml
hooks:
  on_create:
    - "npm install"
    - "cp .env.example .env.local"
  on_delete:
    - "docker compose down"
```

| Hook | When | Use Case |
|------|------|----------|
| `on_create` | After worktree is created | Install dependencies, copy config files |
| `on_delete` | Before worktree is removed | Clean up Docker containers, temp files |

Hooks run in a terminal window (Ghostty, iTerm, etc.) so you can see output and interact if needed.

## Services

```yaml
services:
  api:
    run: "npm run dev"
    working_dir: "./apps/api"
    env:
      PORT: "$PORT"
    port_offset: 0
    needs_port: true
    depends_on:
      - postgres
    hooks:
      on_start: []
      on_stop: []
      on_restart: []
      on_crash:
        commands: []
        auto_restart: true
        max_restarts: 3
        restart_delay: 2s
    health_check:
      type: http
      url: "http://localhost:$PORT/health"
      interval: 2s
      timeout: 30s
```

### Service Options

| Option | Type | Default | Description |
|--------|------|---------|-------------|
| `run` | string | **required** | Command to run |
| `working_dir` | string | project root | Working directory (relative to worktree) |
| `env` | object | | Additional environment variables |
| `port_offset` | int | `0` | Offset from base port for this worktree |
| `needs_port` | bool | `true` | Whether this service needs a port |
| `depends_on` | []string | | Services that must be healthy first |
| `hooks` | object | | Service lifecycle hooks |
| `health_check` | object | | Health check configuration |

### Service Hooks

| Hook | When | Description |
|------|------|-------------|
| `on_start` | Before service starts | Run migrations, generate code |
| `on_stop` | After service stops | Cleanup tasks |
| `on_restart` | Before auto-restart | Regenerate on restart |
| `on_crash` | When service exits unexpectedly | Log errors, notify |

### on_crash Configuration

```yaml
on_crash:
  - "echo 'Service crashed!'"
  auto_restart: true
  max_restarts: 3
  restart_delay: 2s
```

| Option | Type | Default | Description |
|--------|------|---------|-------------|
| commands | []string | | Commands to run on crash |
| `auto_restart` | bool | `false` | Automatically restart on crash |
| `max_restarts` | int | `3` | Max restart attempts before giving up |
| `restart_delay` | duration | `0s` | Delay before restarting |

### Health Checks

Three types of health checks are supported:

#### HTTP Health Check

```yaml
health_check:
  type: http
  url: "http://localhost:$PORT/health"
  interval: 2s
  timeout: 30s
```

Sends a GET request and expects a 2xx response.

#### Port Health Check

```yaml
health_check:
  type: port
  port: 5432  # optional, defaults to service port
  interval: 1s
  timeout: 30s
```

Attempts to open a TCP connection.

#### Log Match Health Check

```yaml
health_check:
  type: log_match
  pattern: "Server listening on port"
  timeout: 15s
```

Scans service stdout for the pattern (supports regex).

### Health Check Options

| Option | Type | Default | Description |
|--------|------|---------|-------------|
| `type` | string | **required** | `http`, `port`, or `log_match` |
| `url` | string | | URL for http checks |
| `port` | int | service port | Port for port checks |
| `pattern` | string | | Regex pattern for log_match |
| `interval` | duration | `2s` | Check interval |
| `timeout` | duration | `30s` | Total timeout to become healthy |

## Tasks

```yaml
tasks:
  db-migrate:
    run: "npx prisma migrate dev"
    working_dir: "./apps/api"
    description: "Run database migrations"
```

| Option | Type | Default | Description |
|--------|------|---------|-------------|
| `run` | string | **required** | Command to run |
| `working_dir` | string | project root | Working directory |
| `description` | string | | Description shown in help and UI |

Run tasks with `wtm task <name>`.

## Terminal Configuration

```yaml
terminal:
  emulator: ghostty
  profile: Dev
```

| Option | Type | Default | Description |
|--------|------|---------|-------------|
| `emulator` | string | auto-detect | Terminal emulator to use |
| `profile` | string | | Terminal profile/theme name |

### Supported Emulators

**macOS:**
- `ghostty` - Ghostty
- `iterm` - iTerm2
- `terminal` - Terminal.app

**Linux:**
- `gnome-terminal` - GNOME Terminal
- `konsole` - KDE Konsole
- `kitty` - Kitty
- `x-terminal-emulator` - System default

If not specified, the terminal is auto-detected using:
1. `$TERM_PROGRAM` environment variable
2. Platform default (Terminal.app on macOS, x-terminal-emulator on Linux)

## Variable Substitution

The following variables are available in `env`, `run`, and hook commands:

| Variable | Description |
|----------|-------------|
| `$PORT` | Allocated port for this service |
| `$WORKTREE` | Current worktree name |
| `$PROJECT` | Project name from config |
| `$DOMAIN` | Base domain from config |
| `$PROXY_PORT` | Proxy port from config |

Plus all variables from loaded `.env` files and system environment.

## State Directory

worktree-manager stores state in `.wtm/`:

```
.wtm/
├── state.json          # Worktree registry
├── env/
│   └── <worktree>.env  # Per-worktree env overrides
├── logs/               # Log persistence (optional)
└── pids/               # PID files
```

This directory should be added to `.gitignore`.

## Example: Full Configuration

```yaml
project: myapp
domain: myapp.local
proxy_port: 8081
ui_port: 9000
port_range_start: 3001

env_files:
  - .env
  - .env.local

terminal:
  emulator: ghostty
  profile: Dev

hooks:
  on_create:
    - "npm install"
    - "npx prisma generate"
  on_delete:
    - "docker compose down"

services:
  postgres:
    run: "docker compose up postgres"
    health_check:
      type: port
      port: 5432
      timeout: 30s

  api:
    run: "npm run dev"
    working_dir: "./apps/api"
    env:
      PORT: "$PORT"
      DATABASE_URL: "postgresql://localhost:5432/$PROJECT_$WORKTREE"
    depends_on:
      - postgres
    hooks:
      on_start:
        - "npx prisma migrate deploy"
      on_crash:
        auto_restart: true
        max_restarts: 3
        restart_delay: 2s
    health_check:
      type: http
      url: "http://localhost:$PORT/health"
      interval: 2s
      timeout: 30s

  web:
    run: "npm run dev"
    working_dir: "./apps/web"
    port_offset: 1
    depends_on:
      - api

tasks:
  db-migrate:
    run: "npx prisma migrate dev"
    working_dir: "./apps/api"
    description: "Run database migrations"

  test:
    run: "npm test"
    description: "Run all tests"
```

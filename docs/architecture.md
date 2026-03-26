# Architecture

This document describes the architecture of worktree-manager.

## Overview

```
┌─────────────────────────────────────────────────────────────┐
│                        Web UI (:9000)                        │
│     Dashboard │ Logs │ Env Editor │ Service Controls        │
└─────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────┐
│                    Reverse Proxy (:8081)                     │
│  api.feature.myapp.local → :3001                            │
│  web.feature.myapp.local → :3002                            │
└─────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────┐
│                    Process Supervisor                        │
│  ┌─────────┐  ┌─────────┐  ┌─────────┐                     │
│  │   api   │  │   web   │  │ worker  │                     │
│  │  :3001  │  │  :3002  │  │  (bg)   │                     │
│  └─────────┘  └─────────┘  └─────────┘                     │
│       ▲            │                                        │
│       └── depends_on                                        │
└─────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────┐
│                    Git Worktrees                             │
│  main/  │  feature-auth/  │  bugfix-login/                  │
└─────────────────────────────────────────────────────────────┘
```

## Package Structure

```
internal/
├── cli/          # Command-line interface (Cobra)
├── config/       # Configuration loading and validation
├── dns/          # /etc/hosts management
├── process/      # Service process supervision
├── proxy/        # Reverse proxy server
├── task/         # One-off task execution
├── terminal/     # Terminal emulator integration
├── ui/           # Web UI server
└── worktree/     # Git worktree management
```

## Core Components

### Config Package

Responsible for:
- Loading and parsing `wtm.yaml`
- Validating configuration
- Environment variable management
- Variable substitution

Key types:
- `Config`: Root configuration struct
- `Service`: Service definition
- `Task`: Task definition

### Worktree Package

Responsible for:
- Git worktree creation and deletion
- State persistence in `.wtm/state.json`
- Port allocation per worktree
- Hook execution

Key types:
- `Manager`: Worktree operations
- `Registry`: State persistence
- `HookRunner`: Hook execution

### Process Package

Responsible for:
- Service process lifecycle
- Health checking
- Auto-restart on crash
- Dependency resolution

Key types:
- `Supervisor`: Orchestrates multiple services
- `Service`: Single service management
- `HealthManager`: Health check coordination
- `LifecycleHooks`: Hook execution for services

### Proxy Package

Responsible for:
- Reverse proxy for local domains
- Dynamic route management
- WebSocket passthrough

Key types:
- `Proxy`: HTTP reverse proxy
- `Router`: Route matching
- `Route`: Domain to port mapping

### DNS Package

Responsible for:
- Managing /etc/hosts entries
- Adding/removing domains
- Marker-based entry management

Key types:
- `HostsManager`: /etc/hosts operations

### UI Package

Responsible for:
- Web dashboard
- REST API
- WebSocket for real-time updates

Key types:
- `Server`: HTTP server
- `Hub`: WebSocket client management
- `Client`: Single WebSocket connection

### Terminal Package

Responsible for:
- Terminal emulator detection
- Opening new terminal windows/tabs
- Cross-platform support

Key types:
- `Terminal`: Interface for emulators
- `Factory`: Creates appropriate terminal
- Platform-specific implementations

## Data Flow

### Starting Services

1. User runs `wtm start`
2. CLI loads config and identifies current worktree
3. Supervisor resolves service dependencies (topological sort)
4. For each service in order:
   - Wait for dependencies to be healthy
   - Run on_start hooks
   - Start process with environment
   - Begin health checking
5. Supervisor monitors all services

### Service Crash

1. Process exits unexpectedly
2. Supervisor detects exit
3. on_crash hooks run
4. If auto_restart enabled:
   - Wait restart_delay
   - Run on_restart hooks
   - Restart process
   - Increment restart count
5. If max_restarts exceeded:
   - Mark service as failed
   - Emit failure event

### Web UI Request

1. Browser connects to UI server
2. WebSocket connection established
3. Server sends initial status
4. Service logs and events broadcast via WebSocket
5. User actions (start/stop) go through REST API

## State Management

State is persisted in `.wtm/`:

```
.wtm/
├── state.json      # Worktree registry
├── env/
│   └── <wt>.env    # Per-worktree env overrides
├── logs/           # Log persistence (optional)
└── pids/           # PID files
```

### state.json Schema

```json
{
  "worktrees": {
    "feature-auth": {
      "name": "feature-auth",
      "path": "/path/to/.worktrees/feature-auth",
      "branch": "feature/auth",
      "base_port": 3101,
      "index": 1
    }
  },
  "next_index": 2,
  "port_range_start": 3001
}
```

## Port Allocation

Ports are allocated using:
```
port = port_range_start + (worktree_index * 100) + service_port_offset
```

Example with `port_range_start: 3001`:
- main (index 0): api=3001, web=3002
- feature-auth (index 1): api=3101, web=3102
- bugfix (index 2): api=3201, web=3202

## Domain Naming

Domains follow the pattern:
```
<service>.<worktree>.<project>.local
```

Example:
- `api.feature-auth.myapp.local`
- `web.main.myapp.local`

## Concurrency

- Supervisor uses goroutines for parallel service management
- errgroup for coordinated startup/shutdown
- sync.RWMutex for thread-safe state access
- Channels for event broadcasting

## Error Handling

- All errors bubble up to CLI
- Hooks can fail without stopping the operation (with warnings)
- Health check failures trigger restart logic
- Graceful shutdown with timeout

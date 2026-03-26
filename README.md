# worktree-manager

A worktree-based development environment manager written in Go.

**worktree-manager** (`wtm`) helps you manage multiple development environments using git worktrees. Run different feature branches in parallel, each with their own service instances, ports, and local domains.

## Features

- **Git Worktree Management** - Create, delete, and switch between worktrees
- **Service Process Supervision** - Run multiple services with health checks and auto-restart
- **Lifecycle Hooks** - Run commands on worktree creation, service start/stop, and crashes
- **Reverse Proxy** - Access services via local domains like `api.feature-auth.myapp.local`
- **Web UI** - Monitor services, view logs, and manage environments from your browser
- **Terminal Integration** - Open terminals in the right directory with your preferred emulator
- **Environment Management** - Load `.env` files with per-worktree overrides

## Installation

### From Source

```bash
# Clone the repository
git clone https://github.com/jeroenfrenken/worktree-manager.git
cd worktree-manager

# Build
make build

# Install to /usr/local/bin
make install
```

### Using Go

```bash
go install github.com/jeroenfrenken/worktree-manager/cmd/wtm@latest
```

## Quick Start

```bash
# Initialize in your project
cd my-project
wtm init

# Edit wtm.yaml to configure your services
# ...

# Create a worktree for a feature branch
wtm create feature-auth --branch feature/auth

# Start all services
wtm start

# Open the web UI
wtm ui

# View logs
wtm logs api --follow

# Stop everything
wtm stop

# Delete the worktree when done
wtm delete feature-auth
```

## Configuration

Create a `wtm.yaml` in your project root:

```yaml
project: myapp
domain: myapp.local
proxy_port: 8081
ui_port: 9000
port_range_start: 3001

env_files:
  - .env
  - .env.local

hooks:
  on_create:
    - "npm install"
  on_delete:
    - "docker compose down"

services:
  api:
    run: "npm run dev"
    working_dir: "./apps/api"
    env:
      PORT: "$PORT"
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
```

See [docs/configuration.md](docs/configuration.md) for the full configuration reference.

## CLI Commands

| Command | Description |
|---------|-------------|
| `wtm init` | Create a new wtm.yaml configuration |
| `wtm create <name>` | Create a new worktree |
| `wtm delete <name>` | Delete a worktree |
| `wtm list` | List all worktrees |
| `wtm start [service...]` | Start all or specific services |
| `wtm stop [service...]` | Stop all or specific services |
| `wtm restart [service...]` | Restart services |
| `wtm status` | Show service status |
| `wtm logs [service]` | View logs |
| `wtm task <name>` | Run a one-off task |
| `wtm ui` | Open the web UI |
| `wtm terminal [service]` | Open a terminal |

See [docs/cli-reference.md](docs/cli-reference.md) for detailed command documentation.

## Architecture

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

## Platform Support

- **macOS** - Full support including AppleScript terminal integration
- **Linux** - Full support with x-terminal-emulator and common terminals

## Documentation

- [Configuration Reference](docs/configuration.md)
- [CLI Reference](docs/cli-reference.md)
- [Architecture](docs/architecture.md)
- [Terminal Integration](docs/terminal-integration.md)

## Development

```bash
# Run tests
make test

# Build for all platforms
make build-all

# Run locally
go run ./cmd/wtm
```

## License

MIT License - see [LICENSE](LICENSE) for details.

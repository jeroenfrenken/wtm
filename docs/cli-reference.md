# CLI Reference

Complete reference for all worktree-manager commands.

## Global Flags

| Flag | Short | Description |
|------|-------|-------------|
| `--config` | `-c` | Path to config file (default: search for wtm.yaml) |
| `--verbose` | `-v` | Enable verbose output |
| `--help` | `-h` | Show help for command |

## Commands

### wtm init

Initialize a new worktree-manager configuration.

```bash
wtm init [project-name] [flags]
```

**Flags:**
- `--force`, `-f`: Overwrite existing config file

**Examples:**
```bash
# Initialize with directory name as project
wtm init

# Initialize with custom project name
wtm init myapp

# Force overwrite existing config
wtm init --force
```

### wtm create

Create a new git worktree.

```bash
wtm create <name> [flags]
```

**Flags:**
- `--branch`, `-b`: Branch name (default: worktree name)
- `--no-hooks`: Skip running on_create hooks

**Examples:**
```bash
# Create worktree with new branch
wtm create feature-auth

# Create worktree from existing branch
wtm create hotfix --branch hotfix/login-fix

# Create without running hooks
wtm create quick-test --no-hooks
```

### wtm delete

Delete a git worktree.

```bash
wtm delete <name> [flags]
```

**Aliases:** `rm`, `remove`

**Flags:**
- `--force`, `-f`: Force delete even with uncommitted changes
- `--no-hooks`: Skip running on_delete hooks
- `--yes`, `-y`: Skip confirmation prompt

**Examples:**
```bash
# Delete with confirmation
wtm delete feature-auth

# Force delete without confirmation
wtm delete feature-auth -f -y
```

### wtm list

List all worktrees.

```bash
wtm list
```

**Aliases:** `ls`

**Output:**
```
Project: myapp
Domain: myapp.local

  NAME          BRANCH         PORT   PATH
● main          main           3001   /path/to/project
○ feature-auth  feature/auth   3101   /path/to/.worktrees/feature-auth

● = current worktree
```

### wtm start

Start services in the current worktree.

```bash
wtm start [service...] [flags]
```

**Examples:**
```bash
# Start all services
wtm start

# Start specific services
wtm start api web

# Services start in dependency order
# Press Ctrl+C to stop all services
```

### wtm stop

Stop running services.

```bash
wtm stop [service...]
```

**Note:** When using `wtm start` in foreground mode, use Ctrl+C to stop services.

### wtm status

Show service status.

```bash
wtm status
```

**Output:**
```
Project: myapp
Worktree: feature-auth
Base port: 3101

SERVICE   PORT   DOMAIN                            DEPENDS ON
api       3101   api.feature-auth.myapp.local      -
web       3102   web.feature-auth.myapp.local      [api]

Run wtm start to start all services.
```

### wtm logs

View service logs.

```bash
wtm logs [service] [flags]
```

**Flags:**
- `--follow`, `-f`: Follow log output
- `--tail`, `-n`: Number of lines to show (default: 100)

**Note:** When services run via `wtm start`, logs stream directly to the terminal.

### wtm task

Run a one-off task.

```bash
wtm task <name>
```

**Examples:**
```bash
# List available tasks
wtm task

# Run a task
wtm task db-migrate

# Run tests
wtm task test
```

### wtm ui

Start the web UI.

```bash
wtm ui [flags]
```

**Flags:**
- `--port`, `-p`: UI port (default: from config or 9000)
- `--no-browser`: Don't open browser automatically
- `--with-start`: Also start all services

**Examples:**
```bash
# Start UI only
wtm ui

# Start UI and services together
wtm ui --with-start

# Custom port
wtm ui --port 3000
```

### wtm terminal

Open a terminal in the worktree.

```bash
wtm terminal [service]
```

**Aliases:** `term`, `shell`

**Examples:**
```bash
# Open terminal in worktree root
wtm terminal

# Open terminal in service directory
wtm terminal api
```

### wtm version

Show version information.

```bash
wtm version
```

## Environment Variables

The following variables are available in commands, hooks, and service environments:

| Variable | Description |
|----------|-------------|
| `$PORT` | Allocated port for the service |
| `$WORKTREE` | Current worktree name |
| `$PROJECT` | Project name from config |
| `$DOMAIN` | Base domain from config |
| `$PROXY_PORT` | Proxy port from config |

## Exit Codes

| Code | Description |
|------|-------------|
| 0 | Success |
| 1 | General error |

## Configuration

See [configuration.md](configuration.md) for the full configuration reference.

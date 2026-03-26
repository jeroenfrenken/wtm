# Terminal Integration

worktree-manager integrates with various terminal emulators to open new windows and tabs.

## Supported Terminals

### macOS

| Terminal | Config Value | Notes |
|----------|--------------|-------|
| Ghostty | `ghostty` | Modern GPU-accelerated terminal |
| iTerm2 | `iterm` | Feature-rich terminal with profiles |
| Terminal.app | `terminal` | Built-in macOS terminal |

### Linux

| Terminal | Config Value | Notes |
|----------|--------------|-------|
| GNOME Terminal | `gnome-terminal` | Default for GNOME desktop |
| Konsole | `konsole` | Default for KDE desktop |
| Kitty | `kitty` | GPU-accelerated terminal |
| x-terminal-emulator | `x-terminal-emulator` | Debian/Ubuntu default |

## Configuration

Set your preferred terminal in `wtm.yaml`:

```yaml
terminal:
  emulator: ghostty
  profile: Dev  # Optional profile name
```

## Auto-Detection

If no terminal is configured, worktree-manager auto-detects:

1. Checks `TERM_PROGRAM` environment variable
2. Checks for installed applications
3. Falls back to platform default

### Detection Order

**macOS:**
1. Ghostty (if installed)
2. iTerm2 (if installed)
3. Terminal.app

**Linux:**
1. gnome-terminal
2. konsole
3. kitty
4. x-terminal-emulator

## Usage

### Open Terminal in Worktree

```bash
wtm terminal
```

Opens a new terminal window in the current worktree's root directory.

### Open Terminal in Service Directory

```bash
wtm terminal api
```

Opens a new terminal in the `api` service's working directory.

## Profiles

Some terminals support profiles (color schemes, fonts, etc.):

### iTerm2

```yaml
terminal:
  emulator: iterm
  profile: MyProfile
```

### GNOME Terminal

```yaml
terminal:
  emulator: gnome-terminal
  profile: Development
```

### Konsole

```yaml
terminal:
  emulator: konsole
  profile: dev-profile
```

## Implementation Details

### macOS Terminals

macOS terminals are controlled via AppleScript:

```applescript
tell application "iTerm2"
    create window with profile "Dev"
    tell current session of current window
        write text "cd /path/to/project"
    end tell
end tell
```

### Linux Terminals

Linux terminals use command-line arguments:

```bash
gnome-terminal --working-directory=/path/to/project
konsole --workdir /path/to/project
kitty --directory /path/to/project
```

## Troubleshooting

### Terminal Not Opening

1. Check if the terminal is installed
2. Verify the config value is correct
3. Try running manually: `ghostty` or `gnome-terminal`

### Wrong Directory

1. Check the service's `working_dir` in config
2. Ensure the path exists
3. Use absolute paths for reliability

### AppleScript Errors (macOS)

1. Enable automation in System Preferences > Privacy & Security > Automation
2. Grant wtm permission to control the terminal app

### No Permissions (Linux)

1. Ensure the terminal command is in PATH
2. Check file permissions on the terminal binary

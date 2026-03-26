package terminal

import (
	"fmt"
	"os"
	"runtime"
)

// Terminal is the interface for terminal emulators
type Terminal interface {
	// OpenWindow opens a new terminal window
	OpenWindow(opts OpenOptions) error
	// OpenTab opens a new tab in an existing window
	OpenTab(opts OpenOptions) error
	// Name returns the terminal name
	Name() string
}

// OpenOptions configures how to open the terminal
type OpenOptions struct {
	Title      string
	WorkingDir string
	Command    string
	Profile    string
}

// Factory creates terminal instances based on configuration
type Factory struct {
	defaultEmulator string
	profile         string
}

// NewFactory creates a new terminal factory
func NewFactory(emulator, profile string) *Factory {
	return &Factory{
		defaultEmulator: emulator,
		profile:         profile,
	}
}

// Get returns the appropriate terminal for the current platform
func (f *Factory) Get() (Terminal, error) {
	emulator := f.defaultEmulator

	// Auto-detect if not specified
	if emulator == "" {
		emulator = detectTerminal()
	}

	switch emulator {
	case "ghostty":
		return &GhosttyTerminal{profile: f.profile}, nil
	case "iterm", "iterm2":
		return &ITermTerminal{profile: f.profile}, nil
	case "terminal", "terminal.app":
		return &MacOSTerminal{profile: f.profile}, nil
	case "gnome-terminal":
		return &GnomeTerminal{profile: f.profile}, nil
	case "konsole":
		return &KonsoleTerminal{profile: f.profile}, nil
	case "kitty":
		return &KittyTerminal{}, nil
	case "x-terminal-emulator":
		return &XTerminalEmulator{}, nil
	default:
		// Try to use the specified emulator as a command
		return &GenericTerminal{command: emulator}, nil
	}
}

// detectTerminal auto-detects the terminal emulator
func detectTerminal() string {
	// Check TERM_PROGRAM environment variable
	if termProgram := os.Getenv("TERM_PROGRAM"); termProgram != "" {
		switch termProgram {
		case "Ghostty":
			return "ghostty"
		case "iTerm.app":
			return "iterm"
		case "Apple_Terminal":
			return "terminal"
		case "kitty":
			return "kitty"
		}
	}

	// Platform defaults
	switch runtime.GOOS {
	case "darwin":
		// Check if Ghostty is installed
		if _, err := os.Stat("/Applications/Ghostty.app"); err == nil {
			return "ghostty"
		}
		// Check if iTerm is installed
		if _, err := os.Stat("/Applications/iTerm.app"); err == nil {
			return "iterm"
		}
		return "terminal"
	case "linux":
		// Check for common terminals
		terminals := []string{"gnome-terminal", "konsole", "kitty", "x-terminal-emulator"}
		for _, t := range terminals {
			if commandExists(t) {
				return t
			}
		}
		return "x-terminal-emulator"
	default:
		return ""
	}
}

// commandExists checks if a command exists in PATH
func commandExists(cmd string) bool {
	_, err := os.Stat("/usr/bin/" + cmd)
	if err == nil {
		return true
	}
	_, err = os.Stat("/usr/local/bin/" + cmd)
	return err == nil
}

// Open is a convenience function to open a terminal window
func Open(emulator, profile string, opts OpenOptions) error {
	factory := NewFactory(emulator, profile)
	term, err := factory.Get()
	if err != nil {
		return err
	}

	if opts.Title == "" {
		opts.Title = "worktree-manager"
	}

	return term.OpenWindow(opts)
}

// OpenInDirectory opens a terminal in the specified directory
func OpenInDirectory(emulator, profile, dir string) error {
	return Open(emulator, profile, OpenOptions{
		WorkingDir: dir,
		Title:      fmt.Sprintf("wtm: %s", dir),
	})
}

// OpenWithCommand opens a terminal and runs a command
func OpenWithCommand(emulator, profile, dir, command string) error {
	return Open(emulator, profile, OpenOptions{
		WorkingDir: dir,
		Command:    command,
		Title:      fmt.Sprintf("wtm: %s", command),
	})
}

package terminal

import (
	"os/exec"
)

// GnomeTerminal implements Terminal for GNOME Terminal
type GnomeTerminal struct {
	profile string
}

func (t *GnomeTerminal) Name() string {
	return "GNOME Terminal"
}

func (t *GnomeTerminal) OpenWindow(opts OpenOptions) error {
	args := []string{}

	if opts.Title != "" {
		args = append(args, "--title="+opts.Title)
	}

	if opts.WorkingDir != "" {
		args = append(args, "--working-directory="+opts.WorkingDir)
	}

	if t.profile != "" {
		args = append(args, "--profile="+t.profile)
	}

	if opts.Command != "" {
		args = append(args, "--", "bash", "-c", opts.Command+"; exec bash")
	}

	return exec.Command("gnome-terminal", args...).Start()
}

func (t *GnomeTerminal) OpenTab(opts OpenOptions) error {
	args := []string{"--tab"}

	if opts.Title != "" {
		args = append(args, "--title="+opts.Title)
	}

	if opts.WorkingDir != "" {
		args = append(args, "--working-directory="+opts.WorkingDir)
	}

	if t.profile != "" {
		args = append(args, "--profile="+t.profile)
	}

	if opts.Command != "" {
		args = append(args, "--", "bash", "-c", opts.Command+"; exec bash")
	}

	return exec.Command("gnome-terminal", args...).Start()
}

// KonsoleTerminal implements Terminal for KDE Konsole
type KonsoleTerminal struct {
	profile string
}

func (t *KonsoleTerminal) Name() string {
	return "Konsole"
}

func (t *KonsoleTerminal) OpenWindow(opts OpenOptions) error {
	args := []string{}

	if opts.WorkingDir != "" {
		args = append(args, "--workdir", opts.WorkingDir)
	}

	if t.profile != "" {
		args = append(args, "--profile", t.profile)
	}

	if opts.Command != "" {
		args = append(args, "-e", "bash", "-c", opts.Command+"; exec bash")
	}

	return exec.Command("konsole", args...).Start()
}

func (t *KonsoleTerminal) OpenTab(opts OpenOptions) error {
	args := []string{"--new-tab"}

	if opts.WorkingDir != "" {
		args = append(args, "--workdir", opts.WorkingDir)
	}

	if t.profile != "" {
		args = append(args, "--profile", t.profile)
	}

	if opts.Command != "" {
		args = append(args, "-e", "bash", "-c", opts.Command+"; exec bash")
	}

	return exec.Command("konsole", args...).Start()
}

// KittyTerminal implements Terminal for Kitty
type KittyTerminal struct{}

func (t *KittyTerminal) Name() string {
	return "Kitty"
}

func (t *KittyTerminal) OpenWindow(opts OpenOptions) error {
	args := []string{}

	if opts.Title != "" {
		args = append(args, "--title", opts.Title)
	}

	if opts.WorkingDir != "" {
		args = append(args, "--directory", opts.WorkingDir)
	}

	if opts.Command != "" {
		args = append(args, "bash", "-c", opts.Command+"; exec bash")
	}

	return exec.Command("kitty", args...).Start()
}

func (t *KittyTerminal) OpenTab(opts OpenOptions) error {
	// Kitty uses kitten for tab management
	args := []string{"@", "launch", "--type=tab"}

	if opts.Title != "" {
		args = append(args, "--tab-title", opts.Title)
	}

	if opts.WorkingDir != "" {
		args = append(args, "--cwd", opts.WorkingDir)
	}

	if opts.Command != "" {
		args = append(args, "bash", "-c", opts.Command+"; exec bash")
	}

	return exec.Command("kitty", args...).Start()
}

// XTerminalEmulator implements Terminal for x-terminal-emulator (Debian/Ubuntu default)
type XTerminalEmulator struct{}

func (t *XTerminalEmulator) Name() string {
	return "x-terminal-emulator"
}

func (t *XTerminalEmulator) OpenWindow(opts OpenOptions) error {
	args := []string{}

	if opts.Command != "" {
		args = append(args, "-e", opts.Command)
	}

	cmd := exec.Command("x-terminal-emulator", args...)
	if opts.WorkingDir != "" {
		cmd.Dir = opts.WorkingDir
	}

	return cmd.Start()
}

func (t *XTerminalEmulator) OpenTab(opts OpenOptions) error {
	// x-terminal-emulator doesn't have a standard way to open tabs
	return t.OpenWindow(opts)
}

// GenericTerminal implements Terminal for any command-based terminal
type GenericTerminal struct {
	command string
}

func (t *GenericTerminal) Name() string {
	return t.command
}

func (t *GenericTerminal) OpenWindow(opts OpenOptions) error {
	args := []string{}

	if opts.Command != "" {
		args = append(args, "-e", opts.Command)
	}

	cmd := exec.Command(t.command, args...)
	if opts.WorkingDir != "" {
		cmd.Dir = opts.WorkingDir
	}

	return cmd.Start()
}

func (t *GenericTerminal) OpenTab(opts OpenOptions) error {
	return t.OpenWindow(opts)
}

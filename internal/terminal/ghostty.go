package terminal

import (
	"fmt"
	"os/exec"
	"strings"
)

// GhosttyTerminal implements Terminal for Ghostty
type GhosttyTerminal struct {
	profile string
}

func (t *GhosttyTerminal) Name() string {
	return "Ghostty"
}

func (t *GhosttyTerminal) OpenWindow(opts OpenOptions) error {
	// Ghostty can be opened via command line
	args := []string{}

	if opts.WorkingDir != "" {
		args = append(args, "--working-directory="+opts.WorkingDir)
	}

	if opts.Command != "" {
		args = append(args, "-e", opts.Command)
	}

	// Try to use ghostty command directly
	cmd := exec.Command("ghostty", args...)
	if err := cmd.Start(); err != nil {
		// Fall back to AppleScript
		return t.openViaAppleScript(opts)
	}
	return nil
}

func (t *GhosttyTerminal) OpenTab(opts OpenOptions) error {
	// Ghostty tabs via AppleScript
	return t.openViaAppleScript(opts)
}

func (t *GhosttyTerminal) openViaAppleScript(opts OpenOptions) error {
	script := fmt.Sprintf(`
		tell application "Ghostty"
			activate
			delay 0.5
			tell application "System Events"
				tell process "Ghostty"
					keystroke "n" using command down
				end tell
			end tell
		end tell
	`)

	if opts.WorkingDir != "" {
		// Quote the path for shell safety
		quotedPath := "'" + strings.ReplaceAll(opts.WorkingDir, "'", "'\\''") + "'"
		script += fmt.Sprintf(`
			delay 0.3
			tell application "System Events"
				tell process "Ghostty"
					keystroke "cd %s && clear"
					keystroke return
				end tell
			end tell
		`, quotedPath)
	}

	if opts.Command != "" {
		script += fmt.Sprintf(`
			delay 0.3
			tell application "System Events"
				tell process "Ghostty"
					keystroke "%s"
					keystroke return
				end tell
			end tell
		`, opts.Command)
	}

	return exec.Command("osascript", "-e", script).Run()
}

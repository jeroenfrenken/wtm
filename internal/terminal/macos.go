package terminal

import (
	"fmt"
	"os/exec"
	"strings"
)

// MacOSTerminal implements Terminal for macOS Terminal.app
type MacOSTerminal struct {
	profile string
}

func (t *MacOSTerminal) Name() string {
	return "Terminal.app"
}

func (t *MacOSTerminal) OpenWindow(opts OpenOptions) error {
	return t.runAppleScript(opts, false)
}

func (t *MacOSTerminal) OpenTab(opts OpenOptions) error {
	return t.runAppleScript(opts, true)
}

func (t *MacOSTerminal) runAppleScript(opts OpenOptions, inTab bool) error {
	var scriptParts []string

	scriptParts = append(scriptParts, `tell application "Terminal"`)
	scriptParts = append(scriptParts, `  activate`)

	// Build the command to run
	var cmds []string
	if opts.WorkingDir != "" {
		cmds = append(cmds, fmt.Sprintf("cd %s", escapeShell(opts.WorkingDir)))
	}
	if opts.Command != "" {
		cmds = append(cmds, opts.Command)
	}

	command := strings.Join(cmds, " && ")
	if command == "" {
		command = "clear"
	}

	if inTab {
		scriptParts = append(scriptParts, `  tell application "System Events"`)
		scriptParts = append(scriptParts, `    tell process "Terminal"`)
		scriptParts = append(scriptParts, `      keystroke "t" using command down`)
		scriptParts = append(scriptParts, `    end tell`)
		scriptParts = append(scriptParts, `  end tell`)
		scriptParts = append(scriptParts, `  delay 0.5`)
		scriptParts = append(scriptParts, fmt.Sprintf(`  do script "%s" in front window`, escapeAppleScript(command)))
	} else {
		scriptParts = append(scriptParts, fmt.Sprintf(`  do script "%s"`, escapeAppleScript(command)))
	}

	// Set window title
	if opts.Title != "" {
		scriptParts = append(scriptParts, `  tell front window`)
		scriptParts = append(scriptParts, fmt.Sprintf(`    set custom title to "%s"`, escapeAppleScript(opts.Title)))
		scriptParts = append(scriptParts, `  end tell`)
	}

	scriptParts = append(scriptParts, `end tell`)

	script := strings.Join(scriptParts, "\n")
	return exec.Command("osascript", "-e", script).Run()
}

func escapeShell(s string) string {
	// Simple escaping for shell
	if strings.ContainsAny(s, " \t\n\"'$`\\") {
		return "'" + strings.ReplaceAll(s, "'", "'\"'\"'") + "'"
	}
	return s
}

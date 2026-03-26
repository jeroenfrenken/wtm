package terminal

import (
	"fmt"
	"os/exec"
	"strings"
)

// ITermTerminal implements Terminal for iTerm2
type ITermTerminal struct {
	profile string
}

func (t *ITermTerminal) Name() string {
	return "iTerm2"
}

func (t *ITermTerminal) OpenWindow(opts OpenOptions) error {
	return t.runAppleScript(opts, false)
}

func (t *ITermTerminal) OpenTab(opts OpenOptions) error {
	return t.runAppleScript(opts, true)
}

func (t *ITermTerminal) runAppleScript(opts OpenOptions, inTab bool) error {
	var scriptParts []string

	scriptParts = append(scriptParts, `tell application "iTerm2"`)
	scriptParts = append(scriptParts, `  activate`)

	if inTab {
		scriptParts = append(scriptParts, `  tell current window`)
		scriptParts = append(scriptParts, `    create tab with default profile`)
		scriptParts = append(scriptParts, `    tell current session`)
	} else {
		if t.profile != "" {
			scriptParts = append(scriptParts, fmt.Sprintf(`  set newWindow to (create window with profile "%s")`, t.profile))
		} else {
			scriptParts = append(scriptParts, `  set newWindow to (create window with default profile)`)
		}
		scriptParts = append(scriptParts, `  tell current session of newWindow`)
	}

	// Change directory (quote the path for shell safety)
	if opts.WorkingDir != "" {
		// Use single quotes and escape any single quotes in the path
		quotedPath := "'" + strings.ReplaceAll(opts.WorkingDir, "'", "'\\''") + "'"
		scriptParts = append(scriptParts, fmt.Sprintf(`      write text "cd %s && clear"`, escapeAppleScript(quotedPath)))
	}

	// Run command
	if opts.Command != "" {
		scriptParts = append(scriptParts, fmt.Sprintf(`      write text "%s"`, escapeAppleScript(opts.Command)))
	}

	// Set title
	if opts.Title != "" {
		scriptParts = append(scriptParts, fmt.Sprintf(`      set name to "%s"`, escapeAppleScript(opts.Title)))
	}

	scriptParts = append(scriptParts, `    end tell`)
	if inTab {
		scriptParts = append(scriptParts, `  end tell`)
	}
	scriptParts = append(scriptParts, `end tell`)

	script := strings.Join(scriptParts, "\n")
	return exec.Command("osascript", "-e", script).Run()
}

func escapeAppleScript(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, `"`, `\"`)
	return s
}

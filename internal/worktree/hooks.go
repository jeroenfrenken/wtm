package worktree

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/jeroenfrenken/worktree-manager/internal/config"
)

// HookRunner executes project-level hooks
type HookRunner struct {
	cfg     *config.Config
	envVars map[string]string
}

// NewHookRunner creates a new hook runner
func NewHookRunner(cfg *config.Config) *HookRunner {
	return &HookRunner{
		cfg:     cfg,
		envVars: make(map[string]string),
	}
}

// WithEnv adds environment variables for hook execution
func (h *HookRunner) WithEnv(env map[string]string) *HookRunner {
	for k, v := range env {
		h.envVars[k] = v
	}
	return h
}

// RunOnCreate runs the on_create hooks
func (h *HookRunner) RunOnCreate(ctx context.Context, wt *Worktree) error {
	if len(h.cfg.Hooks.OnCreate) == 0 {
		return nil
	}

	fmt.Printf("Running on_create hooks for %s...\n", wt.Name)

	for _, cmd := range h.cfg.Hooks.OnCreate {
		if err := h.runCommand(ctx, cmd, wt.Path); err != nil {
			return fmt.Errorf("on_create hook %q failed: %w", cmd, err)
		}
	}

	return nil
}

// RunOnDelete runs the on_delete hooks
func (h *HookRunner) RunOnDelete(ctx context.Context, wt *Worktree) error {
	if len(h.cfg.Hooks.OnDelete) == 0 {
		return nil
	}

	fmt.Printf("Running on_delete hooks for %s...\n", wt.Name)

	for _, cmd := range h.cfg.Hooks.OnDelete {
		if err := h.runCommand(ctx, cmd, wt.Path); err != nil {
			return fmt.Errorf("on_delete hook %q failed: %w", cmd, err)
		}
	}

	return nil
}

// runCommand executes a single command
func (h *HookRunner) runCommand(ctx context.Context, command, workDir string) error {
	// Substitute variables in command
	command = h.substituteVars(command)

	fmt.Printf("  → %s\n", command)

	cmd := exec.CommandContext(ctx, "sh", "-c", command)
	cmd.Dir = workDir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Env = h.buildEnv()

	return cmd.Run()
}

// substituteVars replaces variables in a string
func (h *HookRunner) substituteVars(s string) string {
	for k, v := range h.envVars {
		s = strings.ReplaceAll(s, "$"+k, v)
		s = strings.ReplaceAll(s, "${"+k+"}", v)
	}
	return s
}

// buildEnv builds the environment for hook execution
func (h *HookRunner) buildEnv() []string {
	env := os.Environ()
	for k, v := range h.envVars {
		env = append(env, k+"="+v)
	}
	return env
}

// RunInTerminal runs hooks in a terminal window (for interactive hooks)
type TerminalHookRunner struct {
	*HookRunner
	terminalCmd string
}

// NewTerminalHookRunner creates a hook runner that opens a terminal
func NewTerminalHookRunner(cfg *config.Config, terminalCmd string) *TerminalHookRunner {
	return &TerminalHookRunner{
		HookRunner:  NewHookRunner(cfg),
		terminalCmd: terminalCmd,
	}
}

// RunOnCreateInTerminal runs on_create hooks in a terminal window
func (t *TerminalHookRunner) RunOnCreateInTerminal(ctx context.Context, wt *Worktree) error {
	if len(t.cfg.Hooks.OnCreate) == 0 {
		return nil
	}

	// Build the full command to run in terminal
	commands := make([]string, len(t.cfg.Hooks.OnCreate))
	for i, cmd := range t.cfg.Hooks.OnCreate {
		commands[i] = t.substituteVars(cmd)
	}

	script := strings.Join(commands, " && ")

	// For now, run directly (terminal integration will be added in Phase 7)
	return t.runCommand(ctx, script, wt.Path)
}

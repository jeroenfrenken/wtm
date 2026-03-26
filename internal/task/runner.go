package task

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/jeroenfrenken/worktree-manager/internal/config"
)

// Runner executes one-off tasks
type Runner struct {
	cfg      *config.Config
	workDir  string
	worktree string
	basePort int
}

// NewRunner creates a new task runner
func NewRunner(cfg *config.Config, workDir, worktree string, basePort int) *Runner {
	return &Runner{
		cfg:      cfg,
		workDir:  workDir,
		worktree: worktree,
		basePort: basePort,
	}
}

// Run executes a task by name
func (r *Runner) Run(ctx context.Context, name string) error {
	task, ok := r.cfg.Tasks[name]
	if !ok {
		return fmt.Errorf("task %q not found", name)
	}

	// Determine working directory
	workDir := r.workDir
	if task.WorkingDir != "" {
		if task.WorkingDir[0] == '/' {
			workDir = task.WorkingDir
		} else {
			workDir = r.workDir + "/" + task.WorkingDir
		}
	}

	// Build environment
	env := r.buildEnv()

	// Substitute variables in command
	command := r.substituteVars(task.Run)

	fmt.Printf("Running task: %s\n", name)
	if task.Description != "" {
		fmt.Printf("  %s\n", task.Description)
	}
	fmt.Printf("  → %s\n", command)
	fmt.Println()

	cmd := exec.CommandContext(ctx, "sh", "-c", command)
	cmd.Dir = workDir
	cmd.Env = env
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin

	return cmd.Run()
}

// List returns all available tasks
func (r *Runner) List() map[string]config.Task {
	return r.cfg.Tasks
}

// buildEnv builds the environment for task execution
func (r *Runner) buildEnv() []string {
	// Start with current environment
	env := os.Environ()

	// Load env files
	baseEnv, _ := config.LoadEnvFiles(r.workDir, r.cfg.EnvFiles)
	wtEnv, _ := config.LoadWorktreeEnv(r.workDir, r.worktree)

	// Merge
	mergedEnv := config.MergeEnv(baseEnv, wtEnv)

	// Add built-in variables
	envCtx := config.EnvContext{
		Port:      r.basePort,
		Worktree:  r.worktree,
		Project:   r.cfg.Project,
		Domain:    r.cfg.Domain,
		ProxyPort: r.cfg.ProxyPort,
	}

	// Substitute variables
	finalEnv := config.SubstituteEnvMap(mergedEnv, envCtx)

	// Convert and append
	for k, v := range finalEnv {
		env = append(env, k+"="+v)
	}

	// Add built-in variables explicitly
	env = append(env, fmt.Sprintf("WORKTREE=%s", r.worktree))
	env = append(env, fmt.Sprintf("PROJECT=%s", r.cfg.Project))
	env = append(env, fmt.Sprintf("DOMAIN=%s", r.cfg.Domain))
	env = append(env, fmt.Sprintf("PROXY_PORT=%d", r.cfg.ProxyPort))
	env = append(env, fmt.Sprintf("PORT=%d", r.basePort))

	return env
}

// substituteVars replaces variables in a string
func (r *Runner) substituteVars(s string) string {
	s = strings.ReplaceAll(s, "$PORT", fmt.Sprintf("%d", r.basePort))
	s = strings.ReplaceAll(s, "${PORT}", fmt.Sprintf("%d", r.basePort))
	s = strings.ReplaceAll(s, "$WORKTREE", r.worktree)
	s = strings.ReplaceAll(s, "${WORKTREE}", r.worktree)
	s = strings.ReplaceAll(s, "$PROJECT", r.cfg.Project)
	s = strings.ReplaceAll(s, "${PROJECT}", r.cfg.Project)
	s = strings.ReplaceAll(s, "$DOMAIN", r.cfg.Domain)
	s = strings.ReplaceAll(s, "${DOMAIN}", r.cfg.Domain)
	s = strings.ReplaceAll(s, "$PROXY_PORT", fmt.Sprintf("%d", r.cfg.ProxyPort))
	s = strings.ReplaceAll(s, "${PROXY_PORT}", fmt.Sprintf("%d", r.cfg.ProxyPort))
	return s
}

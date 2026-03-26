package process

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/jeroenfrenken/worktree-manager/internal/config"
)

// LifecycleHooks manages service lifecycle hook execution
type LifecycleHooks struct {
	cfg        *config.Config
	supervisor *Supervisor
}

// NewLifecycleHooks creates a new lifecycle hooks manager
func NewLifecycleHooks(cfg *config.Config, supervisor *Supervisor) *LifecycleHooks {
	return &LifecycleHooks{
		cfg:        cfg,
		supervisor: supervisor,
	}
}

// RunOnStart runs the on_start hooks for a service
func (h *LifecycleHooks) RunOnStart(ctx context.Context, serviceName string) error {
	svc, ok := h.supervisor.GetService(serviceName)
	if !ok {
		return fmt.Errorf("service not found: %s", serviceName)
	}

	if len(svc.Config.Hooks.OnStart) == 0 {
		return nil
	}

	fmt.Printf("[%s] Running on_start hooks...\n", serviceName)

	for _, cmd := range svc.Config.Hooks.OnStart {
		if err := h.runCommand(ctx, serviceName, cmd); err != nil {
			return fmt.Errorf("on_start hook failed: %w", err)
		}
	}

	return nil
}

// RunOnStop runs the on_stop hooks for a service
func (h *LifecycleHooks) RunOnStop(ctx context.Context, serviceName string) error {
	svc, ok := h.supervisor.GetService(serviceName)
	if !ok {
		return fmt.Errorf("service not found: %s", serviceName)
	}

	if len(svc.Config.Hooks.OnStop) == 0 {
		return nil
	}

	fmt.Printf("[%s] Running on_stop hooks...\n", serviceName)

	for _, cmd := range svc.Config.Hooks.OnStop {
		if err := h.runCommand(ctx, serviceName, cmd); err != nil {
			// Log but don't fail on stop hooks
			fmt.Printf("[%s] on_stop hook failed: %v\n", serviceName, err)
		}
	}

	return nil
}

// RunOnRestart runs the on_restart hooks for a service
func (h *LifecycleHooks) RunOnRestart(ctx context.Context, serviceName string) error {
	svc, ok := h.supervisor.GetService(serviceName)
	if !ok {
		return fmt.Errorf("service not found: %s", serviceName)
	}

	if len(svc.Config.Hooks.OnRestart) == 0 {
		return nil
	}

	fmt.Printf("[%s] Running on_restart hooks...\n", serviceName)

	for _, cmd := range svc.Config.Hooks.OnRestart {
		if err := h.runCommand(ctx, serviceName, cmd); err != nil {
			return fmt.Errorf("on_restart hook failed: %w", err)
		}
	}

	return nil
}

// RunOnCrash runs the on_crash hooks for a service
func (h *LifecycleHooks) RunOnCrash(ctx context.Context, serviceName string) error {
	svc, ok := h.supervisor.GetService(serviceName)
	if !ok {
		return fmt.Errorf("service not found: %s", serviceName)
	}

	crashCfg := svc.Config.Hooks.OnCrash
	if crashCfg == nil || len(crashCfg.Commands) == 0 {
		return nil
	}

	fmt.Printf("[%s] Running on_crash hooks...\n", serviceName)

	for _, cmd := range crashCfg.Commands {
		if err := h.runCommand(ctx, serviceName, cmd); err != nil {
			// Log but don't fail on crash hooks
			fmt.Printf("[%s] on_crash hook failed: %v\n", serviceName, err)
		}
	}

	return nil
}

// runCommand executes a single hook command
func (h *LifecycleHooks) runCommand(ctx context.Context, serviceName, command string) error {
	svc, ok := h.supervisor.GetService(serviceName)
	if !ok {
		return fmt.Errorf("service not found: %s", serviceName)
	}

	// Determine working directory
	workDir := h.supervisor.workDir
	if svc.Config.WorkingDir != "" {
		workDir = svc.Config.WorkingDir
		if workDir[0] != '/' {
			workDir = h.supervisor.workDir + "/" + workDir
		}
	}

	// Substitute variables
	envCtx := config.EnvContext{
		Port:      svc.Port,
		Worktree:  h.supervisor.worktree,
		Project:   h.cfg.Project,
		Domain:    h.cfg.Domain,
		ProxyPort: h.cfg.ProxyPort,
	}

	command = h.substituteVars(command, envCtx)

	fmt.Printf("[%s]   → %s\n", serviceName, command)

	cmd := exec.CommandContext(ctx, "sh", "-c", command)
	cmd.Dir = workDir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	// Build environment
	env := h.supervisor.buildEnv(serviceName)
	cmd.Env = append(os.Environ(), env...)

	return cmd.Run()
}

// substituteVars replaces variables in a string
func (h *LifecycleHooks) substituteVars(s string, ctx config.EnvContext) string {
	s = strings.ReplaceAll(s, "$PORT", fmt.Sprintf("%d", ctx.Port))
	s = strings.ReplaceAll(s, "${PORT}", fmt.Sprintf("%d", ctx.Port))
	s = strings.ReplaceAll(s, "$WORKTREE", ctx.Worktree)
	s = strings.ReplaceAll(s, "${WORKTREE}", ctx.Worktree)
	s = strings.ReplaceAll(s, "$PROJECT", ctx.Project)
	s = strings.ReplaceAll(s, "${PROJECT}", ctx.Project)
	s = strings.ReplaceAll(s, "$DOMAIN", ctx.Domain)
	s = strings.ReplaceAll(s, "${DOMAIN}", ctx.Domain)
	s = strings.ReplaceAll(s, "$PROXY_PORT", fmt.Sprintf("%d", ctx.ProxyPort))
	s = strings.ReplaceAll(s, "${PROXY_PORT}", fmt.Sprintf("%d", ctx.ProxyPort))
	return s
}

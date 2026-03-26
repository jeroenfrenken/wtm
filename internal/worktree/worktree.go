package worktree

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// Worktree represents a git worktree
type Worktree struct {
	Name     string `json:"name"`
	Path     string `json:"path"`
	Branch   string `json:"branch"`
	BasePort int    `json:"base_port"`
	Index    int    `json:"index"`
}

// Manager handles git worktree operations
type Manager struct {
	projectDir string
	registry   *Registry
}

// NewManager creates a new worktree manager
func NewManager(projectDir string) (*Manager, error) {
	registry, err := LoadRegistry(projectDir)
	if err != nil {
		return nil, err
	}

	return &Manager{
		projectDir: projectDir,
		registry:   registry,
	}, nil
}

// Create creates a new git worktree
func (m *Manager) Create(ctx context.Context, name, branch string) (*Worktree, error) {
	// Check if worktree already exists
	if _, exists := m.registry.Get(name); exists {
		return nil, fmt.Errorf("worktree %q already exists", name)
	}

	// Determine worktree path (sibling to project directory)
	worktreesDir := filepath.Join(filepath.Dir(m.projectDir), ".worktrees")
	if err := os.MkdirAll(worktreesDir, 0755); err != nil {
		return nil, fmt.Errorf("creating worktrees directory: %w", err)
	}

	worktreePath := filepath.Join(worktreesDir, name)

	// If no branch specified, use the worktree name
	if branch == "" {
		branch = name
	}

	// Check if branch exists
	branchExists, err := m.branchExists(ctx, branch)
	if err != nil {
		return nil, err
	}

	// Create the git worktree
	var cmd *exec.Cmd
	if branchExists {
		cmd = exec.CommandContext(ctx, "git", "worktree", "add", worktreePath, branch)
	} else {
		// Create new branch
		cmd = exec.CommandContext(ctx, "git", "worktree", "add", "-b", branch, worktreePath)
	}
	cmd.Dir = m.projectDir

	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("creating worktree: %s: %w", strings.TrimSpace(string(output)), err)
	}

	// Register the worktree
	wt := &Worktree{
		Name:     name,
		Path:     worktreePath,
		Branch:   branch,
		BasePort: m.registry.NextBasePort(),
		Index:    m.registry.NextIndex(),
	}

	if err := m.registry.Add(wt); err != nil {
		// Try to clean up the worktree if registration fails
		_ = exec.CommandContext(ctx, "git", "worktree", "remove", "--force", worktreePath).Run()
		return nil, fmt.Errorf("registering worktree: %w", err)
	}

	return wt, nil
}

// Delete removes a git worktree
func (m *Manager) Delete(ctx context.Context, name string, force bool) error {
	wt, exists := m.registry.Get(name)
	if !exists {
		return fmt.Errorf("worktree %q not found", name)
	}

	// Try to remove the git worktree
	args := []string{"worktree", "remove"}
	if force {
		args = append(args, "--force")
	}
	args = append(args, wt.Path)

	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = m.projectDir

	output, err := cmd.CombinedOutput()
	if err != nil {
		if force {
			// Git failed even with --force (e.g., untracked files)
			// Manually remove the directory and clean up git
			if removeErr := os.RemoveAll(wt.Path); removeErr != nil {
				return fmt.Errorf("removing worktree directory: %w", removeErr)
			}
			// Prune the worktree reference from git
			pruneCmd := exec.CommandContext(ctx, "git", "worktree", "prune")
			pruneCmd.Dir = m.projectDir
			_ = pruneCmd.Run()
		} else {
			return fmt.Errorf("removing worktree: %s: %w", strings.TrimSpace(string(output)), err)
		}
	}

	// Unregister the worktree
	if err := m.registry.Remove(name); err != nil {
		return fmt.Errorf("unregistering worktree: %w", err)
	}

	return nil
}

// List returns all registered worktrees
func (m *Manager) List() []*Worktree {
	return m.registry.List()
}

// Get returns a specific worktree
func (m *Manager) Get(name string) (*Worktree, bool) {
	return m.registry.Get(name)
}

// Current returns the worktree for the current directory
func (m *Manager) Current() (*Worktree, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return nil, err
	}

	for _, wt := range m.registry.List() {
		if strings.HasPrefix(cwd, wt.Path) {
			return wt, nil
		}
	}

	// Check if we're in the main project directory
	if strings.HasPrefix(cwd, m.projectDir) {
		// Return a virtual "main" worktree
		return &Worktree{
			Name:     "main",
			Path:     m.projectDir,
			Branch:   m.currentBranch(),
			BasePort: m.registry.state.PortRangeStart,
			Index:    0,
		}, nil
	}

	return nil, fmt.Errorf("not in a worktree")
}

// branchExists checks if a git branch exists
func (m *Manager) branchExists(ctx context.Context, branch string) (bool, error) {
	cmd := exec.CommandContext(ctx, "git", "show-ref", "--verify", "--quiet", "refs/heads/"+branch)
	cmd.Dir = m.projectDir

	err := cmd.Run()
	if err != nil {
		if _, ok := err.(*exec.ExitError); ok {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

// currentBranch returns the current git branch
func (m *Manager) currentBranch() string {
	cmd := exec.Command("git", "rev-parse", "--abbrev-ref", "HEAD")
	cmd.Dir = m.projectDir
	output, err := cmd.Output()
	if err != nil {
		return "unknown"
	}
	return strings.TrimSpace(string(output))
}

// Prune cleans up stale worktree entries
func (m *Manager) Prune(ctx context.Context) error {
	cmd := exec.CommandContext(ctx, "git", "worktree", "prune")
	cmd.Dir = m.projectDir
	return cmd.Run()
}

// CalculatePort calculates the port for a service in this worktree
func (w *Worktree) CalculatePort(portOffset int) int {
	return w.BasePort + portOffset
}

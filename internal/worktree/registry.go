package worktree

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/jeroenfrenken/worktree-manager/internal/config"
)

// State represents the persistent state of worktrees
type State struct {
	Worktrees      map[string]*Worktree `json:"worktrees"`
	NextIndex      int                  `json:"next_index"`
	PortRangeStart int                  `json:"port_range_start"`
}

// Registry manages worktree state persistence
type Registry struct {
	mu         sync.RWMutex
	state      *State
	stateFile  string
	projectDir string
}

// LoadRegistry loads or creates a new registry
func LoadRegistry(projectDir string) (*Registry, error) {
	stateDir := filepath.Join(projectDir, config.StateDir)
	if err := os.MkdirAll(stateDir, 0755); err != nil {
		return nil, fmt.Errorf("creating state directory: %w", err)
	}

	stateFile := filepath.Join(stateDir, "state.json")
	registry := &Registry{
		stateFile:  stateFile,
		projectDir: projectDir,
	}

	// Try to load existing state
	data, err := os.ReadFile(stateFile)
	if err != nil {
		if os.IsNotExist(err) {
			// Initialize with default state
			registry.state = &State{
				Worktrees:      make(map[string]*Worktree),
				NextIndex:      1, // 0 is reserved for main
				PortRangeStart: 3001,
			}

			// Try to get port_range_start from config
			cfg, err := config.Load(projectDir)
			if err == nil && cfg.PortRangeStart > 0 {
				registry.state.PortRangeStart = cfg.PortRangeStart
			}

			return registry, nil
		}
		return nil, fmt.Errorf("reading state file: %w", err)
	}

	var state State
	if err := json.Unmarshal(data, &state); err != nil {
		return nil, fmt.Errorf("parsing state file: %w", err)
	}

	if state.Worktrees == nil {
		state.Worktrees = make(map[string]*Worktree)
	}

	registry.state = &state
	return registry, nil
}

// Save persists the current state to disk
func (r *Registry) Save() error {
	r.mu.RLock()
	defer r.mu.RUnlock()

	data, err := json.MarshalIndent(r.state, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling state: %w", err)
	}

	if err := os.WriteFile(r.stateFile, data, 0644); err != nil {
		return fmt.Errorf("writing state file: %w", err)
	}

	return nil
}

// Add registers a new worktree
func (r *Registry) Add(wt *Worktree) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.state.Worktrees[wt.Name]; exists {
		return fmt.Errorf("worktree %q already exists", wt.Name)
	}

	r.state.Worktrees[wt.Name] = wt
	r.state.NextIndex++

	return r.saveUnlocked()
}

// Remove unregisters a worktree
func (r *Registry) Remove(name string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.state.Worktrees[name]; !exists {
		return fmt.Errorf("worktree %q not found", name)
	}

	delete(r.state.Worktrees, name)

	return r.saveUnlocked()
}

// Get returns a worktree by name
func (r *Registry) Get(name string) (*Worktree, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	wt, ok := r.state.Worktrees[name]
	return wt, ok
}

// List returns all registered worktrees
func (r *Registry) List() []*Worktree {
	r.mu.RLock()
	defer r.mu.RUnlock()

	worktrees := make([]*Worktree, 0, len(r.state.Worktrees))
	for _, wt := range r.state.Worktrees {
		worktrees = append(worktrees, wt)
	}
	return worktrees
}

// NextBasePort returns the base port for the next worktree
func (r *Registry) NextBasePort() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.state.PortRangeStart + (r.state.NextIndex * 100)
}

// NextIndex returns the next worktree index
func (r *Registry) NextIndex() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.state.NextIndex
}

// saveUnlocked saves state without acquiring lock (must be called with lock held)
func (r *Registry) saveUnlocked() error {
	data, err := json.MarshalIndent(r.state, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling state: %w", err)
	}

	if err := os.WriteFile(r.stateFile, data, 0644); err != nil {
		return fmt.Errorf("writing state file: %w", err)
	}

	return nil
}

// Update updates a worktree's properties
func (r *Registry) Update(name string, updater func(*Worktree)) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	wt, exists := r.state.Worktrees[name]
	if !exists {
		return fmt.Errorf("worktree %q not found", name)
	}

	updater(wt)

	return r.saveUnlocked()
}

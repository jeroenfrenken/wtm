package process

import (
	"context"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/jeroenfrenken/worktree-manager/internal/config"
)

// Event represents a supervisor event
type Event struct {
	Type    EventType
	Service string
	Message string
	Time    time.Time
}

// EventType represents the type of event
type EventType string

const (
	EventStarted   EventType = "started"
	EventStopped   EventType = "stopped"
	EventFailed    EventType = "failed"
	EventHealthy   EventType = "healthy"
	EventUnhealthy EventType = "unhealthy"
	EventRestart   EventType = "restart"
)

// Supervisor manages multiple services
type Supervisor struct {
	cfg        *config.Config
	worktree   string
	workDir    string
	basePort   int
	services   map[string]*Service
	order      []string // Topologically sorted service names
	mu         sync.RWMutex
	events     chan Event
	ctx        context.Context
	cancel     context.CancelFunc
	wg         sync.WaitGroup
	allLogs    chan LogLine
	hooks      *LifecycleHooks
	healthMgr  *HealthManager
}

// NewSupervisor creates a new supervisor
func NewSupervisor(cfg *config.Config, worktree, workDir string, basePort int) (*Supervisor, error) {
	// Calculate service order based on dependencies
	order, err := TopologicalSort(cfg.Services)
	if err != nil {
		return nil, err
	}

	s := &Supervisor{
		cfg:      cfg,
		worktree: worktree,
		workDir:  workDir,
		basePort: basePort,
		services: make(map[string]*Service),
		order:    order,
		events:   make(chan Event, 100),
		allLogs:  make(chan LogLine, 1000),
	}

	// Create service instances
	for name, svcCfg := range cfg.Services {
		port := basePort + svcCfg.PortOffset
		if !svcCfg.NeedsPortValue() {
			port = 0
		}
		s.services[name] = NewService(name, svcCfg, port)
	}

	s.hooks = NewLifecycleHooks(cfg, s)
	s.healthMgr = NewHealthManager(s)

	return s, nil
}

// Start starts all services in dependency order
func (s *Supervisor) Start(ctx context.Context) error {
	s.mu.Lock()
	s.ctx, s.cancel = context.WithCancel(ctx)
	s.mu.Unlock()

	for _, name := range s.order {
		svc := s.services[name]

		// Wait for dependencies to be healthy
		if err := s.waitForDependencies(s.ctx, name); err != nil {
			return fmt.Errorf("waiting for dependencies of %s: %w", name, err)
		}

		// Run on_start hooks
		if err := s.hooks.RunOnStart(s.ctx, name); err != nil {
			return fmt.Errorf("on_start hook for %s failed: %w", name, err)
		}

		// Build environment
		env := s.buildEnv(name)

		// Start the service
		if err := svc.Start(s.ctx, s.workDir, env); err != nil {
			return fmt.Errorf("starting %s: %w", name, err)
		}

		s.emit(EventStarted, name, "service started")

		// Start log forwarding
		go s.forwardLogs(svc)

		// Start monitoring
		s.wg.Add(1)
		go s.monitor(svc)

		// Start health checking
		s.healthMgr.StartChecking(svc)
	}

	return nil
}

// StartService starts a single service
func (s *Supervisor) StartService(ctx context.Context, name string) error {
	s.mu.RLock()
	svc, ok := s.services[name]
	s.mu.RUnlock()

	if !ok {
		return fmt.Errorf("service %s not found", name)
	}

	if svc.IsRunning() {
		return fmt.Errorf("service %s is already running", name)
	}

	// Wait for dependencies
	if err := s.waitForDependencies(ctx, name); err != nil {
		return err
	}

	// Run on_start hooks
	if err := s.hooks.RunOnStart(ctx, name); err != nil {
		return err
	}

	// Build environment
	env := s.buildEnv(name)

	// Start the service
	if err := svc.Start(ctx, s.workDir, env); err != nil {
		return err
	}

	s.emit(EventStarted, name, "service started")

	// Start log forwarding and monitoring
	go s.forwardLogs(svc)
	s.wg.Add(1)
	go s.monitor(svc)
	s.healthMgr.StartChecking(svc)

	return nil
}

// Stop stops all services in reverse dependency order
func (s *Supervisor) Stop(ctx context.Context) error {
	s.mu.Lock()
	if s.cancel != nil {
		s.cancel()
	}
	s.mu.Unlock()

	// Stop health checking
	s.healthMgr.StopAll()

	// Stop in reverse order
	for i := len(s.order) - 1; i >= 0; i-- {
		name := s.order[i]
		svc := s.services[name]

		if err := svc.Stop(ctx); err != nil {
			fmt.Printf("Warning: error stopping %s: %v\n", name, err)
		}

		// Run on_stop hooks
		_ = s.hooks.RunOnStop(ctx, name)

		s.emit(EventStopped, name, "service stopped")
	}

	// Wait for all monitors to finish
	s.wg.Wait()

	return nil
}

// StopService stops a single service
func (s *Supervisor) StopService(ctx context.Context, name string) error {
	s.mu.RLock()
	svc, ok := s.services[name]
	s.mu.RUnlock()

	if !ok {
		return fmt.Errorf("service %s not found", name)
	}

	s.healthMgr.StopChecking(name)

	if err := svc.Stop(ctx); err != nil {
		return err
	}

	// Run on_stop hooks
	_ = s.hooks.RunOnStop(ctx, name)

	s.emit(EventStopped, name, "service stopped")

	return nil
}

// RestartService restarts a single service
func (s *Supervisor) RestartService(ctx context.Context, name string) error {
	if err := s.StopService(ctx, name); err != nil {
		return err
	}

	// Small delay between stop and start
	time.Sleep(100 * time.Millisecond)

	return s.StartService(ctx, name)
}

// Status returns the status of all services
func (s *Supervisor) Status() map[string]ServiceStatus {
	s.mu.RLock()
	defer s.mu.RUnlock()

	status := make(map[string]ServiceStatus)
	for name, svc := range s.services {
		status[name] = svc.Status()
	}
	return status
}

// Events returns the event channel
func (s *Supervisor) Events() <-chan Event {
	return s.events
}

// Logs returns the aggregated log channel
func (s *Supervisor) Logs() <-chan LogLine {
	return s.allLogs
}

// monitor watches a service and handles restarts
func (s *Supervisor) monitor(svc *Service) {
	defer s.wg.Done()

	for {
		select {
		case <-svc.Done():
			status := svc.Status()

			if status.State == StateFailed {
				s.emit(EventFailed, svc.Name, status.LastError)

				// Run on_crash hooks
				crashCfg := svc.Config.Hooks.OnCrash
				if crashCfg != nil {
					_ = s.hooks.RunOnCrash(context.Background(), svc.Name)

					// Auto restart if configured
					if crashCfg.AutoRestart && status.RestartCount < crashCfg.MaxRestarts {
						s.emit(EventRestart, svc.Name, fmt.Sprintf("restarting (attempt %d/%d)", status.RestartCount+1, crashCfg.MaxRestarts))

						if crashCfg.RestartDelay > 0 {
							time.Sleep(crashCfg.RestartDelay)
						}

						// Run on_restart hooks
						_ = s.hooks.RunOnRestart(context.Background(), svc.Name)

						svc.IncrementRestarts()
						env := s.buildEnv(svc.Name)
						if err := svc.Start(context.Background(), s.workDir, env); err != nil {
							s.emit(EventFailed, svc.Name, fmt.Sprintf("restart failed: %v", err))
							return
						}

						s.emit(EventStarted, svc.Name, "restarted")
						continue
					}
				}
			}
			return

		case <-s.ctx.Done():
			return
		}
	}
}

// forwardLogs forwards logs from a service to the aggregated channel
func (s *Supervisor) forwardLogs(svc *Service) {
	for log := range svc.Logs() {
		select {
		case s.allLogs <- log:
		default:
			// Drop if full
		}
	}
}

// waitForDependencies waits for service dependencies to be healthy
func (s *Supervisor) waitForDependencies(ctx context.Context, name string) error {
	svc := s.services[name]

	for _, depName := range svc.Config.DependsOn {
		dep, ok := s.services[depName]
		if !ok {
			return fmt.Errorf("unknown dependency: %s", depName)
		}

		// Wait for dependency to be healthy
		timeout := 60 * time.Second
		if dep.Config.HealthCheck != nil && dep.Config.HealthCheck.Timeout > 0 {
			timeout = dep.Config.HealthCheck.Timeout
		}

		timeoutCtx, cancel := context.WithTimeout(ctx, timeout)
		defer cancel()

		for {
			select {
			case <-timeoutCtx.Done():
				return fmt.Errorf("timeout waiting for %s to be healthy", depName)
			default:
				status := dep.Status()
				if status.State == StateRunning && status.Health == HealthHealthy {
					break
				}
				if status.State == StateFailed {
					return fmt.Errorf("dependency %s failed", depName)
				}
				time.Sleep(500 * time.Millisecond)
				continue
			}
			break
		}
	}

	return nil
}

// buildEnv builds the environment variables for a service
func (s *Supervisor) buildEnv(name string) []string {
	svc := s.services[name]

	// Add built-in variables
	envCtx := config.EnvContext{
		Port:      svc.Port,
		Worktree:  s.worktree,
		Project:   s.cfg.Project,
		Domain:    s.cfg.Domain,
		ProxyPort: s.cfg.ProxyPort,
	}

	// Load and merge environment files
	baseEnv, _ := config.LoadEnvFiles(s.workDir, s.cfg.EnvFiles)
	wtEnv, _ := config.LoadWorktreeEnv(s.workDir, s.worktree)
	mergedEnv := config.MergeEnv(baseEnv, wtEnv, svc.Config.Env)

	// Substitute variables in env file values
	finalEnv := config.SubstituteEnvMap(mergedEnv, envCtx)

	// Add built-in variables to the environment so they're available
	// for shell expansion in the run command (e.g., $PORT in "npm run dev --port $PORT")
	finalEnv["PORT"] = fmt.Sprintf("%d", svc.Port)
	finalEnv["WORKTREE"] = s.worktree
	finalEnv["PROJECT"] = s.cfg.Project
	finalEnv["DOMAIN"] = s.cfg.Domain
	finalEnv["PROXY_PORT"] = fmt.Sprintf("%d", s.cfg.ProxyPort)

	// Start with inherited system environment (includes PATH etc.),
	// then layer config-defined variables on top
	return append(os.Environ(), config.ToOSEnv(finalEnv)...)
}

// emit sends an event
func (s *Supervisor) emit(t EventType, service, message string) {
	event := Event{
		Type:    t,
		Service: service,
		Message: message,
		Time:    time.Now(),
	}

	select {
	case s.events <- event:
	default:
		// Buffer full
	}
}

// GetService returns a service by name
func (s *Supervisor) GetService(name string) (*Service, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	svc, ok := s.services[name]
	return svc, ok
}

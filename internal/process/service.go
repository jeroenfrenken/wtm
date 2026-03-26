package process

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os/exec"
	"regexp"
	"sync"
	"syscall"
	"time"

	"github.com/jeroenfrenken/worktree-manager/internal/config"
)

// ServiceState represents the current state of a service
type ServiceState string

const (
	StateIdle       ServiceState = "idle"
	StateStarting   ServiceState = "starting"
	StateRunning    ServiceState = "running"
	StateStopping   ServiceState = "stopping"
	StateStopped    ServiceState = "stopped"
	StateFailed     ServiceState = "failed"
	StateRestarting ServiceState = "restarting"
)

// HealthStatus represents the health of a service
type HealthStatus string

const (
	HealthUnknown   HealthStatus = "unknown"
	HealthHealthy   HealthStatus = "healthy"
	HealthUnhealthy HealthStatus = "unhealthy"
	HealthStarting  HealthStatus = "starting"
)

// ServiceStatus contains the current status of a service
type ServiceStatus struct {
	Name         string        `json:"name"`
	State        ServiceState  `json:"state"`
	Health       HealthStatus  `json:"health"`
	PID          int           `json:"pid"`
	Port         int           `json:"port"`
	Uptime       time.Duration `json:"uptime"`
	RestartCount int           `json:"restart_count"`
	LastError    string        `json:"last_error,omitempty"`
}

// LogLine represents a line of service output
type LogLine struct {
	Service   string    `json:"service"`
	Timestamp time.Time `json:"timestamp"`
	Stream    string    `json:"stream"` // stdout or stderr
	Message   string    `json:"message"`
}

// Service manages a single service process
type Service struct {
	Name   string
	Config config.Service
	Port   int

	mu           sync.RWMutex
	state        ServiceState
	health       HealthStatus
	cmd          *exec.Cmd
	startTime    time.Time
	restartCount int
	lastError    string

	// Channels
	logs     chan LogLine
	done     chan struct{}
	stopOnce sync.Once

	// Log buffer for health check pattern matching
	logBuffer   []string
	logBufferMu sync.RWMutex
}

// NewService creates a new service instance
func NewService(name string, cfg config.Service, port int) *Service {
	return &Service{
		Name:   name,
		Config: cfg,
		Port:   port,
		state:  StateIdle,
		health: HealthUnknown,
		logs:   make(chan LogLine, 1000),
	}
}

// Start starts the service
func (s *Service) Start(ctx context.Context, workDir string, env []string) error {
	s.mu.Lock()
	if s.state == StateRunning || s.state == StateStarting {
		s.mu.Unlock()
		return fmt.Errorf("service %s is already running", s.Name)
	}

	s.state = StateStarting
	s.health = HealthStarting
	s.done = make(chan struct{})
	s.stopOnce = sync.Once{}
	s.mu.Unlock()

	// Determine working directory
	cmdDir := workDir
	if s.Config.WorkingDir != "" {
		cmdDir = s.Config.WorkingDir
		if cmdDir[0] != '/' {
			cmdDir = workDir + "/" + cmdDir
		}
	}

	// Create the command
	s.cmd = exec.CommandContext(ctx, "sh", "-c", s.Config.Run)
	s.cmd.Dir = cmdDir
	s.cmd.Env = env

	// Set up process group for clean termination
	s.cmd.SysProcAttr = &syscall.SysProcAttr{
		Setpgid: true,
	}

	// Capture stdout and stderr
	stdout, err := s.cmd.StdoutPipe()
	if err != nil {
		s.setError(err)
		return err
	}

	stderr, err := s.cmd.StderrPipe()
	if err != nil {
		s.setError(err)
		return err
	}

	// Start the process
	if err := s.cmd.Start(); err != nil {
		s.setError(err)
		return err
	}

	s.mu.Lock()
	s.state = StateRunning
	s.startTime = time.Now()
	s.mu.Unlock()

	// Stream output
	go s.streamOutput(stdout, "stdout")
	go s.streamOutput(stderr, "stderr")

	// Wait for process in background
	go s.waitForExit()

	return nil
}

// Stop stops the service gracefully
func (s *Service) Stop(ctx context.Context) error {
	s.mu.Lock()
	if s.state != StateRunning && s.state != StateStarting {
		s.mu.Unlock()
		return nil
	}

	s.state = StateStopping
	s.mu.Unlock()

	return s.terminate(ctx)
}

// terminate sends signals to stop the process
func (s *Service) terminate(ctx context.Context) error {
	if s.cmd == nil || s.cmd.Process == nil {
		return nil
	}

	pid := s.cmd.Process.Pid

	// Get process group ID (we set Setpgid: true on start)
	pgid, pgidErr := syscall.Getpgid(pid)

	// Send SIGTERM to process group first
	if pgidErr == nil && pgid > 0 {
		_ = syscall.Kill(-pgid, syscall.SIGTERM)
	}
	// Also send directly to the main process
	_ = s.cmd.Process.Signal(syscall.SIGTERM)

	// Wait for graceful shutdown with shorter timeout
	select {
	case <-s.done:
		return nil
	case <-time.After(5 * time.Second):
		// Process didn't exit gracefully, force kill
	case <-ctx.Done():
		// Context cancelled, force kill
	}

	// Force kill: send SIGKILL to process group and main process
	if pgidErr == nil && pgid > 0 {
		_ = syscall.Kill(-pgid, syscall.SIGKILL)
	}
	_ = s.cmd.Process.Kill()

	// Also try to kill any child processes that might have escaped
	// by using pkill with the parent PID (best effort)
	killCmd := exec.Command("pkill", "-9", "-P", fmt.Sprintf("%d", pid))
	_ = killCmd.Run()

	// Wait briefly for process to die
	select {
	case <-s.done:
		return nil
	case <-time.After(2 * time.Second):
		return fmt.Errorf("service %s did not stop after SIGKILL", s.Name)
	}
}

// waitForExit waits for the process to exit
func (s *Service) waitForExit() {
	defer s.stopOnce.Do(func() {
		close(s.done)
	})

	err := s.cmd.Wait()

	s.mu.Lock()
	defer s.mu.Unlock()

	if s.state == StateStopping {
		s.state = StateStopped
	} else {
		// Process exited unexpectedly
		s.state = StateFailed
		if err != nil {
			s.lastError = err.Error()
		} else {
			s.lastError = "process exited unexpectedly"
		}
	}

	s.health = HealthUnknown
}

// streamOutput reads from a pipe and sends to the log channel
func (s *Service) streamOutput(r io.Reader, stream string) {
	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		msg := scanner.Text()

		// Add to log buffer for health check pattern matching
		s.AddToLogBuffer(msg)

		line := LogLine{
			Service:   s.Name,
			Timestamp: time.Now(),
			Stream:    stream,
			Message:   msg,
		}

		select {
		case s.logs <- line:
		default:
			// Buffer full, drop oldest
			select {
			case <-s.logs:
			default:
			}
			s.logs <- line
		}
	}
}

// Status returns the current service status
func (s *Service) Status() ServiceStatus {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var uptime time.Duration
	if s.state == StateRunning && !s.startTime.IsZero() {
		uptime = time.Since(s.startTime)
	}

	var pid int
	if s.cmd != nil && s.cmd.Process != nil {
		pid = s.cmd.Process.Pid
	}

	return ServiceStatus{
		Name:         s.Name,
		State:        s.state,
		Health:       s.health,
		PID:          pid,
		Port:         s.Port,
		Uptime:       uptime,
		RestartCount: s.restartCount,
		LastError:    s.lastError,
	}
}

// Logs returns the log channel
func (s *Service) Logs() <-chan LogLine {
	return s.logs
}

// Done returns a channel that closes when the service exits
func (s *Service) Done() <-chan struct{} {
	return s.done
}

// SetHealth updates the health status
func (s *Service) SetHealth(health HealthStatus) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.health = health
}

// IncrementRestarts increments the restart counter
func (s *Service) IncrementRestarts() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.restartCount++
}

// setError sets the service to failed state with an error
func (s *Service) setError(err error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.state = StateFailed
	s.lastError = err.Error()
	s.health = HealthUnhealthy
}

// IsRunning returns true if the service is running
func (s *Service) IsRunning() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.state == StateRunning
}

// GetState returns the current state
func (s *Service) GetState() ServiceState {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.state
}

// AddToLogBuffer adds a log line to the buffer for pattern matching
func (s *Service) AddToLogBuffer(msg string) {
	s.logBufferMu.Lock()
	defer s.logBufferMu.Unlock()

	s.logBuffer = append(s.logBuffer, msg)
	// Keep only last 100 lines
	if len(s.logBuffer) > 100 {
		s.logBuffer = s.logBuffer[len(s.logBuffer)-100:]
	}
}

// HasLogMatch checks if any buffered log matches the pattern
func (s *Service) HasLogMatch(pattern *regexp.Regexp) bool {
	s.logBufferMu.RLock()
	defer s.logBufferMu.RUnlock()

	for _, line := range s.logBuffer {
		if pattern.MatchString(line) {
			return true
		}
	}
	return false
}

// ClearLogBuffer clears the log buffer
func (s *Service) ClearLogBuffer() {
	s.logBufferMu.Lock()
	defer s.logBufferMu.Unlock()
	s.logBuffer = nil
}

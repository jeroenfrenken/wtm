package process

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/jeroenfrenken/worktree-manager/internal/config"
)

// HealthManager manages health checks for all services
type HealthManager struct {
	supervisor *Supervisor
	checkers   map[string]context.CancelFunc
	mu         sync.Mutex
}

// NewHealthManager creates a new health manager
func NewHealthManager(supervisor *Supervisor) *HealthManager {
	return &HealthManager{
		supervisor: supervisor,
		checkers:   make(map[string]context.CancelFunc),
	}
}

// StartChecking starts health checking for a service
func (m *HealthManager) StartChecking(svc *Service) {
	if svc.Config.HealthCheck == nil {
		// No health check configured, assume healthy when running
		svc.SetHealth(HealthHealthy)
		return
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	// Cancel existing checker if any
	if cancel, ok := m.checkers[svc.Name]; ok {
		cancel()
	}

	ctx, cancel := context.WithCancel(context.Background())
	m.checkers[svc.Name] = cancel

	go m.runChecker(ctx, svc)
}

// StopChecking stops health checking for a service
func (m *HealthManager) StopChecking(name string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if cancel, ok := m.checkers[name]; ok {
		cancel()
		delete(m.checkers, name)
	}
}

// StopAll stops all health checkers
func (m *HealthManager) StopAll() {
	m.mu.Lock()
	defer m.mu.Unlock()

	for name, cancel := range m.checkers {
		cancel()
		delete(m.checkers, name)
	}
}

// runChecker runs the health check loop for a service
func (m *HealthManager) runChecker(ctx context.Context, svc *Service) {
	hc := svc.Config.HealthCheck

	interval := hc.Interval
	if interval == 0 {
		interval = 10 * time.Second // Reduced frequency for less CPU usage
	}

	timeout := hc.Timeout
	if timeout == 0 {
		timeout = 30 * time.Second
	}

	// Initial startup period
	startTime := time.Now()
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if !svc.IsRunning() {
				svc.SetHealth(HealthUnknown)
				continue
			}

			healthy, err := m.check(ctx, svc)

			if healthy {
				svc.SetHealth(HealthHealthy)
				m.supervisor.emit(EventHealthy, svc.Name, "health check passed")
			} else {
				// Check if still in startup period
				if time.Since(startTime) < timeout {
					svc.SetHealth(HealthStarting)
				} else {
					svc.SetHealth(HealthUnhealthy)
					errMsg := "health check failed"
					if err != nil {
						errMsg = err.Error()
					}
					m.supervisor.emit(EventUnhealthy, svc.Name, errMsg)
				}
			}
		}
	}
}

// check performs a single health check
func (m *HealthManager) check(ctx context.Context, svc *Service) (bool, error) {
	hc := svc.Config.HealthCheck

	switch hc.Type {
	case config.HealthCheckHTTP:
		return m.checkHTTP(ctx, svc)
	case config.HealthCheckPort:
		return m.checkPort(ctx, svc)
	case config.HealthCheckLogMatch:
		return m.checkLogMatch(ctx, svc)
	default:
		return false, fmt.Errorf("unknown health check type: %s", hc.Type)
	}
}

// checkHTTP performs an HTTP health check
func (m *HealthManager) checkHTTP(ctx context.Context, svc *Service) (bool, error) {
	hc := svc.Config.HealthCheck

	// Substitute port in URL
	url := strings.ReplaceAll(hc.URL, "$PORT", fmt.Sprintf("%d", svc.Port))
	url = strings.ReplaceAll(url, "${PORT}", fmt.Sprintf("%d", svc.Port))

	client := &http.Client{
		Timeout: 5 * time.Second,
	}

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return false, err
	}

	resp, err := client.Do(req)
	if err != nil {
		return false, err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 200 && resp.StatusCode < 400 {
		return true, nil
	}

	return false, fmt.Errorf("HTTP %d", resp.StatusCode)
}

// checkPort performs a TCP port health check
func (m *HealthManager) checkPort(ctx context.Context, svc *Service) (bool, error) {
	hc := svc.Config.HealthCheck

	port := hc.Port
	if port == 0 {
		port = svc.Port
	}

	addr := fmt.Sprintf("localhost:%d", port)

	dialer := net.Dialer{
		Timeout: 5 * time.Second,
	}

	conn, err := dialer.DialContext(ctx, "tcp", addr)
	if err != nil {
		return false, err
	}
	conn.Close()

	return true, nil
}

// checkLogMatch checks if a pattern appears in service logs
// Note: This uses the service's matched flag which is set by the log forwarder
func (m *HealthManager) checkLogMatch(ctx context.Context, svc *Service) (bool, error) {
	hc := svc.Config.HealthCheck

	pattern, err := regexp.Compile(hc.Pattern)
	if err != nil {
		return false, fmt.Errorf("invalid pattern: %w", err)
	}

	// Check if pattern has been matched (set by log forwarder)
	if svc.HasLogMatch(pattern) {
		return true, nil
	}

	return false, fmt.Errorf("pattern not found in logs")
}

// HealthChecker is the interface for health check implementations
type HealthChecker interface {
	Check(ctx context.Context, svc *Service) (bool, error)
}

// HTTPHealthChecker checks HTTP endpoints
type HTTPHealthChecker struct {
	URL     string
	Timeout time.Duration
}

// Check performs the HTTP health check
func (c *HTTPHealthChecker) Check(ctx context.Context, svc *Service) (bool, error) {
	url := strings.ReplaceAll(c.URL, "$PORT", fmt.Sprintf("%d", svc.Port))

	client := &http.Client{Timeout: c.Timeout}
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return false, err
	}

	resp, err := client.Do(req)
	if err != nil {
		return false, err
	}
	defer resp.Body.Close()

	return resp.StatusCode >= 200 && resp.StatusCode < 400, nil
}

// PortHealthChecker checks if a port is open
type PortHealthChecker struct {
	Port    int
	Timeout time.Duration
}

// Check performs the port health check
func (c *PortHealthChecker) Check(ctx context.Context, svc *Service) (bool, error) {
	port := c.Port
	if port == 0 {
		port = svc.Port
	}

	conn, err := net.DialTimeout("tcp", fmt.Sprintf("localhost:%d", port), c.Timeout)
	if err != nil {
		return false, err
	}
	conn.Close()
	return true, nil
}

package ui

import (
	"context"
	"embed"
	"fmt"
	"html/template"
	"io/fs"
	"net/http"
	"sync"
	"time"

	"github.com/jeroenfrenken/worktree-manager/internal/config"
	"github.com/jeroenfrenken/worktree-manager/internal/process"
	"github.com/jeroenfrenken/worktree-manager/internal/proxy"
	"github.com/jeroenfrenken/worktree-manager/internal/worktree"
)

//go:embed templates/*
var templateFS embed.FS

//go:embed static/*
var staticFS embed.FS

// WorktreeStatus represents the overall status of a worktree
type WorktreeStatus string

const (
	WorktreeStatusRunning WorktreeStatus = "running" // All services running
	WorktreeStatusPartial WorktreeStatus = "partial" // Some services running
	WorktreeStatusStopped WorktreeStatus = "stopped" // No services running
	WorktreeStatusError   WorktreeStatus = "error"   // Has failed services
)

// WorktreeInfo contains worktree data with status
type WorktreeInfo struct {
	*worktree.Worktree
	Status   WorktreeStatus                  `json:"status"`
	Services map[string]process.ServiceStatus `json:"services,omitempty"`
}

// Server is the web UI server
type Server struct {
	httpServer  *http.Server
	cfg         *config.Config
	projectDir  string
	hub         *Hub
	templates   *template.Template
	proxy       *proxy.Proxy
	mu          sync.RWMutex
	selectedWT  string                          // Currently selected tab (for UI display)
	supervisors map[string]*process.Supervisor  // One supervisor per worktree
	worktreeMgr *worktree.Manager
}

// NewServer creates a new UI server
func NewServer(cfg *config.Config, projectDir string, port int) (*Server, error) {
	// Parse templates
	tmpl, err := template.ParseFS(templateFS, "templates/*.html")
	if err != nil {
		return nil, fmt.Errorf("parsing templates: %w", err)
	}

	hub := NewHub()
	go hub.Run()

	// Create worktree manager
	wtMgr, err := worktree.NewManager(projectDir)
	if err != nil {
		return nil, fmt.Errorf("creating worktree manager: %w", err)
	}

	// Create proxy
	proxyPort := cfg.ProxyPort
	if proxyPort == 0 {
		proxyPort = 8080
	}
	proxyServer := proxy.NewProxy(proxyPort)

	s := &Server{
		cfg:         cfg,
		projectDir:  projectDir,
		hub:         hub,
		templates:   tmpl,
		proxy:       proxyServer,
		selectedWT:  "main",
		supervisors: make(map[string]*process.Supervisor),
		worktreeMgr: wtMgr,
	}

	// Create router
	mux := http.NewServeMux()

	// Static files
	staticSub, _ := fs.Sub(staticFS, "static")
	mux.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.FS(staticSub))))

	// Pages
	mux.HandleFunc("/", s.handleDashboard)
	mux.HandleFunc("/worktree/", s.handleWorktree)

	// API
	mux.HandleFunc("/api/status", s.handleAPIStatus)
	mux.HandleFunc("/api/worktrees", s.handleAPIWorktrees)
	mux.HandleFunc("/api/worktrees/", s.handleAPIWorktreeAction)
	mux.HandleFunc("/api/services", s.handleAPIServices)
	mux.HandleFunc("/api/services/", s.handleAPIService)
	mux.HandleFunc("/api/hosts/", s.handleAPIHosts)
	mux.HandleFunc("/api/terminal/", s.handleAPITerminal)
	mux.HandleFunc("/api/logs", s.handleAPILogs)
	mux.HandleFunc("/api/proxy/routes", s.handleAPIProxyRoutes)

	// WebSocket
	mux.HandleFunc("/ws", s.handleWebSocket)

	s.httpServer = &http.Server{
		Addr:         fmt.Sprintf("127.0.0.1:%d", port),
		Handler:      mux,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Start periodic status broadcast
	go s.statusBroadcastLoop()

	return s, nil
}

// statusBroadcastLoop periodically broadcasts status to all clients
func (s *Server) statusBroadcastLoop() {
	ticker := time.NewTicker(5 * time.Second) // Reduced frequency
	defer ticker.Stop()
	for range ticker.C {
		// Only broadcast if there are connected clients
		if s.hub.ClientCount() > 0 {
			s.broadcastAllStatus()
		}
	}
}

// broadcastAllStatus sends status for all worktrees
func (s *Server) broadcastAllStatus() {
	status := s.GetAllWorktreeStatus()
	s.hub.Broadcast(Message{
		Type: "worktree_status",
		Data: status,
	})
}

// GetWorktree returns a worktree by name (including "main")
func (s *Server) GetWorktree(name string) *worktree.Worktree {
	if name == "main" {
		return &worktree.Worktree{
			Name:     "main",
			Path:     s.projectDir,
			Branch:   "main",
			BasePort: s.cfg.PortRangeStart,
			Index:    0,
		}
	}
	wt, _ := s.worktreeMgr.Get(name)
	return wt
}

// GetAllWorktrees returns all worktrees including main
func (s *Server) GetAllWorktrees() []*worktree.Worktree {
	worktrees := []*worktree.Worktree{
		{
			Name:     "main",
			Path:     s.projectDir,
			Branch:   "main",
			BasePort: s.cfg.PortRangeStart,
			Index:    0,
		},
	}
	worktrees = append(worktrees, s.worktreeMgr.List()...)
	return worktrees
}

// GetAllWorktreeStatus returns status info for all worktrees
func (s *Server) GetAllWorktreeStatus() map[string]*WorktreeInfo {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make(map[string]*WorktreeInfo)

	for _, wt := range s.GetAllWorktrees() {
		info := &WorktreeInfo{
			Worktree: wt,
			Status:   WorktreeStatusStopped,
		}

		if sup, ok := s.supervisors[wt.Name]; ok {
			services := sup.Status()
			info.Services = services

			running := 0
			failed := 0
			total := len(services)

			for _, st := range services {
				if st.State == process.StateRunning {
					running++
				}
				if st.State == process.StateFailed {
					failed++
				}
			}

			if failed > 0 {
				info.Status = WorktreeStatusError
			} else if running == total && total > 0 {
				info.Status = WorktreeStatusRunning
			} else if running > 0 {
				info.Status = WorktreeStatusPartial
			} else {
				info.Status = WorktreeStatusStopped
			}
		}

		result[wt.Name] = info
	}

	return result
}

// StartWorktree starts services for a specific worktree
func (s *Server) StartWorktree(ctx context.Context, wtName string) error {
	wt := s.GetWorktree(wtName)
	if wt == nil {
		return fmt.Errorf("worktree %s not found", wtName)
	}

	s.mu.Lock()
	// Stop existing supervisor if any
	if sup, ok := s.supervisors[wtName]; ok {
		stopCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
		sup.Stop(stopCtx)
		cancel()
		delete(s.supervisors, wtName)
	}
	s.mu.Unlock()

	// Create new supervisor
	supervisor, err := process.NewSupervisor(s.cfg, wt.Name, wt.Path, wt.BasePort)
	if err != nil {
		return err
	}

	// Start services
	if err := supervisor.Start(ctx); err != nil {
		return err
	}

	s.mu.Lock()
	s.supervisors[wtName] = supervisor
	s.mu.Unlock()

	// Register proxy routes
	s.registerProxyRoutesForWorktree(wt, supervisor)

	// Forward logs and events
	go s.forwardSupervisorOutput(wtName, supervisor)

	// Broadcast updated status
	s.broadcastAllStatus()

	return nil
}

// StopWorktree stops services for a specific worktree
func (s *Server) StopWorktree(ctx context.Context, wtName string) error {
	s.mu.Lock()
	sup, ok := s.supervisors[wtName]
	if !ok {
		s.mu.Unlock()
		return nil // Already stopped
	}
	delete(s.supervisors, wtName)
	s.mu.Unlock()

	// Remove proxy routes
	s.removeProxyRoutesForWorktree(wtName)

	// Stop supervisor
	if err := sup.Stop(ctx); err != nil {
		return err
	}

	// Broadcast updated status
	s.broadcastAllStatus()

	return nil
}

// forwardSupervisorOutput forwards logs and events from a supervisor
func (s *Server) forwardSupervisorOutput(wtName string, supervisor *process.Supervisor) {
	// Forward logs
	go func() {
		for log := range supervisor.Logs() {
			s.hub.Broadcast(Message{
				Type: "log",
				Data: log,
			})
		}
	}()

	// Forward events
	go func() {
		for event := range supervisor.Events() {
			s.hub.Broadcast(Message{
				Type: "event",
				Data: map[string]interface{}{
					"worktree": wtName,
					"event":    event,
				},
			})
			// Broadcast status update
			s.broadcastAllStatus()
		}
	}()
}

// registerProxyRoutesForWorktree registers proxy routes for a worktree
func (s *Server) registerProxyRoutesForWorktree(wt *worktree.Worktree, sup *process.Supervisor) {
	status := sup.Status()

	for name := range s.cfg.Services {
		if st, ok := status[name]; ok && st.Port > 0 {
			domain := fmt.Sprintf("%s.%s.%s", name, wt.Name, s.cfg.Domain)
			s.proxy.AddRoute(domain, st.Port, name, wt.Name)
			fmt.Printf("Proxy route: %s -> localhost:%d\n", domain, st.Port)
		}
	}
}

// removeProxyRoutesForWorktree removes proxy routes for a worktree
func (s *Server) removeProxyRoutesForWorktree(wtName string) {
	for name := range s.cfg.Services {
		domain := fmt.Sprintf("%s.%s.%s", name, wtName, s.cfg.Domain)
		s.proxy.RemoveRoute(domain)
	}
}

// SelectWorktree selects a worktree for display (doesn't affect running services)
func (s *Server) SelectWorktree(wtName string) {
	s.mu.Lock()
	s.selectedWT = wtName
	s.mu.Unlock()
}

// GetSelectedWorktree returns the currently selected worktree name
func (s *Server) GetSelectedWorktree() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.selectedWT
}

// GetSupervisor returns the supervisor for a worktree
func (s *Server) GetSupervisor(wtName string) *process.Supervisor {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.supervisors[wtName]
}

// GetProxyRoutes returns the current proxy routes
func (s *Server) GetProxyRoutes() []*proxy.Route {
	return s.proxy.Routes()
}

// Start starts the UI server and proxy
func (s *Server) Start() error {
	// Start proxy in background (non-fatal if it fails)
	go func() {
		if err := s.proxy.Start(); err != nil {
			if err != http.ErrServerClosed {
				fmt.Printf("\nWarning: Proxy failed to start on port %d: %v\n", s.cfg.ProxyPort, err)
				fmt.Printf("  To fix: lsof -ti:%d | xargs kill -9\n", s.cfg.ProxyPort)
				fmt.Println("  The UI will work but domain-based routing won't be available.\n")
			}
		}
	}()

	fmt.Printf("Web UI available at http://%s\n", s.httpServer.Addr)
	fmt.Printf("Reverse proxy on :%d\n", s.cfg.ProxyPort)
	return s.httpServer.ListenAndServe()
}

// Stop stops the UI server, proxy, and all supervisors
func (s *Server) Stop(ctx context.Context) error {
	// Stop all supervisors
	s.mu.Lock()
	supervisorCount := len(s.supervisors)
	if supervisorCount > 0 {
		fmt.Printf("Stopping %d worktree(s)...\n", supervisorCount)
	}
	for name, sup := range s.supervisors {
		fmt.Printf("  Stopping %s...\n", name)
		if err := sup.Stop(ctx); err != nil {
			fmt.Printf("  Warning: error stopping %s: %v\n", name, err)
		}
		delete(s.supervisors, name)
	}
	s.mu.Unlock()

	fmt.Println("Stopping proxy...")
	s.proxy.Stop(ctx)

	fmt.Println("Stopping web server...")
	return s.httpServer.Shutdown(ctx)
}

// handleDashboard renders the main dashboard
func (s *Server) handleDashboard(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}

	selectedWT := s.GetSelectedWorktree()
	allWorktrees := s.GetAllWorktrees()
	wtStatus := s.GetAllWorktreeStatus()

	// Get the selected worktree info
	var activeWT *worktree.Worktree
	for _, wt := range allWorktrees {
		if wt.Name == selectedWT {
			activeWT = wt
			break
		}
	}

	data := map[string]interface{}{
		"Config":          s.cfg,
		"ActiveWT":        activeWT,
		"SelectedWT":      selectedWT,
		"Worktrees":       s.worktreeMgr.List(), // Non-main worktrees
		"AllWorktrees":    allWorktrees,
		"WorktreeStatus":  wtStatus,
		"Services":        s.cfg.Services,
		"Tasks":           s.cfg.Tasks,
		"ProxyPort":       s.cfg.ProxyPort,
	}

	if err := s.templates.ExecuteTemplate(w, "dashboard.html", data); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

// handleWorktree renders a worktree detail page
func (s *Server) handleWorktree(w http.ResponseWriter, r *http.Request) {
	name := r.URL.Path[len("/worktree/"):]
	if name == "" {
		http.Redirect(w, r, "/", http.StatusFound)
		return
	}

	wt := s.GetWorktree(name)
	if wt == nil {
		http.NotFound(w, r)
		return
	}

	wtStatus := s.GetAllWorktreeStatus()

	data := map[string]interface{}{
		"Config":         s.cfg,
		"Worktree":       wt,
		"WorktreeStatus": wtStatus,
		"Services":       s.cfg.Services,
		"Tasks":          s.cfg.Tasks,
	}

	if err := s.templates.ExecuteTemplate(w, "worktree.html", data); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

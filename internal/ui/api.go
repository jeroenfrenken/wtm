package ui

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"path/filepath"
	"strings"
	"time"

	"github.com/jeroenfrenken/worktree-manager/internal/dns"
	"github.com/jeroenfrenken/worktree-manager/internal/terminal"
)

// APIResponse is a standard API response
type APIResponse struct {
	Success bool        `json:"success"`
	Data    interface{} `json:"data,omitempty"`
	Error   string      `json:"error,omitempty"`
}

// handleAPIStatus returns the current status of all worktrees
func (s *Server) handleAPIStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		s.jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	status := map[string]interface{}{
		"project":        s.cfg.Project,
		"domain":         s.cfg.Domain,
		"proxy_port":     s.cfg.ProxyPort,
		"ui_port":        s.cfg.UIPort,
		"selected":       s.GetSelectedWorktree(),
		"worktrees":      s.GetAllWorktreeStatus(),
	}

	s.jsonSuccess(w, status)
}

// handleAPIWorktrees handles worktree listing and creation
func (s *Server) handleAPIWorktrees(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		s.jsonSuccess(w, map[string]interface{}{
			"worktrees": s.GetAllWorktreeStatus(),
			"selected":  s.GetSelectedWorktree(),
		})

	case http.MethodPost:
		var req struct {
			Name      string `json:"name"`
			Branch    string `json:"branch"`
			SyncHosts bool   `json:"syncHosts"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			s.jsonError(w, "Invalid request body", http.StatusBadRequest)
			return
		}

		if req.Name == "" {
			s.jsonError(w, "Name is required", http.StatusBadRequest)
			return
		}

		ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		defer cancel()

		wt, err := s.worktreeMgr.Create(ctx, req.Name, req.Branch)
		if err != nil {
			s.jsonError(w, err.Error(), http.StatusInternalServerError)
			return
		}

		// Auto-sync hosts if requested
		if req.SyncHosts {
			domains := s.buildDomains(req.Name)
			if len(domains) > 0 {
				hostsMgr := dns.NewHostsManager()
				_ = s.syncHostsForWorktree(hostsMgr, req.Name, domains)
			}
		}

		s.jsonSuccess(w, wt)

	default:
		s.jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// handleAPIWorktreeAction handles actions on a specific worktree
func (s *Server) handleAPIWorktreeAction(w http.ResponseWriter, r *http.Request) {
	// Parse URL: /api/worktrees/{name}/{action}
	path := strings.TrimPrefix(r.URL.Path, "/api/worktrees/")
	parts := strings.Split(path, "/")

	if len(parts) < 1 || parts[0] == "" {
		s.jsonError(w, "Worktree name required", http.StatusBadRequest)
		return
	}

	wtName := parts[0]
	action := ""
	if len(parts) > 1 {
		action = parts[1]
	}

	switch action {
	case "select", "switch":
		if r.Method != http.MethodPost {
			s.jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		s.SelectWorktree(wtName)
		s.jsonSuccess(w, map[string]string{"message": "Selected worktree", "selected": wtName})

	case "start":
		if r.Method != http.MethodPost {
			s.jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		s.handleWorktreeStart(w, r, wtName)

	case "stop":
		if r.Method != http.MethodPost {
			s.jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		s.handleWorktreeStop(w, r, wtName)

	case "terminal":
		if r.Method != http.MethodPost {
			s.jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		s.handleWorktreeTerminal(w, r, wtName)

	case "delete":
		if r.Method != http.MethodPost && r.Method != http.MethodDelete {
			s.jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		s.handleWorktreeDelete(w, r, wtName)

	default:
		// Just get worktree info
		if r.Method != http.MethodGet {
			s.jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		wt := s.GetWorktree(wtName)
		if wt == nil {
			s.jsonError(w, "Worktree not found", http.StatusNotFound)
			return
		}
		status := s.GetAllWorktreeStatus()
		s.jsonSuccess(w, status[wtName])
	}
}

func (s *Server) handleWorktreeStart(w http.ResponseWriter, r *http.Request, wtName string) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	// Sync hosts first
	domains := s.buildDomains(wtName)
	if len(domains) > 0 {
		hostsMgr := dns.NewHostsManager()
		_ = s.syncHostsForWorktree(hostsMgr, wtName, domains)
	}

	// Start services for this worktree
	if err := s.StartWorktree(ctx, wtName); err != nil {
		s.jsonError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	s.jsonSuccess(w, map[string]string{"message": "Services started for " + wtName})
}

func (s *Server) handleWorktreeStop(w http.ResponseWriter, r *http.Request, wtName string) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := s.StopWorktree(ctx, wtName); err != nil {
		s.jsonError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	s.jsonSuccess(w, map[string]string{"message": "Services stopped for " + wtName})
}

func (s *Server) handleWorktreeTerminal(w http.ResponseWriter, r *http.Request, wtName string) {
	wt := s.GetWorktree(wtName)
	if wt == nil {
		s.jsonError(w, "Worktree not found", http.StatusNotFound)
		return
	}

	factory := terminal.NewFactory(s.cfg.Terminal.Emulator, s.cfg.Terminal.Profile)
	term, err := factory.Get()
	if err != nil {
		s.jsonError(w, fmt.Sprintf("Terminal error: %v", err), http.StatusInternalServerError)
		return
	}

	err = term.OpenWindow(terminal.OpenOptions{
		Title:      fmt.Sprintf("wtm: %s", wt.Name),
		WorkingDir: wt.Path,
	})
	if err != nil {
		s.jsonError(w, fmt.Sprintf("Failed to open terminal: %v", err), http.StatusInternalServerError)
		return
	}

	s.jsonSuccess(w, map[string]string{"message": fmt.Sprintf("Terminal opened in %s", wt.Path)})
}

func (s *Server) handleWorktreeDelete(w http.ResponseWriter, r *http.Request, wtName string) {
	// Can't delete main
	if wtName == "main" {
		s.jsonError(w, "Cannot delete main worktree", http.StatusBadRequest)
		return
	}

	// Stop services first if running
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	_ = s.StopWorktree(ctx, wtName)

	if err := s.worktreeMgr.Delete(ctx, wtName, false); err != nil {
		s.jsonError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	s.jsonSuccess(w, map[string]string{"message": "Worktree deleted"})
}

// handleAPIServices handles service listing
func (s *Server) handleAPIServices(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		s.jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	selectedWT := s.GetSelectedWorktree()
	services := make(map[string]interface{})

	for name, svc := range s.cfg.Services {
		svcData := map[string]interface{}{
			"name":        name,
			"run":         svc.Run,
			"working_dir": svc.WorkingDir,
			"port_offset": svc.PortOffset,
			"depends_on":  svc.DependsOn,
			"status":      nil,
		}

		sup := s.GetSupervisor(selectedWT)
		if sup != nil {
			status := sup.Status()
			if st, ok := status[name]; ok {
				svcData["status"] = st
			}
		}

		services[name] = svcData
	}

	s.jsonSuccess(w, services)
}

// handleAPIService handles single service operations
func (s *Server) handleAPIService(w http.ResponseWriter, r *http.Request) {
	// Parse URL: /api/services/{name}/{action}
	path := strings.TrimPrefix(r.URL.Path, "/api/services/")
	parts := strings.Split(path, "/")

	if len(parts) < 1 || parts[0] == "" {
		s.jsonError(w, "Service name required", http.StatusBadRequest)
		return
	}

	serviceName := parts[0]
	action := ""
	if len(parts) > 1 {
		action = parts[1]
	}

	// Handle "all" services operations
	if serviceName == "all" {
		s.handleAllServicesAction(w, r, action)
		return
	}

	// Check if service exists
	if _, ok := s.cfg.Services[serviceName]; !ok {
		s.jsonError(w, "Service not found", http.StatusNotFound)
		return
	}

	selectedWT := s.GetSelectedWorktree()
	sup := s.GetSupervisor(selectedWT)
	if sup == nil {
		s.jsonError(w, "Services not running for selected worktree", http.StatusServiceUnavailable)
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	switch action {
	case "start":
		if r.Method != http.MethodPost {
			s.jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if err := sup.StartService(ctx, serviceName); err != nil {
			s.jsonError(w, err.Error(), http.StatusInternalServerError)
			return
		}
		s.jsonSuccess(w, map[string]string{"message": "Service started"})

	case "stop":
		if r.Method != http.MethodPost {
			s.jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if err := sup.StopService(ctx, serviceName); err != nil {
			s.jsonError(w, err.Error(), http.StatusInternalServerError)
			return
		}
		s.jsonSuccess(w, map[string]string{"message": "Service stopped"})

	case "restart":
		if r.Method != http.MethodPost {
			s.jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if err := sup.RestartService(ctx, serviceName); err != nil {
			s.jsonError(w, err.Error(), http.StatusInternalServerError)
			return
		}
		s.jsonSuccess(w, map[string]string{"message": "Service restarted"})

	case "":
		// Get service status
		if r.Method != http.MethodGet {
			s.jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		status := sup.Status()
		if st, ok := status[serviceName]; ok {
			s.jsonSuccess(w, st)
		} else {
			s.jsonError(w, "Service not running", http.StatusNotFound)
		}

	default:
		s.jsonError(w, "Unknown action", http.StatusBadRequest)
	}
}

// handleAllServicesAction handles operations on all services for selected worktree
func (s *Server) handleAllServicesAction(w http.ResponseWriter, r *http.Request, action string) {
	if r.Method != http.MethodPost {
		s.jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	selectedWT := s.GetSelectedWorktree()

	switch action {
	case "start":
		if err := s.StartWorktree(ctx, selectedWT); err != nil {
			s.jsonError(w, err.Error(), http.StatusInternalServerError)
			return
		}
		s.jsonSuccess(w, map[string]string{"message": "All services started"})

	case "stop":
		if err := s.StopWorktree(ctx, selectedWT); err != nil {
			s.jsonError(w, err.Error(), http.StatusInternalServerError)
			return
		}
		s.jsonSuccess(w, map[string]string{"message": "All services stopped"})

	case "restart":
		_ = s.StopWorktree(ctx, selectedWT)
		time.Sleep(500 * time.Millisecond)
		if err := s.StartWorktree(ctx, selectedWT); err != nil {
			s.jsonError(w, err.Error(), http.StatusInternalServerError)
			return
		}
		s.jsonSuccess(w, map[string]string{"message": "All services restarted"})

	default:
		s.jsonError(w, "Unknown action", http.StatusBadRequest)
	}
}

// handleAPITerminal handles terminal launch requests
func (s *Server) handleAPITerminal(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		s.jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Parse URL: /api/terminal/{worktree}/{service}
	path := strings.TrimPrefix(r.URL.Path, "/api/terminal/")
	parts := strings.Split(strings.TrimSuffix(path, "/"), "/")

	wtName := s.GetSelectedWorktree()
	serviceName := ""

	if len(parts) >= 1 && parts[0] != "" {
		// Check if it's a worktree name or service name
		if s.GetWorktree(parts[0]) != nil {
			wtName = parts[0]
			if len(parts) >= 2 {
				serviceName = parts[1]
			}
		} else {
			serviceName = parts[0]
		}
	}

	wt := s.GetWorktree(wtName)
	if wt == nil {
		s.jsonError(w, "Worktree not found", http.StatusBadRequest)
		return
	}

	workDir := wt.Path
	title := fmt.Sprintf("wtm: %s", wt.Name)

	if serviceName != "" && serviceName != "root" {
		svc, ok := s.cfg.Services[serviceName]
		if !ok {
			s.jsonError(w, "Service not found", http.StatusNotFound)
			return
		}
		if svc.WorkingDir != "" {
			if filepath.IsAbs(svc.WorkingDir) {
				workDir = svc.WorkingDir
			} else {
				workDir = filepath.Join(wt.Path, svc.WorkingDir)
			}
		}
		title = fmt.Sprintf("wtm: %s/%s", wt.Name, serviceName)
	}

	factory := terminal.NewFactory(s.cfg.Terminal.Emulator, s.cfg.Terminal.Profile)
	term, err := factory.Get()
	if err != nil {
		s.jsonError(w, fmt.Sprintf("Terminal error: %v", err), http.StatusInternalServerError)
		return
	}

	err = term.OpenWindow(terminal.OpenOptions{
		Title:      title,
		WorkingDir: workDir,
	})
	if err != nil {
		s.jsonError(w, fmt.Sprintf("Failed to open terminal: %v", err), http.StatusInternalServerError)
		return
	}

	s.jsonSuccess(w, map[string]string{"message": fmt.Sprintf("Terminal opened in %s", workDir)})
}

// handleAPIHosts handles hosts operations
func (s *Server) handleAPIHosts(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/api/hosts/")

	switch path {
	case "sync":
		if r.Method != http.MethodPost {
			s.jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		s.handleHostsSync(w, r)

	case "list":
		if r.Method != http.MethodGet {
			s.jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		s.handleHostsList(w, r)

	default:
		s.jsonError(w, "Unknown endpoint", http.StatusNotFound)
	}
}

func (s *Server) handleHostsSync(w http.ResponseWriter, r *http.Request) {
	// Sync hosts for all worktrees
	allDomains := make([]string, 0)
	for _, wt := range s.GetAllWorktrees() {
		domains := s.buildDomains(wt.Name)
		allDomains = append(allDomains, domains...)
	}

	if len(allDomains) == 0 {
		s.jsonSuccess(w, map[string]string{"message": "No services to sync"})
		return
	}

	hostsMgr := dns.NewHostsManager()
	if err := hostsMgr.Sync(allDomains); err != nil {
		s.jsonError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	s.jsonSuccess(w, map[string]interface{}{
		"message": "Hosts synced",
		"domains": allDomains,
	})
}

func (s *Server) handleHostsList(w http.ResponseWriter, r *http.Request) {
	hostsMgr := dns.NewHostsManager()
	entries, err := hostsMgr.List()
	if err != nil {
		s.jsonError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	s.jsonSuccess(w, entries)
}

// handleAPILogs returns recent logs
func (s *Server) handleAPILogs(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		s.jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	s.jsonSuccess(w, map[string]string{
		"message": "Use WebSocket at /ws for real-time logs",
	})
}

// buildDomains builds the list of domains for a worktree
func (s *Server) buildDomains(worktreeName string) []string {
	domains := make([]string, 0, len(s.cfg.Services))
	for serviceName := range s.cfg.Services {
		domain := fmt.Sprintf("%s.%s.%s", serviceName, worktreeName, s.cfg.Domain)
		domains = append(domains, domain)
	}
	return domains
}

// syncHostsForWorktree syncs hosts entries for a worktree while preserving others
func (s *Server) syncHostsForWorktree(hostsMgr *dns.HostsManager, worktreeName string, domains []string) error {
	existing, err := hostsMgr.List()
	if err != nil {
		return err
	}

	allDomains := make([]string, 0)

	for _, entry := range existing {
		isCurrentWT := false
		for _, d := range domains {
			if entry.Domain == d {
				isCurrentWT = true
				break
			}
		}
		if !isCurrentWT {
			allDomains = append(allDomains, entry.Domain)
		}
	}

	allDomains = append(allDomains, domains...)

	return hostsMgr.Sync(allDomains)
}

// jsonSuccess sends a successful JSON response
func (s *Server) jsonSuccess(w http.ResponseWriter, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(APIResponse{
		Success: true,
		Data:    data,
	})
}

// jsonError sends an error JSON response
func (s *Server) jsonError(w http.ResponseWriter, message string, status int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(APIResponse{
		Success: false,
		Error:   message,
	})
}

// handleAPIProxyRoutes returns current proxy routes
func (s *Server) handleAPIProxyRoutes(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		s.jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	routes := s.GetProxyRoutes()
	s.jsonSuccess(w, routes)
}

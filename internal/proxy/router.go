package proxy

import (
	"strings"
	"sync"
)

// Route represents a proxy route
type Route struct {
	Domain     string `json:"domain"`
	TargetPort int    `json:"target_port"`
	Service    string `json:"service"`
	Worktree   string `json:"worktree"`
}

// Router manages proxy routes
type Router struct {
	routes map[string]*Route
	mu     sync.RWMutex
}

// NewRouter creates a new router
func NewRouter() *Router {
	return &Router{
		routes: make(map[string]*Route),
	}
}

// Add adds a route
func (r *Router) Add(route *Route) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.routes[normalizeHost(route.Domain)] = route
}

// Remove removes a route
func (r *Router) Remove(domain string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.routes, normalizeHost(domain))
}

// Match finds a route for the given host
func (r *Router) Match(host string) *Route {
	r.mu.RLock()
	defer r.mu.RUnlock()

	// Remove port from host if present
	host = normalizeHost(host)

	// Exact match
	if route, ok := r.routes[host]; ok {
		return route
	}

	return nil
}

// List returns all routes
func (r *Router) List() []*Route {
	r.mu.RLock()
	defer r.mu.RUnlock()

	routes := make([]*Route, 0, len(r.routes))
	for _, route := range r.routes {
		routes = append(routes, route)
	}
	return routes
}

// Clear removes all routes
func (r *Router) Clear() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.routes = make(map[string]*Route)
}

// normalizeHost removes port from host
func normalizeHost(host string) string {
	if idx := strings.LastIndex(host, ":"); idx != -1 {
		// Check if it's a port (not IPv6)
		if !strings.Contains(host[idx:], "]") {
			host = host[:idx]
		}
	}
	return strings.ToLower(host)
}

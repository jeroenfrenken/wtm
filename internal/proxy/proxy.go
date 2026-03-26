package proxy

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httputil"
	"net/url"
	"sync"
	"time"
)

// Proxy is a reverse proxy server for routing requests to services
type Proxy struct {
	server  *http.Server
	router  *Router
	port    int
	proxies map[int]*httputil.ReverseProxy // Cache proxies by port
	mu      sync.RWMutex
}

// NewProxy creates a new reverse proxy
func NewProxy(port int) *Proxy {
	p := &Proxy{
		router:  NewRouter(),
		port:    port,
		proxies: make(map[int]*httputil.ReverseProxy),
	}

	p.server = &http.Server{
		Addr:         fmt.Sprintf(":%d", port),
		Handler:      p,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 60 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	return p
}

// Start starts the proxy server
func (p *Proxy) Start() error {
	return p.server.ListenAndServe()
}

// Stop stops the proxy server
func (p *Proxy) Stop(ctx context.Context) error {
	return p.server.Shutdown(ctx)
}

// getOrCreateProxy returns a cached reverse proxy or creates a new one
func (p *Proxy) getOrCreateProxy(targetPort int, service, worktree string) *httputil.ReverseProxy {
	p.mu.RLock()
	proxy, exists := p.proxies[targetPort]
	p.mu.RUnlock()

	if exists {
		return proxy
	}

	// Create new proxy
	target, _ := url.Parse(fmt.Sprintf("http://127.0.0.1:%d", targetPort))
	proxy = httputil.NewSingleHostReverseProxy(target)

	// Use a custom transport with connection pooling
	proxy.Transport = &http.Transport{
		MaxIdleConns:        100,
		MaxIdleConnsPerHost: 10,
		IdleConnTimeout:     90 * time.Second,
		DisableCompression:  true, // Let backend handle compression
	}

	// Error handling
	proxy.ErrorHandler = func(w http.ResponseWriter, r *http.Request, err error) {
		http.Error(w, fmt.Sprintf("Service unavailable: %v", err), http.StatusBadGateway)
	}

	// Add headers
	proxy.ModifyResponse = func(resp *http.Response) error {
		resp.Header.Set("X-WTM-Service", service)
		resp.Header.Set("X-WTM-Worktree", worktree)
		return nil
	}

	// Cache it
	p.mu.Lock()
	p.proxies[targetPort] = proxy
	p.mu.Unlock()

	return proxy
}

// ServeHTTP handles incoming requests
func (p *Proxy) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	route := p.router.Match(r.Host)
	if route == nil {
		http.Error(w, fmt.Sprintf("No route for host: %s", r.Host), http.StatusNotFound)
		return
	}

	proxy := p.getOrCreateProxy(route.TargetPort, route.Service, route.Worktree)
	proxy.ServeHTTP(w, r)
}

// AddRoute adds a route to the proxy
func (p *Proxy) AddRoute(domain string, targetPort int, service, worktree string) {
	p.router.Add(&Route{
		Domain:     domain,
		TargetPort: targetPort,
		Service:    service,
		Worktree:   worktree,
	})
}

// RemoveRoute removes a route from the proxy
func (p *Proxy) RemoveRoute(domain string) {
	route := p.router.Match(domain)
	if route != nil {
		// Remove cached proxy
		p.mu.Lock()
		delete(p.proxies, route.TargetPort)
		p.mu.Unlock()
	}
	p.router.Remove(domain)
}

// Routes returns all routes
func (p *Proxy) Routes() []*Route {
	return p.router.List()
}

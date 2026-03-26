package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

const (
	// ConfigFileName is the default config file name
	ConfigFileName = "wtm.yaml"
	// StateDir is the directory for state files
	StateDir = ".wtm"
)

var (
	ErrConfigNotFound = errors.New("config file not found")
	ErrInvalidConfig  = errors.New("invalid configuration")
)

// Load reads and parses the configuration file from the given directory
func Load(dir string) (*Config, error) {
	configPath := filepath.Join(dir, ConfigFileName)
	return LoadFile(configPath)
}

// LoadFile reads and parses a configuration file
func LoadFile(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("%w: %s", ErrConfigNotFound, path)
		}
		return nil, fmt.Errorf("reading config file: %w", err)
	}

	return Parse(data)
}

// Parse parses YAML data into a Config
func Parse(data []byte) (*Config, error) {
	cfg := Defaults()

	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("parsing config: %w", err)
	}

	if err := Validate(cfg); err != nil {
		return nil, err
	}

	return cfg, nil
}

// Validate checks the configuration for errors
func Validate(cfg *Config) error {
	if cfg.Project == "" {
		return fmt.Errorf("%w: project name is required", ErrInvalidConfig)
	}

	if cfg.PortRangeStart < 1024 {
		return fmt.Errorf("%w: port_range_start must be >= 1024", ErrInvalidConfig)
	}

	if cfg.ProxyPort < 1 || cfg.ProxyPort > 65535 {
		return fmt.Errorf("%w: proxy_port must be between 1 and 65535", ErrInvalidConfig)
	}

	if cfg.UIPort < 1 || cfg.UIPort > 65535 {
		return fmt.Errorf("%w: ui_port must be between 1 and 65535", ErrInvalidConfig)
	}

	// Validate services
	for name, svc := range cfg.Services {
		if svc.Run == "" {
			return fmt.Errorf("%w: service %q requires a run command", ErrInvalidConfig, name)
		}

		// Validate depends_on references
		for _, dep := range svc.DependsOn {
			if _, ok := cfg.Services[dep]; !ok {
				return fmt.Errorf("%w: service %q depends on unknown service %q", ErrInvalidConfig, name, dep)
			}
		}

		// Validate health check
		if svc.HealthCheck != nil {
			if err := validateHealthCheck(name, svc.HealthCheck); err != nil {
				return err
			}
		}
	}

	// Validate tasks
	for name, task := range cfg.Tasks {
		if task.Run == "" {
			return fmt.Errorf("%w: task %q requires a run command", ErrInvalidConfig, name)
		}
	}

	// Check for circular dependencies
	if err := checkCircularDeps(cfg.Services); err != nil {
		return err
	}

	return nil
}

func validateHealthCheck(serviceName string, hc *HealthCheck) error {
	switch hc.Type {
	case HealthCheckHTTP:
		if hc.URL == "" {
			return fmt.Errorf("%w: service %q http health check requires url", ErrInvalidConfig, serviceName)
		}
	case HealthCheckPort:
		// Port will be derived from service if not specified
	case HealthCheckLogMatch:
		if hc.Pattern == "" {
			return fmt.Errorf("%w: service %q log_match health check requires pattern", ErrInvalidConfig, serviceName)
		}
	case "":
		return fmt.Errorf("%w: service %q health check requires type", ErrInvalidConfig, serviceName)
	default:
		return fmt.Errorf("%w: service %q has unknown health check type %q", ErrInvalidConfig, serviceName, hc.Type)
	}
	return nil
}

func checkCircularDeps(services map[string]Service) error {
	visited := make(map[string]bool)
	recStack := make(map[string]bool)

	var visit func(name string) error
	visit = func(name string) error {
		visited[name] = true
		recStack[name] = true

		svc, ok := services[name]
		if !ok {
			return nil
		}

		for _, dep := range svc.DependsOn {
			if !visited[dep] {
				if err := visit(dep); err != nil {
					return err
				}
			} else if recStack[dep] {
				return fmt.Errorf("%w: circular dependency detected: %s -> %s", ErrInvalidConfig, name, dep)
			}
		}

		recStack[name] = false
		return nil
	}

	for name := range services {
		if !visited[name] {
			if err := visit(name); err != nil {
				return err
			}
		}
	}

	return nil
}

// FindConfigDir searches for a config file starting from dir and walking up
// It prioritizes directories with .wtm state folder and skips the wtm source directory
func FindConfigDir(startDir string) (string, error) {
	// First pass: look for directories with .wtm folder (actual project usage)
	dir := startDir
	for {
		// Skip wtm's own source directory
		if isWtmSourceDir(dir) {
			parent := filepath.Dir(dir)
			if parent == dir {
				break
			}
			dir = parent
			continue
		}

		// Check for .wtm state directory (strong indicator of project root)
		stateDir := filepath.Join(dir, StateDir)
		configPath := filepath.Join(dir, ConfigFileName)
		if _, err := os.Stat(stateDir); err == nil {
			if _, err := os.Stat(configPath); err == nil {
				return dir, nil
			}
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}

	// Second pass: fall back to just looking for wtm.yaml
	dir = startDir
	for {
		// Skip wtm's own source directory
		if isWtmSourceDir(dir) {
			parent := filepath.Dir(dir)
			if parent == dir {
				break
			}
			dir = parent
			continue
		}

		configPath := filepath.Join(dir, ConfigFileName)
		if _, err := os.Stat(configPath); err == nil {
			return dir, nil
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return "", fmt.Errorf("%w: searched from %s to root", ErrConfigNotFound, startDir)
}

// isWtmSourceDir checks if a directory is the wtm tool's own source directory
func isWtmSourceDir(dir string) bool {
	goModPath := filepath.Join(dir, "go.mod")
	data, err := os.ReadFile(goModPath)
	if err != nil {
		return false
	}
	// Check if this is the wtm source code directory
	content := string(data)
	return strings.Contains(content, "module github.com/jeroenfrenken/worktree-manager") ||
		strings.Contains(content, "module worktree-manager")
}

// EnsureStateDir creates the state directory if it doesn't exist
func EnsureStateDir(projectDir string) (string, error) {
	stateDir := filepath.Join(projectDir, StateDir)
	if err := os.MkdirAll(stateDir, 0755); err != nil {
		return "", fmt.Errorf("creating state directory: %w", err)
	}
	return stateDir, nil
}

package config

import "time"

// Config represents the root configuration for worktree-manager
type Config struct {
	Project        string             `yaml:"project"`
	Domain         string             `yaml:"domain"`
	ProxyPort      int                `yaml:"proxy_port"`
	UIPort         int                `yaml:"ui_port"`
	PortRangeStart int                `yaml:"port_range_start"`
	EnvFiles       []string           `yaml:"env_files"`
	Hooks          ProjectHooks       `yaml:"hooks"`
	Services       map[string]Service `yaml:"services"`
	Tasks          map[string]Task    `yaml:"tasks"`
	Terminal       TerminalConfig     `yaml:"terminal"`
}

// ProjectHooks defines hooks that run at project/worktree level
type ProjectHooks struct {
	OnCreate []string `yaml:"on_create"`
	OnDelete []string `yaml:"on_delete"`
}

// Service defines a long-running service process
type Service struct {
	Run         string            `yaml:"run"`
	WorkingDir  string            `yaml:"working_dir"`
	Env         map[string]string `yaml:"env"`
	PortOffset  int               `yaml:"port_offset"`
	NeedsPort   *bool             `yaml:"needs_port"`
	Hooks       ServiceHooks      `yaml:"hooks"`
	HealthCheck *HealthCheck      `yaml:"health_check"`
	DependsOn   []string          `yaml:"depends_on"`
}

// ServiceHooks defines lifecycle hooks for a service
type ServiceHooks struct {
	OnStart   []string     `yaml:"on_start"`
	OnStop    []string     `yaml:"on_stop"`
	OnRestart []string     `yaml:"on_restart"`
	OnCrash   *CrashConfig `yaml:"on_crash"`
}

// CrashConfig defines behavior when a service crashes
type CrashConfig struct {
	Commands     []string      `yaml:"commands"`
	AutoRestart  bool          `yaml:"auto_restart"`
	MaxRestarts  int           `yaml:"max_restarts"`
	RestartDelay time.Duration `yaml:"restart_delay"`
}

// HealthCheck defines how to check if a service is healthy
type HealthCheck struct {
	Type     HealthCheckType `yaml:"type"`
	URL      string          `yaml:"url"`
	Port     int             `yaml:"port"`
	Pattern  string          `yaml:"pattern"`
	Interval time.Duration   `yaml:"interval"`
	Timeout  time.Duration   `yaml:"timeout"`
}

// HealthCheckType represents the type of health check
type HealthCheckType string

const (
	HealthCheckHTTP     HealthCheckType = "http"
	HealthCheckPort     HealthCheckType = "port"
	HealthCheckLogMatch HealthCheckType = "log_match"
)

// Task defines a one-off command that can be run
type Task struct {
	Run         string `yaml:"run"`
	WorkingDir  string `yaml:"working_dir"`
	Description string `yaml:"description"`
}

// TerminalConfig defines terminal emulator settings
type TerminalConfig struct {
	Emulator string `yaml:"emulator"`
	Profile  string `yaml:"profile"`
}

// Defaults returns a Config with sensible default values
func Defaults() *Config {
	return &Config{
		Domain:         "local",
		ProxyPort:      8080,
		UIPort:         9000,
		PortRangeStart: 3001,
		EnvFiles:       []string{".env"},
		Services:       make(map[string]Service),
		Tasks:          make(map[string]Task),
	}
}

// NeedsPortValue returns whether the service needs a port (defaults to true)
func (s *Service) NeedsPortValue() bool {
	if s.NeedsPort == nil {
		return true
	}
	return *s.NeedsPort
}

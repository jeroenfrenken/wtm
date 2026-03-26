package config

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/joho/godotenv"
)

// EnvContext holds the context for variable substitution
type EnvContext struct {
	Port      int
	Worktree  string
	Project   string
	Domain    string
	ProxyPort int
}

var varPattern = regexp.MustCompile(`\$\{?([A-Za-z_][A-Za-z0-9_]*)\}?`)

// LoadEnvFiles loads environment variables from the specified files
func LoadEnvFiles(projectDir string, files []string) (map[string]string, error) {
	env := make(map[string]string)

	for _, file := range files {
		path := file
		if !filepath.IsAbs(path) {
			path = filepath.Join(projectDir, file)
		}

		fileEnv, err := godotenv.Read(path)
		if err != nil {
			if os.IsNotExist(err) {
				// Skip missing optional env files
				continue
			}
			return nil, fmt.Errorf("reading env file %s: %w", file, err)
		}

		// Merge into env (later files override earlier)
		for k, v := range fileEnv {
			env[k] = v
		}
	}

	return env, nil
}

// LoadWorktreeEnv loads worktree-specific environment overrides
func LoadWorktreeEnv(projectDir, worktree string) (map[string]string, error) {
	envPath := filepath.Join(projectDir, StateDir, "env", worktree+".env")
	env, err := godotenv.Read(envPath)
	if err != nil {
		if os.IsNotExist(err) {
			return make(map[string]string), nil
		}
		return nil, fmt.Errorf("reading worktree env: %w", err)
	}
	return env, nil
}

// SaveWorktreeEnv saves worktree-specific environment overrides
func SaveWorktreeEnv(projectDir, worktree string, env map[string]string) error {
	envDir := filepath.Join(projectDir, StateDir, "env")
	if err := os.MkdirAll(envDir, 0755); err != nil {
		return fmt.Errorf("creating env directory: %w", err)
	}

	envPath := filepath.Join(envDir, worktree+".env")
	return godotenv.Write(env, envPath)
}

// MergeEnv merges multiple environment maps, later maps override earlier
func MergeEnv(envMaps ...map[string]string) map[string]string {
	result := make(map[string]string)
	for _, env := range envMaps {
		for k, v := range env {
			result[k] = v
		}
	}
	return result
}

// SubstituteVars replaces variables in the format $VAR or ${VAR}
func SubstituteVars(s string, ctx EnvContext, env map[string]string) string {
	return varPattern.ReplaceAllStringFunc(s, func(match string) string {
		// Extract variable name (remove $ and optional braces)
		varName := strings.TrimPrefix(match, "$")
		varName = strings.TrimPrefix(varName, "{")
		varName = strings.TrimSuffix(varName, "}")

		// Check built-in variables first
		switch varName {
		case "PORT":
			return fmt.Sprintf("%d", ctx.Port)
		case "WORKTREE":
			return ctx.Worktree
		case "PROJECT":
			return ctx.Project
		case "DOMAIN":
			return ctx.Domain
		case "PROXY_PORT":
			return fmt.Sprintf("%d", ctx.ProxyPort)
		}

		// Check environment
		if val, ok := env[varName]; ok {
			return val
		}

		// Check system environment
		if val := os.Getenv(varName); val != "" {
			return val
		}

		// Return original if not found
		return match
	})
}

// SubstituteEnvMap applies variable substitution to all values in an env map
func SubstituteEnvMap(env map[string]string, ctx EnvContext) map[string]string {
	result := make(map[string]string, len(env))
	for k, v := range env {
		result[k] = SubstituteVars(v, ctx, env)
	}
	return result
}

// BuildServiceEnv builds the complete environment for a service
func BuildServiceEnv(cfg *Config, svc Service, ctx EnvContext, projectDir string) (map[string]string, error) {
	// Load base env files
	baseEnv, err := LoadEnvFiles(projectDir, cfg.EnvFiles)
	if err != nil {
		return nil, err
	}

	// Load worktree-specific env
	wtEnv, err := LoadWorktreeEnv(projectDir, ctx.Worktree)
	if err != nil {
		return nil, err
	}

	// Merge: base -> worktree -> service-specific
	env := MergeEnv(baseEnv, wtEnv, svc.Env)

	// Apply variable substitution
	env = SubstituteEnvMap(env, ctx)

	return env, nil
}

// ToOSEnv converts a map to the format used by os/exec (KEY=value)
func ToOSEnv(env map[string]string) []string {
	result := make([]string, 0, len(env))
	for k, v := range env {
		result = append(result, k+"="+v)
	}
	return result
}

// FromOSEnv converts os/exec format to a map
func FromOSEnv(osEnv []string) map[string]string {
	result := make(map[string]string)
	for _, e := range osEnv {
		parts := strings.SplitN(e, "=", 2)
		if len(parts) == 2 {
			result[parts[0]] = parts[1]
		}
	}
	return result
}

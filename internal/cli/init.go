package cli

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/fatih/color"
	"github.com/jeroenfrenken/worktree-manager/internal/config"
	"github.com/spf13/cobra"
)

var (
	initForce bool
)

var initCmd = &cobra.Command{
	Use:   "init [project-name]",
	Short: "Initialize a new wtm.yaml configuration",
	Long: `Initialize a new worktree-manager configuration in the current directory.

This creates a wtm.yaml file with sensible defaults and example services.
You can optionally specify the project name as an argument.`,
	Args: cobra.MaximumNArgs(1),
	RunE: runInit,
}

func init() {
	initCmd.Flags().BoolVarP(&initForce, "force", "f", false, "overwrite existing config file")
}

func runInit(cmd *cobra.Command, args []string) error {
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("getting current directory: %w", err)
	}

	configPath := filepath.Join(cwd, config.ConfigFileName)

	// Check if config already exists
	if _, err := os.Stat(configPath); err == nil && !initForce {
		return fmt.Errorf("config file already exists: %s (use --force to overwrite)", configPath)
	}

	// Determine project name
	projectName := filepath.Base(cwd)
	if len(args) > 0 {
		projectName = args[0]
	}

	// Generate config content
	configContent := generateDefaultConfig(projectName)

	// Write config file
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		return fmt.Errorf("writing config file: %w", err)
	}

	// Create state directory
	stateDir := filepath.Join(cwd, config.StateDir)
	if err := os.MkdirAll(stateDir, 0755); err != nil {
		return fmt.Errorf("creating state directory: %w", err)
	}

	// Create subdirectories
	for _, subdir := range []string{"env", "logs", "pids"} {
		if err := os.MkdirAll(filepath.Join(stateDir, subdir), 0755); err != nil {
			return fmt.Errorf("creating %s directory: %w", subdir, err)
		}
	}

	fmt.Println(color.GreenString("✓") + " Created " + configPath)
	fmt.Println(color.GreenString("✓") + " Created " + stateDir + "/")
	fmt.Println()
	fmt.Println("Next steps:")
	fmt.Println("  1. Edit wtm.yaml to configure your services")
	fmt.Println("  2. Run " + color.CyanString("wtm start") + " to start services")
	fmt.Println("  3. Run " + color.CyanString("wtm ui") + " to open the web UI")

	return nil
}

func generateDefaultConfig(projectName string) string {
	return fmt.Sprintf(`# Worktree Manager Configuration
# See docs/configuration.md for full reference

project: %s
domain: %s.local
proxy_port: 8080
ui_port: 9000
port_range_start: 3001

# Environment files to load (in order, later files override earlier)
env_files:
  - .env
  - .env.local

# Project-level hooks - run when worktrees are created/deleted
hooks:
  on_create:
    # - "npm install"
    # - "cp .env.example .env.local"
  on_delete:
    # - "echo 'Cleaning up...'"

# Services to run in each worktree
services:
  # Example API service
  # api:
  #   run: "npm run dev"
  #   working_dir: "./apps/api"
  #   env:
  #     PORT: "$PORT"
  #   port_offset: 0
  #   hooks:
  #     on_start:
  #       - "npx prisma generate"
  #     on_crash:
  #       auto_restart: true
  #       max_restarts: 3
  #       restart_delay: 2s
  #   health_check:
  #     type: http
  #     url: "http://localhost:$PORT/health"
  #     interval: 2s
  #     timeout: 30s

  # Example web frontend
  # web:
  #   run: "npm run dev"
  #   working_dir: "./apps/web"
  #   env:
  #     PORT: "$PORT"
  #   port_offset: 1
  #   depends_on:
  #     - api

# One-off tasks
tasks:
  # db-migrate:
  #   run: "npx prisma migrate dev"
  #   working_dir: "./apps/api"
  #   description: "Run database migrations"

  # test:
  #   run: "npm test"
  #   description: "Run all tests"

# Terminal emulator configuration (optional)
# terminal:
#   emulator: ghostty  # or: iterm, terminal, gnome-terminal, konsole
#   profile: Dev       # optional profile name
`, projectName, projectName)
}

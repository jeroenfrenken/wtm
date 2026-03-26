package cli

import (
	"fmt"
	"os"

	"github.com/fatih/color"
	"github.com/jeroenfrenken/worktree-manager/internal/config"
	"github.com/spf13/cobra"
)

var (
	// Version is set at build time
	Version = "dev"
	// Commit is set at build time
	Commit = "unknown"
)

var (
	// Flags
	configPath string
	verbose    bool
)

// rootCmd represents the base command
var rootCmd = &cobra.Command{
	Use:   "wtm",
	Short: "Worktree Manager - Manage development environments with git worktrees",
	Long: `Worktree Manager (wtm) helps you manage multiple development environments
using git worktrees. It provides:

  - Git worktree lifecycle management
  - Service process supervision with health checks
  - Reverse proxy for local domain routing
  - Web UI for monitoring and control
  - Terminal integration for interactive tasks

Get started with:
  wtm init     Create a new wtm.yaml configuration
  wtm create   Create a new worktree
  wtm start    Start services in the current worktree
  wtm ui       Open the web UI`,
	SilenceUsage:  true,
	SilenceErrors: true,
}

// Execute runs the root command
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, color.RedString("Error: %v", err))
		os.Exit(1)
	}
}

func init() {
	rootCmd.PersistentFlags().StringVarP(&configPath, "config", "c", "", "config file path (default: search for wtm.yaml)")
	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "verbose output")

	// Add subcommands
	rootCmd.AddCommand(versionCmd)
	rootCmd.AddCommand(initCmd)
}

// versionCmd shows version information
var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print version information",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Printf("wtm version %s (commit: %s)\n", Version, Commit)
	},
}

// findProjectDir finds the project directory using:
// 1. Explicit path argument (if provided)
// 2. WTM_PROJECT environment variable
// 3. Auto-detect by walking up from current directory
func findProjectDir(explicitPath string) (string, error) {
	var projectDir string

	if explicitPath != "" {
		projectDir = explicitPath
	} else if envProject := os.Getenv("WTM_PROJECT"); envProject != "" {
		projectDir = envProject
	} else {
		cwd, err := os.Getwd()
		if err != nil {
			return "", err
		}
		projectDir, err = config.FindConfigDir(cwd)
		if err != nil {
			fmt.Println(color.RedString("Error: Could not find a wtm project"))
			fmt.Println()
			fmt.Println("Make sure you're running from within a project directory that has wtm.yaml,")
			fmt.Println("or specify the project directory explicitly:")
			fmt.Println()
			fmt.Printf("  %s\n", color.CyanString("wtm <command> -C /path/to/your/project"))
			fmt.Println()
			fmt.Println("Or set the WTM_PROJECT environment variable:")
			fmt.Println()
			fmt.Printf("  %s\n", color.CyanString("export WTM_PROJECT=/path/to/your/project"))
			fmt.Println()
			return "", fmt.Errorf("not in a wtm project")
		}
	}

	// Verify the project directory has a wtm.yaml
	configPath := projectDir + "/" + config.ConfigFileName
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		return "", fmt.Errorf("no wtm.yaml found in %s", projectDir)
	}

	return projectDir, nil
}

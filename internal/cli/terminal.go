package cli

import (
	"fmt"
	"path/filepath"

	"github.com/fatih/color"
	"github.com/jeroenfrenken/worktree-manager/internal/config"
	"github.com/jeroenfrenken/worktree-manager/internal/terminal"
	"github.com/jeroenfrenken/worktree-manager/internal/worktree"
	"github.com/spf13/cobra"
)

var terminalCmd = &cobra.Command{
	Use:   "terminal [service]",
	Short: "Open a terminal in the worktree",
	Long: `Open a terminal window in the current worktree directory.

If a service name is provided, opens the terminal in that service's
working directory.

The terminal emulator is auto-detected or can be configured in wtm.yaml.`,
	Aliases: []string{"term", "shell"},
	RunE:    runTerminal,
}

func init() {
	rootCmd.AddCommand(terminalCmd)
}

func runTerminal(cmd *cobra.Command, args []string) error {
	// Find project directory
	projectDir, err := findProjectDir("")
	if err != nil {
		return err
	}

	// Load config
	cfg, err := config.Load(projectDir)
	if err != nil {
		return err
	}

	// Get current worktree
	mgr, err := worktree.NewManager(projectDir)
	if err != nil {
		return err
	}

	wt, err := mgr.Current()
	if err != nil {
		return err
	}

	// Determine working directory
	workDir := wt.Path

	// If a service is specified, use its working directory
	if len(args) > 0 {
		serviceName := args[0]
		svc, ok := cfg.Services[serviceName]
		if !ok {
			return fmt.Errorf("service %q not found", serviceName)
		}

		if svc.WorkingDir != "" {
			if filepath.IsAbs(svc.WorkingDir) {
				workDir = svc.WorkingDir
			} else {
				workDir = filepath.Join(wt.Path, svc.WorkingDir)
			}
		}
	}

	// Create terminal factory
	factory := terminal.NewFactory(cfg.Terminal.Emulator, cfg.Terminal.Profile)
	term, err := factory.Get()
	if err != nil {
		return fmt.Errorf("getting terminal: %w", err)
	}

	// Open terminal
	title := fmt.Sprintf("wtm: %s", wt.Name)
	if len(args) > 0 {
		title = fmt.Sprintf("wtm: %s/%s", wt.Name, args[0])
	}

	fmt.Printf("Opening %s in %s...\n", color.CyanString(term.Name()), workDir)

	err = term.OpenWindow(terminal.OpenOptions{
		Title:      title,
		WorkingDir: workDir,
	})

	if err != nil {
		return fmt.Errorf("opening terminal: %w", err)
	}

	return nil
}

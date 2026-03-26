package cli

import (
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/fatih/color"
	"github.com/jeroenfrenken/worktree-manager/internal/config"
	"github.com/jeroenfrenken/worktree-manager/internal/worktree"
	"github.com/spf13/cobra"
)

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show service status",
	Long:  `Show the status of all services in the current worktree.`,
	RunE:  runStatus,
}

func init() {
	rootCmd.AddCommand(statusCmd)
}

func runStatus(cmd *cobra.Command, args []string) error {
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

	// Print header
	fmt.Printf("Project: %s\n", color.CyanString(cfg.Project))
	fmt.Printf("Worktree: %s\n", color.CyanString(wt.Name))
	fmt.Printf("Base port: %d\n", wt.BasePort)
	fmt.Println()

	if len(cfg.Services) == 0 {
		fmt.Println("No services configured.")
		return nil
	}

	// Create table
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintf(w, "SERVICE\tPORT\tDOMAIN\tDEPENDS ON\n")

	for name, svc := range cfg.Services {
		port := wt.BasePort + svc.PortOffset
		if !svc.NeedsPortValue() {
			port = 0
		}

		domain := fmt.Sprintf("%s.%s.%s", name, wt.Name, cfg.Domain)
		if !svc.NeedsPortValue() {
			domain = "-"
		}

		deps := "-"
		if len(svc.DependsOn) > 0 {
			deps = fmt.Sprintf("%v", svc.DependsOn)
		}

		portStr := fmt.Sprintf("%d", port)
		if port == 0 {
			portStr = "-"
		}

		fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", name, portStr, domain, deps)
	}

	w.Flush()

	fmt.Println()
	fmt.Println("Run " + color.CyanString("wtm start") + " to start all services.")

	return nil
}

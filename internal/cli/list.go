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

var listCmd = &cobra.Command{
	Use:     "list",
	Aliases: []string{"ls"},
	Short:   "List all worktrees",
	Long:    `List all registered worktrees with their status.`,
	RunE:    runList,
}

func init() {
	rootCmd.AddCommand(listCmd)
}

func runList(cmd *cobra.Command, args []string) error {
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

	// Create worktree manager
	mgr, err := worktree.NewManager(projectDir)
	if err != nil {
		return err
	}

	// Get current worktree
	current, _ := mgr.Current()

	// Get all worktrees
	worktrees := mgr.List()

	if len(worktrees) == 0 {
		fmt.Println("No worktrees found.")
		fmt.Println()
		fmt.Println("Create one with:")
		fmt.Printf("  wtm create <name>\n")
		return nil
	}

	// Print header
	fmt.Printf("Project: %s\n", color.CyanString(cfg.Project))
	fmt.Printf("Domain: %s\n", cfg.Domain)
	fmt.Println()

	// Create tabwriter for aligned output
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)

	// Print header
	fmt.Fprintf(w, "  \tNAME\tBRANCH\tPORT\tPATH\n")

	// Add main worktree
	mainMarker := "○"
	if current != nil && current.Name == "main" {
		mainMarker = color.GreenString("●")
	}
	mainBranch := "main"
	if current != nil && current.Name == "main" {
		mainBranch = current.Branch
	}
	fmt.Fprintf(w, "%s\t%s\t%s\t%d\t%s\n",
		mainMarker,
		color.CyanString("main"),
		mainBranch,
		cfg.PortRangeStart,
		projectDir,
	)

	// Add other worktrees
	for _, wt := range worktrees {
		marker := "○"
		name := wt.Name
		if current != nil && current.Name == wt.Name {
			marker = color.GreenString("●")
			name = color.CyanString(wt.Name)
		}

		fmt.Fprintf(w, "%s\t%s\t%s\t%d\t%s\n",
			marker,
			name,
			wt.Branch,
			wt.BasePort,
			wt.Path,
		)
	}

	w.Flush()

	fmt.Println()
	fmt.Println(color.GreenString("●") + " = current worktree")

	return nil
}

package cli

import (
	"context"
	"fmt"

	"github.com/fatih/color"
	"github.com/jeroenfrenken/worktree-manager/internal/config"
	"github.com/jeroenfrenken/worktree-manager/internal/worktree"
	"github.com/spf13/cobra"
)

var (
	createBranch   string
	createNoHooks  bool
)

var createCmd = &cobra.Command{
	Use:   "create <name>",
	Short: "Create a new worktree",
	Long: `Create a new git worktree for development.

The worktree will be created as a sibling directory to your project root
in a .worktrees folder. For example:

  project/
  .worktrees/
    feature-auth/
    bugfix-login/

If no branch is specified, a new branch with the same name as the worktree
will be created.`,
	Args: cobra.ExactArgs(1),
	RunE: runCreate,
}

func init() {
	createCmd.Flags().StringVarP(&createBranch, "branch", "b", "", "branch name (default: worktree name)")
	createCmd.Flags().BoolVar(&createNoHooks, "no-hooks", false, "skip running on_create hooks")
	rootCmd.AddCommand(createCmd)
}

func runCreate(cmd *cobra.Command, args []string) error {
	name := args[0]
	ctx := context.Background()

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

	// Create the worktree
	fmt.Printf("Creating worktree %s...\n", color.CyanString(name))

	wt, err := mgr.Create(ctx, name, createBranch)
	if err != nil {
		return err
	}

	fmt.Println(color.GreenString("✓") + " Created worktree at " + wt.Path)
	fmt.Printf("  Branch: %s\n", wt.Branch)
	fmt.Printf("  Base port: %d\n", wt.BasePort)

	// Run on_create hooks
	if !createNoHooks && len(cfg.Hooks.OnCreate) > 0 {
		fmt.Println()
		hookRunner := worktree.NewHookRunner(cfg)
		hookRunner.WithEnv(map[string]string{
			"WORKTREE": wt.Name,
			"PROJECT":  cfg.Project,
			"PORT":     fmt.Sprintf("%d", wt.BasePort),
		})

		if err := hookRunner.RunOnCreate(ctx, wt); err != nil {
			fmt.Println(color.YellowString("⚠") + " Hook failed: " + err.Error())
			fmt.Println("  The worktree was created, but setup may be incomplete.")
		}
	}

	fmt.Println()
	fmt.Println("Next steps:")
	fmt.Printf("  cd %s\n", wt.Path)
	fmt.Printf("  wtm start\n")

	return nil
}

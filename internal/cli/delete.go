package cli

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/fatih/color"
	"github.com/jeroenfrenken/worktree-manager/internal/config"
	"github.com/jeroenfrenken/worktree-manager/internal/worktree"
	"github.com/spf13/cobra"
)

var (
	deleteForce   bool
	deleteNoHooks bool
	deleteYes     bool
)

var deleteCmd = &cobra.Command{
	Use:     "delete <name>",
	Aliases: []string{"rm", "remove"},
	Short:   "Delete a worktree",
	Long: `Delete a git worktree and clean up its resources.

This will:
1. Run on_delete hooks (if any)
2. Remove the git worktree
3. Clean up worktree state

Use --force to delete even if the worktree has uncommitted changes.`,
	Args: cobra.ExactArgs(1),
	RunE: runDelete,
}

func init() {
	deleteCmd.Flags().BoolVarP(&deleteForce, "force", "f", false, "force delete even with uncommitted changes")
	deleteCmd.Flags().BoolVar(&deleteNoHooks, "no-hooks", false, "skip running on_delete hooks")
	deleteCmd.Flags().BoolVarP(&deleteYes, "yes", "y", false, "skip confirmation prompt")
	rootCmd.AddCommand(deleteCmd)
}

func runDelete(cmd *cobra.Command, args []string) error {
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

	// Get the worktree
	wt, exists := mgr.Get(name)
	if !exists {
		return fmt.Errorf("worktree %q not found", name)
	}

	// Confirm deletion
	if !deleteYes {
		fmt.Printf("Delete worktree %s at %s? [y/N] ", color.CyanString(name), wt.Path)
		reader := bufio.NewReader(os.Stdin)
		response, _ := reader.ReadString('\n')
		response = strings.TrimSpace(strings.ToLower(response))
		if response != "y" && response != "yes" {
			fmt.Println("Cancelled.")
			return nil
		}
	}

	// Run on_delete hooks
	if !deleteNoHooks && len(cfg.Hooks.OnDelete) > 0 {
		hookRunner := worktree.NewHookRunner(cfg)
		hookRunner.WithEnv(map[string]string{
			"WORKTREE": wt.Name,
			"PROJECT":  cfg.Project,
		})

		if err := hookRunner.RunOnDelete(ctx, wt); err != nil {
			if !deleteForce {
				return fmt.Errorf("on_delete hook failed: %w (use --force to skip)", err)
			}
			fmt.Println(color.YellowString("⚠") + " Hook failed: " + err.Error())
		}
	}

	// Delete the worktree
	fmt.Printf("Deleting worktree %s...\n", color.CyanString(name))

	if err := mgr.Delete(ctx, name, deleteForce); err != nil {
		return err
	}

	fmt.Println(color.GreenString("✓") + " Deleted worktree " + name)

	return nil
}

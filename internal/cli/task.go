package cli

import (
	"context"
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/fatih/color"
	"github.com/jeroenfrenken/worktree-manager/internal/config"
	"github.com/jeroenfrenken/worktree-manager/internal/task"
	"github.com/jeroenfrenken/worktree-manager/internal/worktree"
	"github.com/spf13/cobra"
)

var taskCmd = &cobra.Command{
	Use:   "task <name>",
	Short: "Run a one-off task",
	Long: `Run a one-off task defined in wtm.yaml.

Tasks are useful for common operations like running migrations,
seeding databases, or running tests.

Without arguments, lists all available tasks.`,
	RunE: runTask,
}

func init() {
	rootCmd.AddCommand(taskCmd)
}

func runTask(cmd *cobra.Command, args []string) error {
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

	// Get current worktree
	mgr, err := worktree.NewManager(projectDir)
	if err != nil {
		return err
	}

	wt, err := mgr.Current()
	if err != nil {
		return err
	}

	// Create task runner
	runner := task.NewRunner(cfg, wt.Path, wt.Name, wt.BasePort)

	// If no args, list tasks
	if len(args) == 0 {
		return listTasks(runner)
	}

	// Run the task
	taskName := args[0]
	return runner.Run(ctx, taskName)
}

func listTasks(runner *task.Runner) error {
	tasks := runner.List()

	if len(tasks) == 0 {
		fmt.Println("No tasks defined.")
		fmt.Println()
		fmt.Println("Add tasks to your wtm.yaml:")
		fmt.Println()
		fmt.Println("  tasks:")
		fmt.Println("    test:")
		fmt.Println("      run: \"npm test\"")
		fmt.Println("      description: \"Run tests\"")
		return nil
	}

	fmt.Println("Available tasks:")
	fmt.Println()

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	for name, t := range tasks {
		desc := t.Description
		if desc == "" {
			desc = t.Run
		}
		fmt.Fprintf(w, "  %s\t%s\n", color.CyanString(name), desc)
	}
	w.Flush()

	fmt.Println()
	fmt.Println("Run a task with: " + color.CyanString("wtm task <name>"))

	return nil
}

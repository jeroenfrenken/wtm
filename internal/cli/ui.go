package cli

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"runtime"
	"syscall"
	"time"

	"github.com/fatih/color"
	"github.com/jeroenfrenken/worktree-manager/internal/config"
	"github.com/jeroenfrenken/worktree-manager/internal/ui"
	"github.com/jeroenfrenken/worktree-manager/internal/worktree"
	"github.com/spf13/cobra"
)

var (
	uiPort       int
	uiNoBrowser  bool
	uiWithStart  bool
	uiProjectDir string
)

var uiCmd = &cobra.Command{
	Use:   "ui",
	Short: "Start the web UI",
	Long: `Start the worktree-manager web UI.

The web UI provides:
- Service status dashboard
- Real-time log streaming
- Start/stop controls
- Worktree management

Use --with-start to also start all services.`,
	RunE: runUI,
}

func init() {
	uiCmd.Flags().IntVarP(&uiPort, "port", "p", 0, "UI port (default: from config or 9000)")
	uiCmd.Flags().BoolVar(&uiNoBrowser, "no-browser", false, "don't open browser")
	uiCmd.Flags().BoolVar(&uiWithStart, "with-start", false, "also start all services")
	uiCmd.Flags().StringVarP(&uiProjectDir, "project", "C", "", "project directory (default: auto-detect from current dir)")
	rootCmd.AddCommand(uiCmd)
}

func runUI(cmd *cobra.Command, args []string) error {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Handle interrupt
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// Find project directory
	projectDir, err := findProjectDir(uiProjectDir)
	if err != nil {
		return err
	}

	fmt.Printf("Project: %s\n", color.CyanString(projectDir))

	// Load config
	cfg, err := config.Load(projectDir)
	if err != nil {
		return err
	}

	// Determine port
	port := uiPort
	if port == 0 {
		port = cfg.UIPort
	}
	if port == 0 {
		port = 9000
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

	// Create UI server
	server, err := ui.NewServer(cfg, projectDir, port)
	if err != nil {
		return err
	}

	// Optionally start services
	if uiWithStart {
		fmt.Println("Starting services...")
		if err := server.StartWorktree(ctx, wt.Name); err != nil {
			return err
		}
	}

	// Start UI server in background
	go func() {
		if err := server.Start(); err != nil {
			fmt.Printf("UI server error: %v\n", err)
		}
	}()

	// Open browser
	url := fmt.Sprintf("http://127.0.0.1:%d", port)
	if !uiNoBrowser {
		openBrowser(url)
	}

	fmt.Println()
	fmt.Printf("Web UI: %s\n", color.CyanString(url))
	fmt.Println("Press Ctrl+C to stop")
	fmt.Println()

	// Wait for interrupt
	<-sigChan
	fmt.Println("\nShutting down... (press Ctrl+C again to force quit)")

	// Stop accepting new signals during shutdown (prevents double-quit issues)
	signal.Stop(sigChan)

	// Start a goroutine to handle force quit
	go func() {
		forceChan := make(chan os.Signal, 1)
		signal.Notify(forceChan, syscall.SIGINT, syscall.SIGTERM)
		<-forceChan
		fmt.Println("\nForce quitting...")
		os.Exit(1)
	}()

	// Stop UI server (this also stops all supervisors)
	stopCtx, stopCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer stopCancel()
	if err := server.Stop(stopCtx); err != nil {
		fmt.Printf("Warning during shutdown: %v\n", err)
	}

	fmt.Println("Shutdown complete.")
	return nil
}

func openBrowser(url string) {
	var cmd *exec.Cmd

	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", url)
	case "linux":
		cmd = exec.Command("xdg-open", url)
	default:
		return
	}

	cmd.Start()
}

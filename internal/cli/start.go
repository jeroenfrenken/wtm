package cli

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/fatih/color"
	"github.com/jeroenfrenken/worktree-manager/internal/config"
	"github.com/jeroenfrenken/worktree-manager/internal/dns"
	"github.com/jeroenfrenken/worktree-manager/internal/process"
	"github.com/jeroenfrenken/worktree-manager/internal/worktree"
	"github.com/spf13/cobra"
)

var startCmd = &cobra.Command{
	Use:   "start [service...]",
	Short: "Start services",
	Long: `Start all or specific services in the current worktree.

If no services are specified, all services will be started in dependency order.
Use Ctrl+C to stop all services.`,
	RunE: runStart,
}

func init() {
	rootCmd.AddCommand(startCmd)
}

func runStart(cmd *cobra.Command, args []string) error {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Handle interrupt
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

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

	// Sync hosts entries for this worktree
	domains := buildDomains(cfg, wt.Name)
	if len(domains) > 0 {
		hostsMgr := dns.NewHostsManager()
		if err := syncHostsForWorktree(hostsMgr, cfg, wt.Name, domains); err != nil {
			fmt.Printf("Warning: could not sync hosts: %v\n", err)
		}
	}

	// Create supervisor
	supervisor, err := process.NewSupervisor(cfg, wt.Name, wt.Path, wt.BasePort)
	if err != nil {
		return err
	}

	// Print startup info
	fmt.Printf("Project: %s\n", color.CyanString(cfg.Project))
	fmt.Printf("Worktree: %s\n", color.CyanString(wt.Name))
	fmt.Printf("Base port: %d\n", wt.BasePort)
	fmt.Println()

	// Start event handler
	go func() {
		for event := range supervisor.Events() {
			switch event.Type {
			case process.EventStarted:
				fmt.Printf("[%s] %s %s\n", color.CyanString(event.Service), color.GreenString("●"), event.Message)
			case process.EventStopped:
				fmt.Printf("[%s] %s %s\n", color.CyanString(event.Service), color.YellowString("○"), event.Message)
			case process.EventFailed:
				fmt.Printf("[%s] %s %s\n", color.CyanString(event.Service), color.RedString("✗"), event.Message)
			case process.EventHealthy:
				fmt.Printf("[%s] %s healthy\n", color.CyanString(event.Service), color.GreenString("♥"))
			case process.EventUnhealthy:
				fmt.Printf("[%s] %s %s\n", color.CyanString(event.Service), color.RedString("♥"), event.Message)
			case process.EventRestart:
				fmt.Printf("[%s] %s %s\n", color.CyanString(event.Service), color.YellowString("↻"), event.Message)
			}
		}
	}()

	// Start log streaming
	go func() {
		for log := range supervisor.Logs() {
			streamColor := color.WhiteString
			if log.Stream == "stderr" {
				streamColor = color.RedString
			}
			fmt.Printf("[%s] %s\n", color.CyanString(log.Service), streamColor(log.Message))
		}
	}()

	// Start services
	if len(args) > 0 {
		// Start specific services
		for _, name := range args {
			if err := supervisor.StartService(ctx, name); err != nil {
				return fmt.Errorf("starting %s: %w", name, err)
			}
		}
	} else {
		// Start all services
		if err := supervisor.Start(ctx); err != nil {
			return err
		}
	}

	fmt.Println()
	fmt.Println(color.GreenString("All services started.") + " Press Ctrl+C to stop.")
	fmt.Println()

	// Wait for interrupt
	<-sigChan
	fmt.Println()
	fmt.Println("Shutting down...")

	// Stop all services
	stopCtx, stopCancel := context.WithTimeout(context.Background(), 30)
	defer stopCancel()

	if err := supervisor.Stop(stopCtx); err != nil {
		return fmt.Errorf("stopping services: %w", err)
	}

	fmt.Println(color.GreenString("All services stopped."))

	return nil
}

// syncHostsForWorktree syncs hosts entries for the current worktree while preserving others
func syncHostsForWorktree(hostsMgr *dns.HostsManager, cfg *config.Config, worktreeName string, domains []string) error {
	// Get existing domains to preserve other worktrees
	existing, err := hostsMgr.List()
	if err != nil {
		return err
	}

	// Collect all domains (existing from other worktrees + new ones)
	allDomains := make([]string, 0)

	// Keep domains from other worktrees (those that don't match current worktree pattern)
	for _, entry := range existing {
		isCurrentWT := false
		for _, d := range domains {
			if entry.Domain == d {
				isCurrentWT = true
				break
			}
		}
		if !isCurrentWT {
			allDomains = append(allDomains, entry.Domain)
		}
	}

	// Add current worktree domains
	allDomains = append(allDomains, domains...)

	return hostsMgr.Sync(allDomains)
}

package cli

import (
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/fatih/color"
	"github.com/jeroenfrenken/worktree-manager/internal/config"
	"github.com/jeroenfrenken/worktree-manager/internal/dns"
	"github.com/jeroenfrenken/worktree-manager/internal/worktree"
	"github.com/spf13/cobra"
)

var hostsCmd = &cobra.Command{
	Use:   "hosts",
	Short: "Manage /etc/hosts entries",
	Long: `Manage /etc/hosts entries for worktree domains.

This command adds or removes entries for local development domains.
Requires sudo access to modify /etc/hosts.`,
}

var hostsSyncCmd = &cobra.Command{
	Use:   "sync",
	Short: "Sync hosts entries for current worktree",
	Long: `Sync /etc/hosts entries for the current worktree's services.

This adds entries like:
  127.0.0.1 api.feature-auth.myapp.local
  127.0.0.1 web.feature-auth.myapp.local`,
	RunE: runHostsSync,
}

var hostsListCmd = &cobra.Command{
	Use:   "list",
	Short: "List managed hosts entries",
	RunE:  runHostsList,
}

var hostsClearCmd = &cobra.Command{
	Use:   "clear",
	Short: "Remove all managed hosts entries",
	RunE:  runHostsClear,
}

func init() {
	hostsCmd.AddCommand(hostsSyncCmd)
	hostsCmd.AddCommand(hostsListCmd)
	hostsCmd.AddCommand(hostsClearCmd)
	rootCmd.AddCommand(hostsCmd)
}

func runHostsSync(cmd *cobra.Command, args []string) error {
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

	// Build domain list for current worktree
	domains := buildDomains(cfg, wt.Name)

	if len(domains) == 0 {
		fmt.Println("No services configured.")
		return nil
	}

	fmt.Printf("Syncing hosts for worktree: %s\n", color.CyanString(wt.Name))
	fmt.Println()

	// Get existing domains to preserve other worktrees
	hostsMgr := dns.NewHostsManager()
	existing, err := hostsMgr.List()
	if err != nil {
		return err
	}

	// Collect all domains (existing from other worktrees + new ones)
	allDomains := make([]string, 0)
	wtPrefix := fmt.Sprintf(".%s.%s", wt.Name, cfg.Domain)

	// Keep domains from other worktrees
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

	// Sync
	if err := hostsMgr.Sync(allDomains); err != nil {
		return fmt.Errorf("syncing hosts: %w", err)
	}

	fmt.Println("Added entries:")
	for _, d := range domains {
		fmt.Printf("  127.0.0.1 %s\n", color.GreenString(d))
	}
	fmt.Println()
	fmt.Println(color.GreenString("Hosts synced successfully."))

	_ = wtPrefix // suppress unused warning

	return nil
}

func runHostsList(cmd *cobra.Command, args []string) error {
	hostsMgr := dns.NewHostsManager()
	entries, err := hostsMgr.List()
	if err != nil {
		return err
	}

	if len(entries) == 0 {
		fmt.Println("No managed hosts entries.")
		return nil
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "IP\tDOMAIN")
	for _, entry := range entries {
		fmt.Fprintf(w, "%s\t%s\n", entry.IP, entry.Domain)
	}
	w.Flush()

	return nil
}

func runHostsClear(cmd *cobra.Command, args []string) error {
	fmt.Println("Removing all managed hosts entries...")

	hostsMgr := dns.NewHostsManager()
	if err := hostsMgr.Sync(nil); err != nil {
		return fmt.Errorf("clearing hosts: %w", err)
	}

	fmt.Println(color.GreenString("Hosts cleared successfully."))
	return nil
}

// buildDomains builds the list of domains for a worktree
func buildDomains(cfg *config.Config, worktreeName string) []string {
	domains := make([]string, 0, len(cfg.Services))

	for serviceName := range cfg.Services {
		domain := fmt.Sprintf("%s.%s.%s", serviceName, worktreeName, cfg.Domain)
		domains = append(domains, domain)
	}

	return domains
}

package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

var stopCmd = &cobra.Command{
	Use:   "stop [service...]",
	Short: "Stop services",
	Long: `Stop all or specific running services.

This command is primarily used when services are running in the background.
When using 'wtm start' in the foreground, use Ctrl+C to stop.`,
	RunE: runStop,
}

func init() {
	rootCmd.AddCommand(stopCmd)
}

func runStop(cmd *cobra.Command, args []string) error {
	// TODO: Implement background service management
	// For now, services are stopped via Ctrl+C in the foreground
	fmt.Println("Services are currently managed in the foreground.")
	fmt.Println("Use Ctrl+C to stop services started with 'wtm start'.")
	fmt.Println()
	fmt.Println("Background service management will be added in a future version.")
	return nil
}

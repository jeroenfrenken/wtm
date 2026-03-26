package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

var (
	logsFollow bool
	logsTail   int
)

var logsCmd = &cobra.Command{
	Use:   "logs [service]",
	Short: "View service logs",
	Long: `View logs from services.

When services are running in the foreground with 'wtm start',
logs are streamed directly to the terminal.

This command will be enhanced to view historical logs in a future version.`,
	RunE: runLogs,
}

func init() {
	logsCmd.Flags().BoolVarP(&logsFollow, "follow", "f", false, "follow log output")
	logsCmd.Flags().IntVarP(&logsTail, "tail", "n", 100, "number of lines to show")
	rootCmd.AddCommand(logsCmd)
}

func runLogs(cmd *cobra.Command, args []string) error {
	fmt.Println("Log viewing is integrated with 'wtm start'.")
	fmt.Println("When you run 'wtm start', logs are streamed directly to the terminal.")
	fmt.Println()
	fmt.Println("Historical log storage will be added in a future version.")
	return nil
}

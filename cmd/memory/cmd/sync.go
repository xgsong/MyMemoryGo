// Package cmd contains the CLI commands for the memory application.
package cmd

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"
)

// syncCmd represents the sync command.
var syncCmd = &cobra.Command{
	Use:   "sync",
	Short: "Synchronize file index with database",
	Long: `Synchronize the search index with the file system.

This command reads all markdown files in the workspace and updates
the search index to ensure consistency.

Useful after:
- Manual file edits outside of the memory CLI
- Restoring from backup
- Database corruption recovery`,
	RunE: runSync,
}

func init() {
	rootCmd.AddCommand(syncCmd)
}

func runSync(cmd *cobra.Command, args []string) error {
	ctx := context.Background()

	// Initialize app
	app, err := InitializeApp(ctx)
	if err != nil {
		return fmt.Errorf("failed to initialize: %w", err)
	}
	defer app.Cleanup()

	fmt.Println("Synchronizing files with database...")
	fmt.Println("This may take a while for large workspaces.")

	// Perform sync
	if err := app.MemoryApp.SyncIndex(ctx); err != nil {
		return fmt.Errorf("failed to sync index: %w", err)
	}

	fmt.Println("✅ Synchronization completed successfully")

	return nil
}

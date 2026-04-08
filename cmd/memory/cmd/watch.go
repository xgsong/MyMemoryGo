// Package cmd contains the CLI commands for the memory application.
package cmd

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/xgsong/MyMemoryGo/internal/domain/repository"
)

// watchCmd represents the watch command.
var watchCmd = &cobra.Command{
	Use:   "watch",
	Short: "Watch for file changes and auto-sync",
	Long: `Watch the workspace directory for file changes and automatically
synchronize the search index.

The watcher monitors:
- MEMORY.md (long-term memory)
- memory/*.md (daily logs)
- session/*.md (session memories)

Press Ctrl+C to stop watching.`,
	RunE: runWatch,
}

func init() {
	rootCmd.AddCommand(watchCmd)
}

func runWatch(cmd *cobra.Command, args []string) error {
	ctx := context.Background()

	// Initialize app
	app, err := InitializeApp(ctx)
	if err != nil {
		return fmt.Errorf("failed to initialize: %w", err)
	}
	defer app.Cleanup()

	workspaceDir := viper.GetString("workspace.dir")

	fmt.Printf("Watching workspace: %s\n", workspaceDir)
	fmt.Println("Press Ctrl+C to stop...")

	// Start file watcher
	ctx = WaitForInterrupt(ctx)

	handler := func(event repository.FileChangeEvent) {
		fmt.Printf("File changed: %s (%s), syncing...\n", event.Path, event.Operation)
		if err := app.MemoryApp.SyncIndex(ctx); err != nil {
			fmt.Printf("Warning: sync failed: %v\n", err)
		}
	}

	if err := app.FileMgr.Watch(ctx, workspaceDir, handler); err != nil {
		return fmt.Errorf("failed to start watcher: %w", err)
	}

	fmt.Println("\nStopped watching.")

	return nil
}

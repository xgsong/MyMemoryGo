// Package cmd contains the CLI commands for the memory application.
package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/xgsong/MyMemoryGo/internal/application/service"
	"github.com/xgsong/MyMemoryGo/internal/domain/entity"
	"github.com/xgsong/MyMemoryGo/internal/pkg/errors"
)

// storeCmd represents the store command.
var storeCmd = &cobra.Command{
	Use:   "store <content>",
	Short: "Store a new memory",
	Long: `Store a new memory with automatic embedding generation and indexing.

The memory will be stored in the appropriate file based on the source type:
- longterm: MEMORY.md (curated long-term memories)
- daily: memory/YYYY-MM-DD.md (daily logs)
- session: session/YYYYMMDD-HHMMSS.md (session-specific memories)`,
	Args: cobra.MinimumNArgs(1),
	RunE: runStore,
}

var (
	storePath   string
	storeSource string
	storeTags   string
)

func init() {
	rootCmd.AddCommand(storeCmd)

	storeCmd.Flags().StringVarP(&storePath, "path", "p", "", "file path (auto-determined if not specified)")
	storeCmd.Flags().StringVarP(&storeSource, "source", "s", "daily", "source type (longterm, daily, session)")
	storeCmd.Flags().StringVarP(&storeTags, "tags", "t", "", "comma-separated tags")
}

func runStore(cmd *cobra.Command, args []string) error {
	ctx := context.Background()

	app, err := InitializeApp(ctx)
	if err != nil {
		return errors.WrapOp(errors.CodeInternal, "runStore", "failed to initialize", err)
	}
	defer app.Cleanup()

	content := args[0]
	if len(args) > 1 {
		for _, arg := range args[1:] {
			content += " " + arg
		}
	}

	var source entity.SourceType
	switch storeSource {
	case "longterm":
		source = entity.SourceLongTerm
	case "daily":
		source = entity.SourceDaily
	case "session":
		source = entity.SourceSession
	default:
		return errors.New(errors.CodeInvalidInput, fmt.Sprintf("invalid source type: %s (must be longterm, daily, or session)", storeSource))
	}

	// Prepare metadata
	metadata := make(map[string]string)
	if storeTags != "" {
		metadata["tags"] = storeTags
	}

	req := &service.StoreMemoryRequest{
		Content:  content,
		Path:     storePath,
		Source:   source,
		Metadata: metadata,
	}

	resp, err := app.MemoryApp.StoreMemory(ctx, req)
	if err != nil {
		return errors.WrapOp(errors.CodeInternal, "runStore", "failed to store memory", err)
	}

	// Output result
	output := viper.GetString("output")
	if output == "json" {
		result := map[string]string{
			"id":         resp.Memory.ID,
			"path":       resp.Memory.Path,
			"source":     string(resp.Memory.Source),
			"created_at": resp.Memory.CreatedAt.Format(time.RFC3339),
		}
		data, err := json.Marshal(result)
		if err != nil {
			return errors.WrapOp(errors.CodeInternal, "runStore", "failed to marshal result", err)
		}
		fmt.Println(string(data))
	} else {
		fmt.Printf("✅ Memory stored successfully\n")
		fmt.Printf("   ID: %s\n", resp.Memory.ID)
		fmt.Printf("   Path: %s\n", resp.Memory.Path)
		fmt.Printf("   Source: %s\n", resp.Memory.Source)
		fmt.Printf("   Created: %s\n", resp.Memory.CreatedAt.Format("2006-01-02 15:04:05"))
	}

	return nil
}

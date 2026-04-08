// Package cmd contains the CLI commands for the memory application.
package cmd

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/xgsong/MyMemoryGo/internal/application/service"
	"github.com/xgsong/MyMemoryGo/internal/domain/entity"
)

// searchCmd represents the search command.
var searchCmd = &cobra.Command{
	Use:   "search <query>",
	Short: "Search memories",
	Long: `Search memories using simplified text matching.

Examples:
  memory search "project architecture"
  memory search "meeting notes" --limit 5
  memory search "important" --source longterm`,
	Args: cobra.MinimumNArgs(1),
	RunE: runSearch,
}

var (
	searchLimit    int
	searchSource   string
	searchMinScore float64
)

func init() {
	rootCmd.AddCommand(searchCmd)

	searchCmd.Flags().IntVarP(&searchLimit, "limit", "l", 10, "maximum number of results")
	searchCmd.Flags().StringVarP(&searchSource, "source", "s", "", "filter by source type (longterm, daily, session)")
	searchCmd.Flags().Float64Var(&searchMinScore, "min-score", 0.0, "minimum relevance score")
}

func runSearch(cmd *cobra.Command, args []string) error {
	ctx := context.Background()

	// Initialize app
	app, err := InitializeApp(ctx)
	if err != nil {
		return fmt.Errorf("failed to initialize: %w", err)
	}
	defer app.Cleanup()

	// Get query from args
	query := args[0]
	if len(args) > 1 {
		for _, arg := range args[1:] {
			query += " " + arg
		}
	}

	// Prepare search request
	req := &service.SearchMemoriesRequest{
		Query:    query,
		Limit:    searchLimit,
		MinScore: searchMinScore,
	}

	// Add source filter if specified
	if searchSource != "" {
		var source entity.SourceType
		switch searchSource {
		case "longterm":
			source = entity.SourceLongTerm
		case "daily":
			source = entity.SourceDaily
		case "session":
			source = entity.SourceSession
		default:
			return fmt.Errorf("invalid source type: %s", searchSource)
		}
		req.SourceFilter = []entity.SourceType{source}
	}

	// Perform search
	resp, err := app.MemoryApp.SearchMemories(ctx, req)
	if err != nil {
		return fmt.Errorf("failed to search memories: %w", err)
	}

	// Output results
	output := viper.GetString("output")
	if output == "json" {
		data, _ := json.MarshalIndent(resp.Results, "", "  ")
		fmt.Println(string(data))
	} else {
		fmt.Printf("Found %d results for query: %s\n\n", len(resp.Results.Hits), query)
		for i, hit := range resp.Results.Hits {
			fmt.Printf("%d. [%s] %s\n", i+1, hit.Source, hit.Path)
			fmt.Printf("   Score: %.2f\n", hit.Score)
			fmt.Printf("   Snippet: %s\n", truncate(hit.Snippet, 100))
			fmt.Printf("   ID: %s\n\n", hit.ID)
		}
	}

	return nil
}

// truncate truncates a string to maxLen characters.
func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

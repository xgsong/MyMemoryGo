// Package cmd contains the CLI commands for the memory application.
package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/xgsong/MyMemoryGo/internal/pkg/errors"
)

// initCmd represents the init command.
var initCmd = &cobra.Command{
	Use:   "init [workspace]",
	Short: "Initialize a new memory workspace",
	Long: `Initialize a new memory workspace with the required directory structure.

This creates:
- Workspace directory
- MEMORY.md file (long-term memory)
- memory/ directory (daily logs)
- session/ directory (session memories)
- SQLite database
- Configuration file`,
	Args: cobra.MaximumNArgs(1),
	RunE: runInit,
}

func init() {
	rootCmd.AddCommand(initCmd)
}

func runInit(cmd *cobra.Command, args []string) error {
	// Determine workspace directory
	workspaceDir := viper.GetString("workspace.dir")
	if len(args) > 0 {
		workspaceDir = args[0]
	}

	verbose := viper.GetBool("verbose")

	if verbose {
		fmt.Printf("Initializing workspace at: %s\n", workspaceDir)
	}

	// Create directory structure
	dirs := []string{
		workspaceDir,
		filepath.Join(workspaceDir, "memory"),
		filepath.Join(workspaceDir, "session"),
		filepath.Join(workspaceDir, "logs"),
	}

	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return errors.WrapOp(errors.CodeFilesystem, "runInit", "failed to create directory", err)
		}
		if verbose {
			fmt.Printf("  Created: %s\n", dir)
		}
	}

	// Create default MEMORY.md file
	memoryFile := filepath.Join(workspaceDir, "MEMORY.md")
	if _, err := os.Stat(memoryFile); os.IsNotExist(err) {
		content := `# Long-term Memory

This file contains curated, long-term memories that persist across sessions.

## Structure

Organize your memories using markdown headings:

### Project Information
- Key facts about your project
- Technical decisions
- Architecture notes

### User Preferences
- Language preferences
- Response styles
- Custom configurations

### Knowledge Base
- Domain knowledge
- Best practices
- Reference materials
`
		if err := os.WriteFile(memoryFile, []byte(content), 0644); err != nil {
			return errors.WrapOp(errors.CodeFilesystem, "runInit", "failed to create MEMORY.md", err)
		}
		if verbose {
			fmt.Printf("  Created: %s\n", memoryFile)
		}
	}

	configFile := filepath.Join(workspaceDir, "..", "config.yaml")
	if _, err := os.Stat(configFile); os.IsNotExist(err) {
		configContent := `# Memory Configuration

workspace:
  dir: ` + workspaceDir + `

embedding:
  preset: ollama-nomic
  base_url: http://localhost:11434/v1
  model: nomic-embed-text
  dimensions: 768

storage:
  db_path: ` + filepath.Join(workspaceDir, "..", "memory.db") + `
  wal_mode: true

search:
  vector_weight: 0.7
  fulltext_weight: 0.3
  default_limit: 10
  min_score: 0.5
  mmr:
    enabled: true
    lambda: 0.7
  decay:
    enabled: true
    half_life: 720h

logging:
  level: info
  format: json
`
		if err := os.WriteFile(configFile, []byte(configContent), 0644); err != nil {
			return errors.WrapOp(errors.CodeFilesystem, "runInit", "failed to create config.yaml", err)
		}
		if verbose {
			fmt.Printf("  Created: %s\n", configFile)
		}
	}

	fmt.Printf("✅ Workspace initialized at: %s\n", workspaceDir)
	fmt.Println()
	fmt.Println("Next steps:")
	fmt.Println("  1. Start Ollama: ollama serve")
	fmt.Println("  2. Pull embedding model: ollama pull nomic-embed-text")
	fmt.Println("  3. Store a memory: memory store \"Your content here\"")
	fmt.Println("  4. Search memories: memory search \"query\"")

	return nil
}

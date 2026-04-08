// Package cmd contains the CLI commands for the memory application.
package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	// Used for flags.
	cfgFile string

	// rootCmd represents the base command when called without any subcommands.
	rootCmd = &cobra.Command{
		Use:   "memory",
		Short: "A persistent memory management system for AI applications",
		Long: `Memory is a production-ready, OpenAI-compatible persistent memory component.

It provides:
- File-first architecture (Markdown as source of truth)
- Hybrid search (vector + full-text)
- Zero external dependencies (SQLite + file system)
- Multiple memory sources (long-term, daily, session)`,
	}
)

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() error {
	return rootCmd.Execute()
}

func init() {
	cobra.OnInitialize(initConfig)

	// Global flags
	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is $HOME/.memory/config.yaml)")
	rootCmd.PersistentFlags().String("workspace", "", "workspace directory (default is $HOME/.memory/workspace)")
	rootCmd.PersistentFlags().String("output", "text", "output format (text, json)")
	rootCmd.PersistentFlags().Bool("verbose", false, "verbose output")

	// Bind flags to viper
	viper.BindPFlag("workspace.dir", rootCmd.PersistentFlags().Lookup("workspace"))
	viper.BindPFlag("output", rootCmd.PersistentFlags().Lookup("output"))
	viper.BindPFlag("verbose", rootCmd.PersistentFlags().Lookup("verbose"))
}

// initConfig reads in config file and ENV variables if set.
func initConfig() {
	if cfgFile != "" {
		// Use config file from the flag.
		viper.SetConfigFile(cfgFile)
	} else {
		// Find home directory.
		home, err := os.UserHomeDir()
		cobra.CheckErr(err)

		// Search config in home directory with name ".memory" (without extension).
		viper.AddConfigPath(filepath.Join(home, ".memory"))
		viper.AddConfigPath(".")
		viper.SetConfigType("yaml")
		viper.SetConfigName("config")
	}

	// Set environment variable prefix
	viper.SetEnvPrefix("MEMORY")
	viper.AutomaticEnv() // read in environment variables that match

	// Set defaults
	viper.SetDefault("workspace.dir", filepath.Join(os.Getenv("HOME"), ".memory", "workspace"))
	viper.SetDefault("output", "text")
	viper.SetDefault("verbose", false)
	viper.SetDefault("storage.db_path", filepath.Join(os.Getenv("HOME"), ".memory", "memory.db"))
	viper.SetDefault("storage.wal_mode", true)
	viper.SetDefault("embedding.preset", "ollama-nomic")
	viper.SetDefault("embedding.base_url", "http://localhost:11434/v1")
	viper.SetDefault("embedding.model", "nomic-embed-text")
	viper.SetDefault("embedding.dimensions", 768)
	viper.SetDefault("search.vector_weight", 0.7)
	viper.SetDefault("search.fulltext_weight", 0.3)
	viper.SetDefault("search.default_limit", 10)
	viper.SetDefault("search.min_score", 0.5)
	viper.SetDefault("search.mmr.enabled", true)
	viper.SetDefault("search.mmr.lambda", 0.7)
	viper.SetDefault("search.decay.enabled", true)
	viper.SetDefault("search.decay.half_life", "720h")
	viper.SetDefault("api.enabled", false)
	viper.SetDefault("api.host", "0.0.0.0")
	viper.SetDefault("api.port", 8080)
	viper.SetDefault("logging.level", "info")
	viper.SetDefault("logging.format", "json")

	// If a config file is found, read it in.
	if err := viper.ReadInConfig(); err == nil {
		if viper.GetBool("verbose") {
			fmt.Fprintln(os.Stderr, "Using config file:", viper.ConfigFileUsed())
		}
	}
}

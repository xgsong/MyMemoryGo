// Package cmd contains the CLI commands for the memory application.
package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

// versionCmd represents the version command.
var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print the version number",
	Long:  `Print the version number of the memory CLI.`,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("Memory CLI v1.0.0")
		fmt.Println("Go Memory Component - Production-ready persistent memory for AI applications")
	},
}

func init() {
	rootCmd.AddCommand(versionCmd)
}

// Package main is the entry point for the memory CLI application.
package main

import (
	"os"

	"github.com/xgsong/MyMemoryGo/cmd/memory/cmd"
)

func main() {
	if err := cmd.Execute(); err != nil {
		os.Exit(1)
	}
}

// Package service defines domain services that orchestrate business logic.
package service

import (
	"strings"

	"github.com/xgsong/MyMemoryGo/internal/domain/entity"
)

// NormalizePath normalizes a file path to ensure consistency.
// It removes leading/trailing slashes and converts backslashes to forward slashes.
func NormalizePath(path string) string {
	path = strings.TrimSpace(path)
	path = strings.TrimPrefix(path, "/")
	path = strings.TrimSuffix(path, "/")
	path = strings.ReplaceAll(path, "\\", "/")
	return path
}

// DetermineSourceType determines the source type based on the file path.
// This follows OpenClaw's convention:
// - MEMORY.md -> longterm
// - memory/YYYY-MM-DD.md -> daily
// - others -> session
func DetermineSourceType(path string) entity.SourceType {
	path = NormalizePath(path)

	// Check for long-term memory
	if path == "MEMORY.md" || path == "memory.md" {
		return entity.SourceLongTerm
	}

	// Check for daily log pattern: memory/YYYY-MM-DD.md
	if strings.HasPrefix(path, "memory/") && strings.HasSuffix(path, ".md") {
		// Extract filename (between "memory/" and ".md")
		if len(path) < 11 { // "memory/" (7) + "YYYY-MM-DD.md" (11) = 18 total, but we check minimum length
			return entity.SourceSession
		}
		filename := path[7 : len(path)-3] // Remove "memory/" prefix and ".md" suffix
		// Date format: YYYY-MM-DD (10 characters)
		if len(filename) == 10 && filename[4] == '-' && filename[7] == '-' {
			return entity.SourceDaily
		}
	}

	return entity.SourceSession
}

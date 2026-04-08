// Package service defines domain services that orchestrate business logic.
package service

import (
	"fmt"
	"strings"
)

// GenerateID generates a unique ID for a memory entry.
// The ID is based on path and line numbers to ensure consistency.
func GenerateID(path string, startLine, endLine int) string {
	normalizedPath := NormalizePath(path)
	return fmt.Sprintf("%s:%d-%d", normalizedPath, startLine, endLine)
}

// ParseID parses a memory ID into its components.
// Returns path, startLine, endLine, or an error if parsing fails.
func ParseID(id string) (path string, startLine, endLine int, err error) {
	parts := strings.Split(id, ":")
	if len(parts) != 2 {
		return "", 0, 0, fmt.Errorf("invalid ID format: %s", id)
	}

	path = parts[0]
	lineRange := parts[1]

	lineParts := strings.Split(lineRange, "-")
	if len(lineParts) != 2 {
		return "", 0, 0, fmt.Errorf("invalid line range: %s", lineRange)
	}

	_, err = fmt.Sscanf(lineParts[0], "%d", &startLine)
	if err != nil {
		return "", 0, 0, fmt.Errorf("invalid start line: %s", lineParts[0])
	}

	_, err = fmt.Sscanf(lineParts[1], "%d", &endLine)
	if err != nil {
		return "", 0, 0, fmt.Errorf("invalid end line: %s", lineParts[1])
	}

	return path, startLine, endLine, nil
}

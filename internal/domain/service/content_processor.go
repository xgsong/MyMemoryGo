// Package service defines domain services that orchestrate business logic.
package service

import (
	"strings"

	"github.com/xgsong/MyMemoryGo/internal/domain/entity"
)

// MergeMemories combines multiple memories into a single content string.
// This is useful for creating summaries or consolidated views.
func MergeMemories(memories []*entity.Memory, separator string) string {
	if len(memories) == 0 {
		return ""
	}

	var builder strings.Builder
	for i, memory := range memories {
		if i > 0 {
			builder.WriteString(separator)
		}
		builder.WriteString(memory.Content)
	}

	return builder.String()
}

// SplitContentIntoChunks splits content into chunks of approximately equal size.
// This is used for indexing large documents.
func SplitContentIntoChunks(content string, maxChunkSize int) []string {
	if len(content) <= maxChunkSize {
		return []string{content}
	}

	var chunks []string
	lines := strings.Split(content, "\n")

	var currentChunk strings.Builder
	currentSize := 0

	for _, line := range lines {
		lineSize := len(line) + 1 // +1 for newline

		if currentSize+lineSize > maxChunkSize && currentSize > 0 {
			// Save current chunk
			chunks = append(chunks, strings.TrimSpace(currentChunk.String()))
			currentChunk.Reset()
			currentSize = 0
		}

		if currentSize > 0 {
			currentChunk.WriteString("\n")
		}
		currentChunk.WriteString(line)
		currentSize += lineSize
	}

	// Add the last chunk
	if currentSize > 0 {
		chunks = append(chunks, strings.TrimSpace(currentChunk.String()))
	}

	return chunks
}

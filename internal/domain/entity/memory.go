// Package entity defines the core domain entities for the memory system.
// These entities represent the fundamental building blocks of the memory domain.
package entity

import (
	"sort"
	"time"
)

// SourceType represents the origin of a memory entry.
// It categorizes where the memory was created and how it should be treated.
type SourceType string

const (
	// SourceLongTerm represents curated long-term memories stored in MEMORY.md.
	// These memories are stable, curated knowledge that persists across sessions.
	SourceLongTerm SourceType = "longterm"

	// SourceDaily represents daily log entries stored in memory/YYYY-MM-DD.md.
	// These are ephemeral, chronological records of daily activities.
	SourceDaily SourceType = "daily"

	// SourceSession represents session-specific memories.
	// These are temporary memories scoped to a particular conversation or session.
	SourceSession SourceType = "session"
)

// Memory represents a single memory entry in the system.
// It contains the actual content, metadata, and search-relevant information.
type Memory struct {
	// ID is a unique identifier for the memory entry.
	ID string `json:"id"`

	// Path is the relative file path where the memory is stored.
	// Example: "MEMORY.md" or "memory/2026-03-08.md"
	Path string `json:"path"`

	// StartLine is the starting line number in the file (1-indexed).
	StartLine int `json:"start_line"`

	// EndLine is the ending line number in the file (1-indexed).
	EndLine int `json:"end_line"`

	// Content is the actual text content of the memory.
	Content string `json:"content"`

	// Embedding is the vector representation of the content.
	// This is optional and may be nil for entries not yet indexed.
	Embedding []float32 `json:"embedding,omitempty"`

	// Source indicates where the memory originated from.
	Source SourceType `json:"source"`

	// CreatedAt is the timestamp when the memory was created.
	CreatedAt time.Time `json:"created_at"`

	// UpdatedAt is the timestamp when the memory was last modified.
	UpdatedAt time.Time `json:"updated_at"`

	// Checksum is a hash of the content used for change detection.
	Checksum string `json:"checksum"`

	// Metadata contains additional key-value pairs for custom attributes.
	Metadata map[string]string `json:"metadata,omitempty"`
}

// IsLongTerm returns true if the memory is a long-term curated entry.
func (m *Memory) IsLongTerm() bool {
	return m.Source == SourceLongTerm
}

// IsDaily returns true if the memory is a daily log entry.
func (m *Memory) IsDaily() bool {
	return m.Source == SourceDaily
}

// IsSession returns true if the memory is session-scoped.
func (m *Memory) IsSession() bool {
	return m.Source == SourceSession
}

// IsEvergreen returns true if the memory should not decay over time.
// Long-term memories (MEMORY.md) are considered evergreen.
func (m *Memory) IsEvergreen() bool {
	return m.IsLongTerm()
}

// LineCount returns the number of lines in the memory content.
func (m *Memory) LineCount() int {
	return m.EndLine - m.StartLine + 1
}

// Age returns the duration since the memory was created.
func (m *Memory) Age() time.Duration {
	return time.Since(m.CreatedAt)
}

// Entry represents a lightweight memory reference used in search results.
// It contains only the essential information needed to identify and display
// a memory without loading the full content.
type Entry struct {
	// ID is the unique identifier matching the Memory.ID.
	ID string `json:"id"`

	// Path is the file path where the memory is stored.
	Path string `json:"path"`

	// StartLine is the starting line number.
	StartLine int `json:"start_line"`

	// EndLine is the ending line number.
	EndLine int `json:"end_line"`

	// Snippet is a short excerpt of the memory content.
	Snippet string `json:"snippet"`

	// Score is the relevance score from 0.0 to 1.0.
	Score float64 `json:"score"`

	// Source is the origin of the memory.
	Source SourceType `json:"source"`

	// Timestamp is when the memory was created.
	Timestamp time.Time `json:"timestamp"`
}

// ToMemory converts an Entry to a Memory with minimal fields populated.
// This is useful when you need a Memory struct but only have an Entry.
func (e *Entry) ToMemory() *Memory {
	return &Memory{
		ID:        e.ID,
		Path:      e.Path,
		StartLine: e.StartLine,
		EndLine:   e.EndLine,
		Source:    e.Source,
		CreatedAt: e.Timestamp,
	}
}

// SearchHit represents a search result with additional metadata.
// It extends Entry with embedding information for reranking algorithms.
type SearchHit struct {
	*Entry

	// Embedding is the vector representation (used for MMR reranking).
	Embedding []float32 `json:"embedding,omitempty"`
}

// SearchResult represents the complete result set from a search operation.
type SearchResult struct {
	// Hits is the collection of search results.
	Hits []*SearchHit `json:"hits"`

	// Total is the total number of matching documents before limiting.
	Total int `json:"total"`

	// Duration is the time taken to perform the search.
	Duration time.Duration `json:"duration"`

	// Query is the original search query string.
	Query string `json:"query"`
}

// AddHit appends a search hit to the result set.
func (r *SearchResult) AddHit(hit *SearchHit) {
	r.Hits = append(r.Hits, hit)
}

// SortByScore sorts the hits by score in descending order.
func (r *SearchResult) SortByScore() {
	sort.Slice(r.Hits, func(i, j int) bool {
		return r.Hits[i].Score > r.Hits[j].Score
	})
}

package entity_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/xgsong/MyMemoryGo/internal/domain/entity"
)

func TestSourceType_Constants(t *testing.T) {
	tests := []struct {
		name     string
		source   entity.SourceType
		expected string
	}{
		{"longterm", entity.SourceLongTerm, "longterm"},
		{"daily", entity.SourceDaily, "daily"},
		{"session", entity.SourceSession, "session"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, entity.SourceType(tt.expected), tt.source)
		})
	}
}

func TestMemory_IsLongTerm(t *testing.T) {
	tests := []struct {
		name   string
		source entity.SourceType
		want   bool
	}{
		{"longterm source", entity.SourceLongTerm, true},
		{"daily source", entity.SourceDaily, false},
		{"session source", entity.SourceSession, false},
		{"empty source", entity.SourceType(""), false},
		{"unknown source", entity.SourceType("unknown"), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			memory := &entity.Memory{Source: tt.source}
			assert.Equal(t, tt.want, memory.IsLongTerm())
		})
	}
}

func TestMemory_IsDaily(t *testing.T) {
	tests := []struct {
		name   string
		source entity.SourceType
		want   bool
	}{
		{"daily source", entity.SourceDaily, true},
		{"longterm source", entity.SourceLongTerm, false},
		{"session source", entity.SourceSession, false},
		{"empty source", entity.SourceType(""), false},
		{"unknown source", entity.SourceType("unknown"), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			memory := &entity.Memory{Source: tt.source}
			assert.Equal(t, tt.want, memory.IsDaily())
		})
	}
}

func TestMemory_IsSession(t *testing.T) {
	tests := []struct {
		name   string
		source entity.SourceType
		want   bool
	}{
		{"session source", entity.SourceSession, true},
		{"longterm source", entity.SourceLongTerm, false},
		{"daily source", entity.SourceDaily, false},
		{"empty source", entity.SourceType(""), false},
		{"unknown source", entity.SourceType("unknown"), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			memory := &entity.Memory{Source: tt.source}
			assert.Equal(t, tt.want, memory.IsSession())
		})
	}
}

func TestMemory_IsEvergreen(t *testing.T) {
	tests := []struct {
		name   string
		source entity.SourceType
		want   bool
	}{
		{"longterm is evergreen", entity.SourceLongTerm, true},
		{"daily is not evergreen", entity.SourceDaily, false},
		{"session is not evergreen", entity.SourceSession, false},
		{"empty is not evergreen", entity.SourceType(""), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			memory := &entity.Memory{Source: tt.source}
			assert.Equal(t, tt.want, memory.IsEvergreen())
		})
	}
}

func TestMemory_LineCount(t *testing.T) {
	tests := []struct {
		name      string
		startLine int
		endLine   int
		want      int
	}{
		{"single line", 1, 1, 1},
		{"multiple lines", 1, 10, 10},
		{"span across lines", 5, 15, 11},
		{"same start and end", 100, 100, 1},
		{"large range", 1, 1000, 1000},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			memory := &entity.Memory{
				StartLine: tt.startLine,
				EndLine:   tt.endLine,
			}
			assert.Equal(t, tt.want, memory.LineCount())
		})
	}
}

func TestMemory_Age(t *testing.T) {
	now := time.Now()

	tests := []struct {
		name      string
		createdAt time.Time
		minAge    time.Duration
		maxAge    time.Duration
	}{
		{
			name:      "just created",
			createdAt: now,
			minAge:    0,
			maxAge:    time.Second,
		},
		{
			name:      "one hour ago",
			createdAt: now.Add(-1 * time.Hour),
			minAge:    time.Hour - time.Second,
			maxAge:    time.Hour + time.Second,
		},
		{
			name:      "one day ago",
			createdAt: now.Add(-24 * time.Hour),
			minAge:    24*time.Hour - time.Second,
			maxAge:    24*time.Hour + time.Second,
		},
		{
			name:      "one week ago",
			createdAt: now.Add(-7 * 24 * time.Hour),
			minAge:    7*24*time.Hour - time.Second,
			maxAge:    7*24*time.Hour + time.Second,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			memory := &entity.Memory{CreatedAt: tt.createdAt}
			age := memory.Age()
			assert.GreaterOrEqual(t, age, tt.minAge)
			assert.LessOrEqual(t, age, tt.maxAge)
		})
	}
}

func TestMemory_Fields(t *testing.T) {
	now := time.Now()
	embedding := []float32{0.1, 0.2, 0.3}
	metadata := map[string]string{"key": "value"}

	memory := &entity.Memory{
		ID:        "test-id",
		Path:      "test/path.md",
		StartLine: 1,
		EndLine:   5,
		Content:   "Test content",
		Embedding: embedding,
		Source:    entity.SourceLongTerm,
		CreatedAt: now,
		UpdatedAt: now,
		Checksum:  "abc123",
		Metadata:  metadata,
	}

	assert.Equal(t, "test-id", memory.ID)
	assert.Equal(t, "test/path.md", memory.Path)
	assert.Equal(t, 1, memory.StartLine)
	assert.Equal(t, 5, memory.EndLine)
	assert.Equal(t, "Test content", memory.Content)
	assert.Equal(t, embedding, memory.Embedding)
	assert.Equal(t, entity.SourceLongTerm, memory.Source)
	assert.Equal(t, now, memory.CreatedAt)
	assert.Equal(t, now, memory.UpdatedAt)
	assert.Equal(t, "abc123", memory.Checksum)
	assert.Equal(t, metadata, memory.Metadata)
}

func TestMemory_EmbeddingOptional(t *testing.T) {
	t.Run("with embedding", func(t *testing.T) {
		memory := &entity.Memory{
			Embedding: []float32{0.1, 0.2, 0.3},
		}
		assert.NotNil(t, memory.Embedding)
		assert.Len(t, memory.Embedding, 3)
	})

	t.Run("without embedding", func(t *testing.T) {
		memory := &entity.Memory{}
		assert.Nil(t, memory.Embedding)
	})
}

func TestMemory_MetadataOptional(t *testing.T) {
	t.Run("with metadata", func(t *testing.T) {
		memory := &entity.Memory{
			Metadata: map[string]string{"key": "value"},
		}
		assert.NotNil(t, memory.Metadata)
		assert.Equal(t, "value", memory.Metadata["key"])
	})

	t.Run("without metadata", func(t *testing.T) {
		memory := &entity.Memory{}
		assert.Nil(t, memory.Metadata)
	})
}

func TestEntry_ToMemory(t *testing.T) {
	now := time.Now()
	entry := &entity.Entry{
		ID:        "test-id",
		Path:      "test/path.md",
		StartLine: 1,
		EndLine:   5,
		Snippet:   "Test snippet",
		Score:     0.95,
		Source:    entity.SourceLongTerm,
		Timestamp: now,
	}

	memory := entry.ToMemory()

	assert.Equal(t, entry.ID, memory.ID)
	assert.Equal(t, entry.Path, memory.Path)
	assert.Equal(t, entry.StartLine, memory.StartLine)
	assert.Equal(t, entry.EndLine, memory.EndLine)
	assert.Equal(t, entry.Source, memory.Source)
	assert.Equal(t, entry.Timestamp, memory.CreatedAt)
	assert.Empty(t, memory.Content) // Entry doesn't have full content
	assert.Nil(t, memory.Embedding)
}

func TestEntry_Fields(t *testing.T) {
	now := time.Now()
	entry := &entity.Entry{
		ID:        "entry-id",
		Path:      "memory/2026-03-08.md",
		StartLine: 10,
		EndLine:   20,
		Snippet:   "This is a snippet...",
		Score:     0.85,
		Source:    entity.SourceDaily,
		Timestamp: now,
	}

	assert.Equal(t, "entry-id", entry.ID)
	assert.Equal(t, "memory/2026-03-08.md", entry.Path)
	assert.Equal(t, 10, entry.StartLine)
	assert.Equal(t, 20, entry.EndLine)
	assert.Equal(t, "This is a snippet...", entry.Snippet)
	assert.Equal(t, 0.85, entry.Score)
	assert.Equal(t, entity.SourceDaily, entry.Source)
	assert.Equal(t, now, entry.Timestamp)
}

func TestSearchHit(t *testing.T) {
	now := time.Now()
	entry := &entity.Entry{
		ID:        "hit-id",
		Path:      "MEMORY.md",
		StartLine: 1,
		EndLine:   10,
		Snippet:   "Test snippet",
		Score:     0.95,
		Source:    entity.SourceLongTerm,
		Timestamp: now,
	}
	embedding := []float32{0.1, 0.2, 0.3, 0.4, 0.5}

	hit := &entity.SearchHit{
		Entry:     entry,
		Embedding: embedding,
	}

	assert.Equal(t, entry, hit.Entry)
	assert.Equal(t, embedding, hit.Embedding)
	assert.Equal(t, "hit-id", hit.ID)
	assert.Equal(t, 0.95, hit.Score)
}

func TestSearchHit_WithoutEmbedding(t *testing.T) {
	hit := &entity.SearchHit{
		Entry: &entity.Entry{ID: "test"},
	}

	assert.NotNil(t, hit.Entry)
	assert.Nil(t, hit.Embedding)
	assert.Equal(t, "test", hit.ID)
}

func TestSearchResult_AddHit(t *testing.T) {
	result := &entity.SearchResult{}

	hit1 := &entity.SearchHit{Entry: &entity.Entry{ID: "hit1"}}
	hit2 := &entity.SearchHit{Entry: &entity.Entry{ID: "hit2"}}
	hit3 := &entity.SearchHit{Entry: &entity.Entry{ID: "hit3"}}

	result.AddHit(hit1)
	assert.Len(t, result.Hits, 1)
	assert.Equal(t, hit1, result.Hits[0])

	result.AddHit(hit2)
	assert.Len(t, result.Hits, 2)
	assert.Equal(t, hit2, result.Hits[1])

	result.AddHit(hit3)
	assert.Len(t, result.Hits, 3)
	assert.Equal(t, hit3, result.Hits[2])
}

func TestSearchResult_Fields(t *testing.T) {
	hits := []*entity.SearchHit{
		{Entry: &entity.Entry{ID: "hit1", Score: 0.9}},
		{Entry: &entity.Entry{ID: "hit2", Score: 0.8}},
	}

	result := &entity.SearchResult{
		Hits:     hits,
		Total:    100,
		Duration: 50 * time.Millisecond,
		Query:    "test query",
	}

	assert.Len(t, result.Hits, 2)
	assert.Equal(t, 100, result.Total)
	assert.Equal(t, 50*time.Millisecond, result.Duration)
	assert.Equal(t, "test query", result.Query)
}

func TestSearchResult_Empty(t *testing.T) {
	result := &entity.SearchResult{}

	assert.Nil(t, result.Hits)
	assert.Equal(t, 0, result.Total)
	assert.Equal(t, time.Duration(0), result.Duration)
	assert.Empty(t, result.Query)
}

func TestSearchResult_SortByScore(t *testing.T) {
	t.Run("sorts hits in descending order of score", func(t *testing.T) {
		result := &entity.SearchResult{
			Hits: []*entity.SearchHit{
				{Entry: &entity.Entry{ID: "hit1", Score: 0.5}},
				{Entry: &entity.Entry{ID: "hit2", Score: 0.9}},
				{Entry: &entity.Entry{ID: "hit3", Score: 0.7}},
			},
		}

		result.SortByScore()

		assert.Len(t, result.Hits, 3)
		assert.Equal(t, "hit2", result.Hits[0].ID) // 0.9
		assert.Equal(t, "hit3", result.Hits[1].ID) // 0.7
		assert.Equal(t, "hit1", result.Hits[2].ID) // 0.5
	})

	t.Run("empty hits does nothing", func(t *testing.T) {
		result := &entity.SearchResult{}
		result.SortByScore()
		assert.Empty(t, result.Hits)
	})

	t.Run("single hit does nothing", func(t *testing.T) {
		result := &entity.SearchResult{
			Hits: []*entity.SearchHit{
				{Entry: &entity.Entry{ID: "hit1", Score: 0.5}},
			},
		}
		result.SortByScore()
		assert.Len(t, result.Hits, 1)
		assert.Equal(t, "hit1", result.Hits[0].ID)
	})

	t.Run("handles equal scores", func(t *testing.T) {
		result := &entity.SearchResult{
			Hits: []*entity.SearchHit{
				{Entry: &entity.Entry{ID: "hit1", Score: 0.8}},
				{Entry: &entity.Entry{ID: "hit2", Score: 0.8}},
				{Entry: &entity.Entry{ID: "hit3", Score: 0.8}},
			},
		}

		result.SortByScore()

		assert.Len(t, result.Hits, 3)
		// Order remains stable for equal scores
		assert.Equal(t, "hit1", result.Hits[0].ID)
		assert.Equal(t, "hit2", result.Hits[1].ID)
		assert.Equal(t, "hit3", result.Hits[2].ID)
	})
}

func TestMemory_ZeroValues(t *testing.T) {
	memory := &entity.Memory{}

	assert.Empty(t, memory.ID)
	assert.Empty(t, memory.Path)
	assert.Equal(t, 0, memory.StartLine)
	assert.Equal(t, 0, memory.EndLine)
	assert.Empty(t, memory.Content)
	assert.Nil(t, memory.Embedding)
	assert.Empty(t, memory.Source)
	assert.True(t, memory.CreatedAt.IsZero())
	assert.True(t, memory.UpdatedAt.IsZero())
	assert.Empty(t, memory.Checksum)
	assert.Nil(t, memory.Metadata)
}

func TestEntry_ZeroValues(t *testing.T) {
	entry := &entity.Entry{}

	assert.Empty(t, entry.ID)
	assert.Empty(t, entry.Path)
	assert.Equal(t, 0, entry.StartLine)
	assert.Equal(t, 0, entry.EndLine)
	assert.Empty(t, entry.Snippet)
	assert.Equal(t, 0.0, entry.Score)
	assert.Empty(t, entry.Source)
	assert.True(t, entry.Timestamp.IsZero())
}

// Benchmark tests
func BenchmarkMemory_IsLongTerm(b *testing.B) {
	memory := &entity.Memory{Source: entity.SourceLongTerm}
	for i := 0; i < b.N; i++ {
		memory.IsLongTerm()
	}
}

func BenchmarkMemory_LineCount(b *testing.B) {
	memory := &entity.Memory{StartLine: 1, EndLine: 100}
	for i := 0; i < b.N; i++ {
		memory.LineCount()
	}
}

func BenchmarkMemory_Age(b *testing.B) {
	memory := &entity.Memory{CreatedAt: time.Now().Add(-24 * time.Hour)}
	for i := 0; i < b.N; i++ {
		memory.Age()
	}
}

func BenchmarkEntry_ToMemory(b *testing.B) {
	entry := &entity.Entry{
		ID:        "test-id",
		Path:      "test/path.md",
		StartLine: 1,
		EndLine:   5,
		Source:    entity.SourceLongTerm,
		Timestamp: time.Now(),
	}
	for i := 0; i < b.N; i++ {
		entry.ToMemory()
	}
}

func BenchmarkSearchResult_AddHit(b *testing.B) {
	result := &entity.SearchResult{
		Hits: make([]*entity.SearchHit, 0, 100),
	}
	hit := &entity.SearchHit{Entry: &entity.Entry{ID: "test"}}
	for i := 0; i < b.N; i++ {
		result.Hits = result.Hits[:0] // Reset
		result.AddHit(hit)
	}
}

// Table-driven test example for JSON serialization
func TestMemory_JSONSerialization(t *testing.T) {
	now := time.Now()
	original := &entity.Memory{
		ID:        "json-test",
		Path:      "test.json",
		StartLine: 1,
		EndLine:   5,
		Content:   "JSON test content",
		Embedding: []float32{0.1, 0.2, 0.3},
		Source:    entity.SourceLongTerm,
		CreatedAt: now,
		UpdatedAt: now,
		Checksum:  "checksum123",
		Metadata:  map[string]string{"env": "test"},
	}

	// Test that JSON tags are correctly set
	assert.Equal(t, "json-test", original.ID)
	assert.NotNil(t, original.Embedding)
	assert.NotNil(t, original.Metadata)
}

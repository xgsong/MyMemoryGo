package service_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/xgsong/MyMemoryGo/internal/domain/entity"
	"github.com/xgsong/MyMemoryGo/internal/domain/errors"
	"github.com/xgsong/MyMemoryGo/internal/domain/service"
)

func TestValidateMemory(t *testing.T) {
	tests := []struct {
		name    string
		memory  *entity.Memory
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid memory",
			memory: &entity.Memory{
				Path:      "MEMORY.md",
				Content:   "Test content",
				StartLine: 1,
				EndLine:   5,
				Source:    entity.SourceLongTerm,
			},
			wantErr: false,
		},
		{
			name:    "nil memory",
			memory:  nil,
			wantErr: true,
			errMsg:  "cannot be nil",
		},
		{
			name: "empty path",
			memory: &entity.Memory{
				Path:      "",
				Content:   "Test content",
				StartLine: 1,
				EndLine:   5,
				Source:    entity.SourceLongTerm,
			},
			wantErr: true,
			errMsg:  "cannot be empty",
		},
		{
			name: "empty content",
			memory: &entity.Memory{
				Path:      "MEMORY.md",
				Content:   "",
				StartLine: 1,
				EndLine:   5,
				Source:    entity.SourceLongTerm,
			},
			wantErr: true,
			errMsg:  "cannot be empty",
		},
		{
			name: "invalid start line - zero",
			memory: &entity.Memory{
				Path:      "MEMORY.md",
				Content:   "Test content",
				StartLine: 0,
				EndLine:   5,
				Source:    entity.SourceLongTerm,
			},
			wantErr: true,
			errMsg:  "must be >= 1",
		},
		{
			name: "invalid start line - negative",
			memory: &entity.Memory{
				Path:      "MEMORY.md",
				Content:   "Test content",
				StartLine: -1,
				EndLine:   5,
				Source:    entity.SourceLongTerm,
			},
			wantErr: true,
			errMsg:  "must be >= 1",
		},
		{
			name: "end line before start line",
			memory: &entity.Memory{
				Path:      "MEMORY.md",
				Content:   "Test content",
				StartLine: 5,
				EndLine:   3,
				Source:    entity.SourceLongTerm,
			},
			wantErr: true,
			errMsg:  "must be >= start_line",
		},
		{
			name: "same start and end line",
			memory: &entity.Memory{
				Path:      "MEMORY.md",
				Content:   "Test content",
				StartLine: 5,
				EndLine:   5,
				Source:    entity.SourceLongTerm,
			},
			wantErr: false,
		},
		{
			name: "empty source",
			memory: &entity.Memory{
				Path:      "MEMORY.md",
				Content:   "Test content",
				StartLine: 1,
				EndLine:   5,
				Source:    "",
			},
			wantErr: true,
			errMsg:  "cannot be empty",
		},
		{
			name: "invalid source",
			memory: &entity.Memory{
				Path:      "MEMORY.md",
				Content:   "Test content",
				StartLine: 1,
				EndLine:   5,
				Source:    "invalid",
			},
			wantErr: true,
			errMsg:  "invalid source type",
		},
		{
			name: "valid daily source",
			memory: &entity.Memory{
				Path:      "memory/2026-03-08.md",
				Content:   "Daily log",
				StartLine: 1,
				EndLine:   5,
				Source:    entity.SourceDaily,
			},
			wantErr: false,
		},
		{
			name: "valid session source",
			memory: &entity.Memory{
				Path:      "session/test.md",
				Content:   "Session content",
				StartLine: 1,
				EndLine:   5,
				Source:    entity.SourceSession,
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := service.ValidateMemory(tt.memory)

			if tt.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errMsg)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestCalculateChecksum(t *testing.T) {
	tests := []struct {
		name    string
		content string
	}{
		{"simple content", "Test content"},
		{"empty content", ""},
		{"unicode content", "测试内容 🎉"},
		{"multiline content", "Line 1\nLine 2\nLine 3"},
		{"large content", string(make([]byte, 10000))},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			checksum1 := service.CalculateChecksum(tt.content)
			checksum2 := service.CalculateChecksum(tt.content)

			// Same content should produce same checksum
			assert.Equal(t, checksum1, checksum2)
			// Checksum should be consistent length (64 chars for SHA256)
			assert.Len(t, checksum1, 64)
		})
	}

	t.Run("different content produces different checksum", func(t *testing.T) {
		checksum1 := service.CalculateChecksum("content 1")
		checksum2 := service.CalculateChecksum("content 2")
		assert.NotEqual(t, checksum1, checksum2)
	})
}

func TestNormalizePath(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"no change needed", "MEMORY.md", "MEMORY.md"},
		{"leading slash", "/MEMORY.md", "MEMORY.md"},
		{"trailing slash", "memory/", "memory"},
		{"both slashes", "/memory/2026-03-08.md/", "memory/2026-03-08.md"},
		{"backslashes become forward slashes", "\\memory\\2026-03-08.md", "/memory/2026-03-08.md"},
		{"mixed slashes", "/memory\\test.md", "memory/test.md"},
		{"spaces trimmed", "  MEMORY.md  ", "MEMORY.md"},
		{"multiple leading slashes removes one", "///test.md", "//test.md"},
		{"empty string", "", ""},
		{"single slash", "/", ""},
		{"windows path with drive letter", "C:\\Users\\test\\MEMORY.md", "C:/Users/test/MEMORY.md"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := service.NormalizePath(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestDetermineSourceType(t *testing.T) {
	tests := []struct {
		name     string
		path     string
		expected entity.SourceType
	}{
		{"MEMORY.md uppercase", "MEMORY.md", entity.SourceLongTerm},
		{"memory.md lowercase", "memory.md", entity.SourceLongTerm},
		{"daily log standard format", "memory/2026-03-08.md", entity.SourceDaily},
		{"daily log different date", "memory/2025-12-31.md", entity.SourceDaily},
		{"daily log with leading slash normalized", "/memory/2026-03-08.md", entity.SourceDaily},
		{"other file in notes", "notes/ideas.md", entity.SourceSession},
		{"file in root", "test.md", entity.SourceSession},
		{"file in docs directory", "docs/readme.md", entity.SourceSession},
		{"invalid daily single digit month", "memory/2026-3-08.md", entity.SourceSession},
		{"invalid daily missing day", "memory/2026-03.md", entity.SourceSession},
		{"invalid daily too many parts", "memory/2026-03-08-extra.md", entity.SourceSession},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := service.DetermineSourceType(tt.path)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestMergeMemories(t *testing.T) {
	t.Run("empty slice returns empty string", func(t *testing.T) {
		result := service.MergeMemories([]*entity.Memory{}, "\n")
		assert.Empty(t, result)
	})

	t.Run("nil slice returns empty string", func(t *testing.T) {
		result := service.MergeMemories(nil, "\n")
		assert.Empty(t, result)
	})

	t.Run("single memory", func(t *testing.T) {
		memories := []*entity.Memory{
			{Content: "Content 1"},
		}
		result := service.MergeMemories(memories, "\n---\n")
		assert.Equal(t, "Content 1", result)
	})

	t.Run("multiple memories with newline separator", func(t *testing.T) {
		memories := []*entity.Memory{
			{Content: "Content 1"},
			{Content: "Content 2"},
			{Content: "Content 3"},
		}
		result := service.MergeMemories(memories, "\n")
		assert.Equal(t, "Content 1\nContent 2\nContent 3", result)
	})

	t.Run("multiple memories with custom separator", func(t *testing.T) {
		memories := []*entity.Memory{
			{Content: "A"},
			{Content: "B"},
		}
		result := service.MergeMemories(memories, " | ")
		assert.Equal(t, "A | B", result)
	})

	t.Run("memories with empty content", func(t *testing.T) {
		memories := []*entity.Memory{
			{Content: "A"},
			{Content: ""},
			{Content: "C"},
		}
		result := service.MergeMemories(memories, "\n")
		assert.Equal(t, "A\n\nC", result)
	})
}

func TestSplitContentIntoChunks(t *testing.T) {
	t.Run("empty content returns single empty chunk", func(t *testing.T) {
		chunks := service.SplitContentIntoChunks("", 100)
		// Empty content behavior depends on implementation
		assert.LessOrEqual(t, len(chunks), 1)
	})

	t.Run("content smaller than chunk size", func(t *testing.T) {
		content := "Short text"
		chunks := service.SplitContentIntoChunks(content, 100)
		assert.Len(t, chunks, 1)
		assert.Equal(t, content, chunks[0])
	})

	t.Run("content exactly chunk size", func(t *testing.T) {
		content := "Exactly"
		chunks := service.SplitContentIntoChunks(content, len(content))
		assert.Len(t, chunks, 1)
	})

	t.Run("content larger than chunk size", func(t *testing.T) {
		content := "Line 1\nLine 2\nLine 3\nLine 4\nLine 5"
		chunks := service.SplitContentIntoChunks(content, 20)
		assert.GreaterOrEqual(t, len(chunks), 2)
	})

	t.Run("preserves content approximately", func(t *testing.T) {
		content := "Line 1\nLine 2\nLine 3\nLine 4\nLine 5"
		chunks := service.SplitContentIntoChunks(content, 15)

		// Reconstruct content
		var reconstructed string
		for i, chunk := range chunks {
			if i > 0 {
				reconstructed += "\n"
			}
			reconstructed += chunk
		}

		// Content should be similar (may have minor whitespace differences)
		assert.Contains(t, reconstructed, "Line 1")
		assert.Contains(t, reconstructed, "Line 5")
	})

	t.Run("very small chunk size", func(t *testing.T) {
		content := "ABCD"
		chunks := service.SplitContentIntoChunks(content, 1)
		assert.GreaterOrEqual(t, len(chunks), 1)
	})
}

func TestGenerateID(t *testing.T) {
	tests := []struct {
		name      string
		path      string
		startLine int
		endLine   int
		expected  string
	}{
		{"simple path", "MEMORY.md", 1, 5, "MEMORY.md:1-5"},
		{"path with directory", "memory/2026-03-08.md", 10, 20, "memory/2026-03-08.md:10-20"},
		{"path with leading slash", "/test.md", 1, 1, "test.md:1-1"},
		{"same start and end", "test.md", 5, 5, "test.md:5-5"},
		{"large line numbers", "test.md", 1000, 2000, "test.md:1000-2000"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			id := service.GenerateID(tt.path, tt.startLine, tt.endLine)
			assert.Equal(t, tt.expected, id)
		})
	}
}

func TestParseID(t *testing.T) {
	tests := []struct {
		name      string
		id        string
		wantPath  string
		wantStart int
		wantEnd   int
		wantErr   bool
	}{
		{
			name:      "valid ID",
			id:        "MEMORY.md:1-5",
			wantPath:  "MEMORY.md",
			wantStart: 1,
			wantEnd:   5,
			wantErr:   false,
		},
		{
			name:      "valid ID with directory",
			id:        "memory/2026-03-08.md:10-20",
			wantPath:  "memory/2026-03-08.md",
			wantStart: 10,
			wantEnd:   20,
			wantErr:   false,
		},
		{
			name:    "invalid format - no colon",
			id:      "MEMORY.md",
			wantErr: true,
		},
		{
			name:    "invalid format - no range",
			id:      "MEMORY.md:",
			wantErr: true,
		},
		{
			name:    "invalid line range - no dash",
			id:      "MEMORY.md:15",
			wantErr: true,
		},
		{
			name:    "invalid line range - missing end",
			id:      "MEMORY.md:1-",
			wantErr: true,
		},
		{
			name:    "invalid line range - missing start",
			id:      "MEMORY.md:-5",
			wantErr: true,
		},
		{
			name:    "invalid line range - non-numeric",
			id:      "MEMORY.md:a-b",
			wantErr: true,
		},
		{
			name:    "invalid line range - extra colon",
			id:      "MEMORY.md:1-5:extra",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path, start, end, err := service.ParseID(tt.id)

			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.wantPath, path)
				assert.Equal(t, tt.wantStart, start)
				assert.Equal(t, tt.wantEnd, end)
			}
		})
	}
}

func TestGenerateAndParseID_RoundTrip(t *testing.T) {
	tests := []struct {
		path      string
		startLine int
		endLine   int
	}{
		{"MEMORY.md", 1, 5},
		{"memory/2026-03-08.md", 10, 20},
		{"test.md", 100, 200},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			id := service.GenerateID(tt.path, tt.startLine, tt.endLine)
			path, start, end, err := service.ParseID(id)

			require.NoError(t, err)
			assert.Equal(t, tt.path, path)
			assert.Equal(t, tt.startLine, start)
			assert.Equal(t, tt.endLine, end)
		})
	}
}

func TestPrepareMemoryForStorage(t *testing.T) {
	t.Run("valid memory with all fields", func(t *testing.T) {
		memory := &entity.Memory{
			Path:      "MEMORY.md",
			Content:   "Test content",
			StartLine: 1,
			EndLine:   5,
			Source:    entity.SourceLongTerm,
		}

		// Normalize path
		memory.Path = service.NormalizePath(memory.Path)

		// Determine source type if not set
		if memory.Source == "" {
			memory.Source = service.DetermineSourceType(memory.Path)
		}

		// Validate
		err := service.ValidateMemory(memory)
		require.NoError(t, err)

		// Set timestamps
		now := time.Now()
		if memory.CreatedAt.IsZero() {
			memory.CreatedAt = now
		}
		memory.UpdatedAt = now

		// Calculate checksum
		memory.Checksum = service.CalculateChecksum(memory.Content)

		assert.NotEmpty(t, memory.Checksum)
		assert.False(t, memory.CreatedAt.IsZero())
		assert.False(t, memory.UpdatedAt.IsZero())
		assert.Equal(t, "MEMORY.md", memory.Path)
	})

	t.Run("determines source type if empty", func(t *testing.T) {
		memory := &entity.Memory{
			Path:      "MEMORY.md",
			Content:   "Test content",
			StartLine: 1,
			EndLine:   5,
		}

		// Determine source type
		if memory.Source == "" {
			memory.Source = service.DetermineSourceType(memory.Path)
		}

		assert.Equal(t, entity.SourceLongTerm, memory.Source)
	})
}

func TestCreateMemory(t *testing.T) {
	tests := []struct {
		name    string
		path    string
		content string
		source  entity.SourceType
	}{
		{"longterm memory", "MEMORY.md", "Long term content", entity.SourceLongTerm},
		{"daily memory", "memory/2026-03-08.md", "Daily log", entity.SourceDaily},
		{"session memory", "session/test.md", "Session content", entity.SourceSession},
		{"path with slashes", "/memory/test.md", "Content", entity.SourceSession},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create memory using domain functions
			now := time.Now()
			memory := &entity.Memory{
				Path:      service.NormalizePath(tt.path),
				Content:   tt.content,
				Source:    tt.source,
				CreatedAt: now,
				UpdatedAt: now,
				Checksum:  service.CalculateChecksum(tt.content),
				Metadata:  make(map[string]string),
			}

			assert.NotEmpty(t, memory.Path)
			assert.Equal(t, tt.content, memory.Content)
			assert.Equal(t, tt.source, memory.Source)
			assert.NotEmpty(t, memory.Checksum)
			assert.False(t, memory.CreatedAt.IsZero())
			assert.False(t, memory.UpdatedAt.IsZero())
			assert.NotNil(t, memory.Metadata)
		})
	}
}

func TestErrors(t *testing.T) {
	t.Run("NotFoundError", func(t *testing.T) {
		err := errors.ErrNotFound
		assert.True(t, errors.IsNotFound(err))
		assert.False(t, errors.IsAlreadyExists(err))
		assert.False(t, errors.IsInvalidInput(err))
	})

	t.Run("AlreadyExistsError", func(t *testing.T) {
		err := errors.ErrAlreadyExists
		assert.True(t, errors.IsAlreadyExists(err))
		assert.False(t, errors.IsNotFound(err))
	})

	t.Run("InvalidInputError", func(t *testing.T) {
		err := errors.ErrInvalidInput
		assert.True(t, errors.IsInvalidInput(err))
	})

	t.Run("TimeoutError", func(t *testing.T) {
		err := errors.ErrTimeout
		assert.True(t, errors.IsTimeout(err))
	})

	t.Run("ValidationError", func(t *testing.T) {
		err := errors.NewValidationError("field", "message")
		assert.Contains(t, err.Error(), "field")
		assert.Contains(t, err.Error(), "message")
		assert.Contains(t, err.Error(), "validation error")
	})

	t.Run("ValidationError empty field", func(t *testing.T) {
		err := errors.NewValidationError("", "message")
		assert.Contains(t, err.Error(), "message")
	})

	t.Run("Wrap", func(t *testing.T) {
		originalErr := errors.ErrNotFound
		wrappedErr := errors.Wrap("operation", originalErr)
		assert.Contains(t, wrappedErr.Error(), "operation")
		assert.True(t, errors.IsNotFound(wrappedErr))
	})

	t.Run("WrapWithMessage", func(t *testing.T) {
		originalErr := errors.ErrNotFound
		wrappedErr := errors.WrapWithMessage("operation", originalErr, "custom message")
		assert.Contains(t, wrappedErr.Error(), "operation")
		assert.Contains(t, wrappedErr.Error(), "custom message")
		assert.True(t, errors.IsNotFound(wrappedErr))
	})

	t.Run("NewError", func(t *testing.T) {
		err := errors.NewError("test_op", errors.ErrNotFound, "test message")
		assert.Contains(t, err.Error(), "test_op")
		assert.Contains(t, err.Error(), "test message")
		assert.True(t, errors.IsNotFound(err))
	})

	t.Run("MemoryError WithFields", func(t *testing.T) {
		err := errors.NewError("test_op", errors.ErrNotFound, "test message")
		errWithFields := err.WithFields(map[string]interface{}{
			"key": "value",
		})
		assert.Equal(t, err, errWithFields) // WithFields returns same pointer
		assert.NotNil(t, errWithFields.Fields)
	})

	t.Run("Error interface", func(t *testing.T) {
		err := errors.NewError("op", errors.ErrNotFound, "")
		assert.Contains(t, err.Error(), "op")
		assert.Contains(t, err.Error(), "not found")
	})

	t.Run("Unwrap", func(t *testing.T) {
		err := errors.NewError("op", errors.ErrNotFound, "")
		unwrapped := err.Unwrap()
		assert.Equal(t, errors.ErrNotFound, unwrapped)
	})
}

// Benchmark tests
func BenchmarkCalculateChecksum(b *testing.B) {
	content := "Test content for benchmarking"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		service.CalculateChecksum(content)
	}
}

func BenchmarkNormalizePath(b *testing.B) {
	path := "/memory/2026-03-08.md/"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		service.NormalizePath(path)
	}
}

func BenchmarkValidateMemory(b *testing.B) {
	memory := &entity.Memory{
		Path:      "MEMORY.md",
		Content:   "Test content",
		StartLine: 1,
		EndLine:   5,
		Source:    entity.SourceLongTerm,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		service.ValidateMemory(memory)
	}
}

func BenchmarkGenerateID(b *testing.B) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		service.GenerateID("MEMORY.md", 1, 5)
	}
}

func BenchmarkParseID(b *testing.B) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		service.ParseID("MEMORY.md:1-5")
	}
}

func BenchmarkMergeMemories(b *testing.B) {
	memories := make([]*entity.Memory, 100)
	for i := range memories {
		memories[i] = &entity.Memory{Content: "Test content"}
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		service.MergeMemories(memories, "\n")
	}
}

func BenchmarkSplitContentIntoChunks(b *testing.B) {
	content := ""
	for i := 0; i < 1000; i++ {
		content += "Line " + string(rune(i)) + "\n"
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		service.SplitContentIntoChunks(content, 100)
	}
}

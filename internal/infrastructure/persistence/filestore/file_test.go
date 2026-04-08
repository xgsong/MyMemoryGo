package filestore_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/xgsong/MyMemoryGo/internal/domain/entity"
	"github.com/xgsong/MyMemoryGo/internal/infrastructure/persistence/filestore"
)

func TestDefaultFileManagerConfig(t *testing.T) {
	cfg := filestore.DefaultConfig()

	assert.NotEmpty(t, cfg.WorkspaceDir)
	assert.Equal(t, "MEMORY.md", cfg.LongTermFile)
	assert.Equal(t, "memory", cfg.DailyDir)
	assert.Equal(t, ".md", cfg.FileExtension)
}

func TestNewFileManager(t *testing.T) {
	t.Run("with config", func(t *testing.T) {
		tmpDir := t.TempDir()

		fm, err := filestore.New(&filestore.Config{
			WorkspaceDir:  tmpDir,
			LongTermFile:  "MEMORY.md",
			DailyDir:      "memory",
			FileExtension: ".md",
		})
		require.NoError(t, err)
		require.NotNil(t, fm)
		defer fm.Close()
	})

	t.Run("with nil config uses defaults", func(t *testing.T) {
		// Skip to avoid creating directories in user's home
	})

	t.Run("creates directories", func(t *testing.T) {
		tmpDir := t.TempDir()
		workspaceDir := filepath.Join(tmpDir, "workspace")

		fm, err := filestore.New(&filestore.Config{
			WorkspaceDir:  workspaceDir,
			LongTermFile:  "MEMORY.md",
			DailyDir:      "memory",
			FileExtension: ".md",
		})
		require.NoError(t, err)
		defer fm.Close()

		// Check directories were created
		_, err = os.Stat(workspaceDir)
		assert.NoError(t, err)

		_, err = os.Stat(filepath.Join(workspaceDir, "memory"))
		assert.NoError(t, err)
	})
}

func TestFileManager_Read(t *testing.T) {
	fm, cleanup := createTestFileManager(t)
	defer cleanup()

	ctx := context.Background()

	t.Run("read existing file", func(t *testing.T) {
		content := []byte("# Test Memory\n\nThis is a test.")
		err := fm.Write(ctx, "MEMORY.md", content)
		require.NoError(t, err)

		readContent, err := fm.Read(ctx, "MEMORY.md")
		require.NoError(t, err)
		assert.Equal(t, content, readContent)
	})

	t.Run("read non-existing file", func(t *testing.T) {
		_, err := fm.Read(ctx, "nonexistent.md")
		require.Error(t, err)
	})
}

func TestFileManager_Write(t *testing.T) {
	fm, cleanup := createTestFileManager(t)
	defer cleanup()

	ctx := context.Background()

	t.Run("write new file", func(t *testing.T) {
		content := []byte("New file content")
		err := fm.Write(ctx, "new.md", content)
		require.NoError(t, err)

		readContent, err := fm.Read(ctx, "new.md")
		require.NoError(t, err)
		assert.Equal(t, content, readContent)
	})

	t.Run("overwrite existing file", func(t *testing.T) {
		content1 := []byte("Original content")
		err := fm.Write(ctx, "overwrite.md", content1)
		require.NoError(t, err)

		content2 := []byte("New content")
		err = fm.Write(ctx, "overwrite.md", content2)
		require.NoError(t, err)

		readContent, err := fm.Read(ctx, "overwrite.md")
		require.NoError(t, err)
		assert.Equal(t, content2, readContent)
	})

	t.Run("write to nested path", func(t *testing.T) {
		content := []byte("Nested content")
		err := fm.Write(ctx, "nested/deep/file.md", content)
		require.NoError(t, err)

		readContent, err := fm.Read(ctx, "nested/deep/file.md")
		require.NoError(t, err)
		assert.Equal(t, content, readContent)
	})
}

func TestFileManager_Append(t *testing.T) {
	fm, cleanup := createTestFileManager(t)
	defer cleanup()

	ctx := context.Background()

	t.Run("append to existing file", func(t *testing.T) {
		content1 := []byte("First line")
		err := fm.Write(ctx, "append.md", content1)
		require.NoError(t, err)

		content2 := []byte("Second line")
		err = fm.Append(ctx, "append.md", content2)
		require.NoError(t, err)

		readContent, err := fm.Read(ctx, "append.md")
		require.NoError(t, err)
		assert.Contains(t, string(readContent), "First line")
		assert.Contains(t, string(readContent), "Second line")
	})

	t.Run("append to new file", func(t *testing.T) {
		content := []byte("New file content")
		err := fm.Append(ctx, "new-append.md", content)
		require.NoError(t, err)

		readContent, err := fm.Read(ctx, "new-append.md")
		require.NoError(t, err)
		assert.Equal(t, content, readContent)
	})
}

func TestFileManager_Delete(t *testing.T) {
	fm, cleanup := createTestFileManager(t)
	defer cleanup()

	ctx := context.Background()

	t.Run("delete existing file", func(t *testing.T) {
		content := []byte("To be deleted")
		err := fm.Write(ctx, "delete.md", content)
		require.NoError(t, err)

		err = fm.Delete(ctx, "delete.md")
		require.NoError(t, err)

		_, err = fm.Read(ctx, "delete.md")
		require.Error(t, err)
	})

	t.Run("delete non-existing file", func(t *testing.T) {
		err := fm.Delete(ctx, "nonexistent.md")
		require.Error(t, err)
	})
}

func TestFileManager_Exists(t *testing.T) {
	fm, cleanup := createTestFileManager(t)
	defer cleanup()

	ctx := context.Background()

	t.Run("existing file", func(t *testing.T) {
		err := fm.Write(ctx, "exists.md", []byte("content"))
		require.NoError(t, err)

		exists, err := fm.Exists(ctx, "exists.md")
		require.NoError(t, err)
		assert.True(t, exists)
	})

	t.Run("non-existing file", func(t *testing.T) {
		exists, err := fm.Exists(ctx, "nonexistent.md")
		require.NoError(t, err)
		assert.False(t, exists)
	})
}

func TestFileManager_List(t *testing.T) {
	fm, cleanup := createTestFileManager(t)
	defer cleanup()

	ctx := context.Background()

	// Create some files
	files := []string{"file1.md", "file2.md", "memory/2026-03-08.md"}
	for _, f := range files {
		err := fm.Write(ctx, f, []byte("content"))
		require.NoError(t, err)
	}

	t.Run("list all files", func(t *testing.T) {
		result, err := fm.List(ctx, "*.md")
		require.NoError(t, err)
		assert.GreaterOrEqual(t, len(result), 2) // At least 2 files
	})
}

func TestFileManager_GetLongTermPath(t *testing.T) {
	fm, cleanup := createTestFileManager(t)
	defer cleanup()

	path := fm.GetLongTermPath()
	assert.Equal(t, "MEMORY.md", path)
}

func TestFileManager_GetDailyPath(t *testing.T) {
	fm, cleanup := createTestFileManager(t)
	defer cleanup()

	date := time.Date(2026, 3, 8, 0, 0, 0, 0, time.UTC)
	path := fm.GetDailyPath(date)
	// Accept both forward and backward slashes (platform-dependent)
	assert.Contains(t, path, "memory")
	assert.Contains(t, path, "2026-03-08.md")
}

func TestFileManager_ParseDailyPath(t *testing.T) {
	fm, cleanup := createTestFileManager(t)
	defer cleanup()

	t.Run("valid daily path", func(t *testing.T) {
		date, err := fm.ParseDailyPath("memory/2026-03-08.md")
		require.NoError(t, err)
		assert.Equal(t, 2026, date.Year())
		assert.Equal(t, time.March, date.Month())
		assert.Equal(t, 8, date.Day())
	})

	t.Run("invalid format", func(t *testing.T) {
		_, err := fm.ParseDailyPath("memory/invalid.md")
		require.Error(t, err)
	})
}

func TestFileManager_IsLongTerm(t *testing.T) {
	fm, cleanup := createTestFileManager(t)
	defer cleanup()

	tests := []struct {
		path     string
		expected bool
	}{
		{"MEMORY.md", true},
		{"memory.md", false}, // Case sensitive
		{"memory/2026-03-08.md", false},
		{"other.md", false},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			result := fm.IsLongTerm(tt.path)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestFileManager_IsDaily(t *testing.T) {
	fm, cleanup := createTestFileManager(t)
	defer cleanup()

	tests := []struct {
		path     string
		expected bool
	}{
		{"memory/2026-03-08.md", true},
		{"memory/2025-12-31.md", true},
		{"MEMORY.md", false},
		{"memory/invalid.md", false},
		{"other/2026-03-08.md", false},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			result := fm.IsDaily(tt.path)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestFileManager_DetermineSourceType(t *testing.T) {
	fm, cleanup := createTestFileManager(t)
	defer cleanup()

	tests := []struct {
		path     string
		expected entity.SourceType
	}{
		{"MEMORY.md", entity.SourceLongTerm},
		{"memory/2026-03-08.md", entity.SourceDaily},
		{"notes/test.md", entity.SourceSession},
		{"other.md", entity.SourceSession},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			result := fm.DetermineSourceType(tt.path)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestFileManager_ListLongTerm(t *testing.T) {
	fm, cleanup := createTestFileManager(t)
	defer cleanup()

	ctx := context.Background()

	t.Run("no long-term file", func(t *testing.T) {
		files, err := fm.ListLongTerm(ctx)
		require.NoError(t, err)
		assert.Empty(t, files)
	})

	t.Run("with long-term file", func(t *testing.T) {
		err := fm.Write(ctx, "MEMORY.md", []byte("Long-term memory"))
		require.NoError(t, err)

		files, err := fm.ListLongTerm(ctx)
		require.NoError(t, err)
		assert.Len(t, files, 1)
		assert.Equal(t, "MEMORY.md", files[0])
	})
}

func TestFileManager_ListDaily(t *testing.T) {
	fm, cleanup := createTestFileManager(t)
	defer cleanup()

	ctx := context.Background()

	t.Run("no daily files", func(t *testing.T) {
		files, err := fm.ListDaily(ctx)
		require.NoError(t, err)
		assert.Empty(t, files)
	})

	t.Run("with daily files", func(t *testing.T) {
		err := fm.Write(ctx, "memory/2026-03-08.md", []byte("Daily log 1"))
		require.NoError(t, err)
		err = fm.Write(ctx, "memory/2026-03-07.md", []byte("Daily log 2"))
		require.NoError(t, err)

		files, err := fm.ListDaily(ctx)
		require.NoError(t, err)
		assert.Len(t, files, 2)
	})
}

func TestFileManager_Close(t *testing.T) {
	tmpDir := t.TempDir()

	fm, err := filestore.New(&filestore.Config{
		WorkspaceDir:  tmpDir,
		LongTermFile:  "MEMORY.md",
		DailyDir:      "memory",
		FileExtension: ".md",
	})
	require.NoError(t, err)

	err = fm.Close()
	require.NoError(t, err)
}

// Helper functions

func createTestFileManager(t *testing.T) (*filestore.Manager, func()) {
	tmpDir := t.TempDir()

	fm, err := filestore.New(&filestore.Config{
		WorkspaceDir:  tmpDir,
		LongTermFile:  "MEMORY.md",
		DailyDir:      "memory",
		FileExtension: ".md",
	})
	require.NoError(t, err)

	cleanup := func() {
		fm.Close()
	}

	return fm, cleanup
}

// Benchmark tests

func BenchmarkFileManager_Read(b *testing.B) {
	tmpDir := b.TempDir()

	fm, _ := filestore.New(&filestore.Config{
		WorkspaceDir:  tmpDir,
		LongTermFile:  "MEMORY.md",
		DailyDir:      "memory",
		FileExtension: ".md",
	})
	defer fm.Close()

	ctx := context.Background()
	content := []byte("Benchmark content")
	fm.Write(ctx, "bench.md", content)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		fm.Read(ctx, "bench.md")
	}
}

func BenchmarkFileManager_Write(b *testing.B) {
	tmpDir := b.TempDir()

	fm, _ := filestore.New(&filestore.Config{
		WorkspaceDir:  tmpDir,
		LongTermFile:  "MEMORY.md",
		DailyDir:      "memory",
		FileExtension: ".md",
	})
	defer fm.Close()

	ctx := context.Background()
	content := []byte("Benchmark content")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		fm.Write(ctx, string(rune(i))+".md", content)
	}
}

func BenchmarkFileManager_DetermineSourceType(b *testing.B) {
	tmpDir := b.TempDir()

	fm, _ := filestore.New(&filestore.Config{
		WorkspaceDir:  tmpDir,
		LongTermFile:  "MEMORY.md",
		DailyDir:      "memory",
		FileExtension: ".md",
	})
	defer fm.Close()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		fm.DetermineSourceType("memory/2026-03-08.md")
	}
}

func TestFileError(t *testing.T) {
	t.Run("error message contains operation and underlying error", func(t *testing.T) {
		underlying := os.ErrNotExist
		err := filestore.FileError{
			Op:  "read file",
			Err: underlying,
		}

		assert.Equal(t, "read file: file does not exist", err.Error())
	})

	t.Run("Unwrap returns underlying error", func(t *testing.T) {
		underlying := os.ErrPermission
		err := filestore.FileError{
			Op:  "write file",
			Err: underlying,
		}

		assert.Equal(t, underlying, err.Unwrap())
	})

	t.Run("errors.Is works with FileError", func(t *testing.T) {
		underlying := os.ErrNotExist
		err := &filestore.FileError{
			Op:  "open file",
			Err: underlying,
		}

		assert.True(t, errors.Is(err, os.ErrNotExist))
	})
}

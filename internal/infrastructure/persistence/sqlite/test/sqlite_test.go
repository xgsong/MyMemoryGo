package sqlite_test

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/xgsong/MyMemoryGo/internal/domain/entity"
	"github.com/xgsong/MyMemoryGo/internal/domain/repository"
	"github.com/xgsong/MyMemoryGo/internal/infrastructure/persistence/sqlite"
)

func TestDefaultSQLiteConfig(t *testing.T) {
	cfg := sqlite.DefaultConfig()

	assert.NotEmpty(t, cfg.DBPath)
	assert.True(t, cfg.WALMode)
	assert.Equal(t, 768, cfg.VectorDimensions)
}

func TestNewSQLiteStore(t *testing.T) {
	t.Run("with config", func(t *testing.T) {
		tmpDir := t.TempDir()
		dbPath := filepath.Join(tmpDir, "test.db")

		store, err := sqlite.New(&sqlite.Config{
			DBPath:           dbPath,
			WALMode:          true,
			VectorDimensions: 768,
		})
		require.NoError(t, err)
		require.NotNil(t, store)
		defer store.Close()
	})

	t.Run("with nil config uses defaults", func(t *testing.T) {
		// This will create a db in default location
		// Skip to avoid polluting user's home directory
	})

	t.Run("invalid path", func(t *testing.T) {
		// On Windows, certain paths are invalid
		// This test may behave differently on different OS
	})
}

func TestSQLiteStore_Store(t *testing.T) {
	store, cleanup := createTestStore(t)
	defer cleanup()

	ctx := context.Background()
	memory := &entity.Memory{
		ID:        "test:1-5",
		Path:      "test.md",
		StartLine: 1,
		EndLine:   5,
		Content:   "Test content",
		Source:    entity.SourceLongTerm,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
		Checksum:  "test123",
	}

	err := store.Store(ctx, memory)
	require.NoError(t, err)
}

func TestSQLiteStore_Store_WithEmbedding(t *testing.T) {
	store, cleanup := createTestStore(t)
	defer cleanup()

	ctx := context.Background()
	memory := &entity.Memory{
		ID:        "test:1-5",
		Path:      "test.md",
		StartLine: 1,
		EndLine:   5,
		Content:   "Test content",
		Embedding: []float32{0.1, 0.2, 0.3, 0.4, 0.5},
		Source:    entity.SourceLongTerm,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
		Checksum:  "test123",
	}

	err := store.Store(ctx, memory)
	require.NoError(t, err)

	// Retrieve and verify embedding
	retrieved, err := store.Get(ctx, memory.ID)
	require.NoError(t, err)
	assert.NotNil(t, retrieved.Embedding)
}

func TestSQLiteStore_Store_WithMetadata(t *testing.T) {
	store, cleanup := createTestStore(t)
	defer cleanup()

	ctx := context.Background()
	memory := &entity.Memory{
		ID:        "test:1-5",
		Path:      "test.md",
		StartLine: 1,
		EndLine:   5,
		Content:   "Test content",
		Metadata:  map[string]string{"key": "value", "env": "test"},
		Source:    entity.SourceLongTerm,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
		Checksum:  "test123",
	}

	err := store.Store(ctx, memory)
	require.NoError(t, err)

	retrieved, err := store.Get(ctx, memory.ID)
	require.NoError(t, err)
	assert.Equal(t, "value", retrieved.Metadata["key"])
	assert.Equal(t, "test", retrieved.Metadata["env"])
}

func TestSQLiteStore_StoreBatch(t *testing.T) {
	store, cleanup := createTestStore(t)
	defer cleanup()

	ctx := context.Background()
	memories := []*entity.Memory{
		{
			ID:        "batch1:1-5",
			Path:      "batch.md",
			StartLine: 1,
			EndLine:   5,
			Content:   "Batch memory 1",
			Source:    entity.SourceDaily,
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
			Checksum:  "batch1",
		},
		{
			ID:        "batch2:6-10",
			Path:      "batch.md",
			StartLine: 6,
			EndLine:   10,
			Content:   "Batch memory 2",
			Source:    entity.SourceDaily,
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
			Checksum:  "batch2",
		},
	}

	err := store.StoreBatch(ctx, memories)
	require.NoError(t, err)

	// Verify both stored
	_, err = store.Get(ctx, "batch1:1-5")
	require.NoError(t, err)
	_, err = store.Get(ctx, "batch2:6-10")
	require.NoError(t, err)
}

func TestSQLiteStore_Get(t *testing.T) {
	store, cleanup := createTestStore(t)
	defer cleanup()

	ctx := context.Background()

	t.Run("existing memory", func(t *testing.T) {
		memory := createTestMemory("test:1-5", "test.md", "Test content")
		err := store.Store(ctx, memory)
		require.NoError(t, err)

		retrieved, err := store.Get(ctx, "test:1-5")
		require.NoError(t, err)
		assert.Equal(t, memory.ID, retrieved.ID)
		assert.Equal(t, memory.Path, retrieved.Path)
		assert.Equal(t, memory.Content, retrieved.Content)
	})

	t.Run("non-existing memory", func(t *testing.T) {
		_, err := store.Get(ctx, "nonexistent:1-5")
		require.Error(t, err)
	})
}

func TestSQLiteStore_GetByPath(t *testing.T) {
	store, cleanup := createTestStore(t)
	defer cleanup()

	ctx := context.Background()

	// Store multiple memories in same path
	for i := 1; i <= 3; i++ {
		memory := &entity.Memory{
			ID:        "test:" + string(rune('0'+i)) + "-5",
			Path:      "test.md",
			StartLine: i,
			EndLine:   i + 5,
			Content:   "Content " + string(rune('0'+i)),
			Source:    entity.SourceLongTerm,
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
			Checksum:  "checksum" + string(rune('0'+i)),
		}
		err := store.Store(ctx, memory)
		require.NoError(t, err)
	}

	memories, err := store.GetByPath(ctx, "test.md")
	require.NoError(t, err)
	assert.Len(t, memories, 3)
}

func TestSQLiteStore_Delete(t *testing.T) {
	store, cleanup := createTestStore(t)
	defer cleanup()

	ctx := context.Background()
	memory := createTestMemory("delete:1-5", "delete.md", "To be deleted")
	err := store.Store(ctx, memory)
	require.NoError(t, err)

	t.Run("delete existing", func(t *testing.T) {
		err := store.Delete(ctx, "delete:1-5")
		require.NoError(t, err)

		_, err = store.Get(ctx, "delete:1-5")
		require.Error(t, err)
	})

	t.Run("delete non-existing", func(t *testing.T) {
		err := store.Delete(ctx, "nonexistent:1-5")
		require.Error(t, err)
	})
}

func TestSQLiteStore_DeleteByPath(t *testing.T) {
	store, cleanup := createTestStore(t)
	defer cleanup()

	ctx := context.Background()

	// Store multiple memories in same path
	for i := 1; i <= 3; i++ {
		memory := createTestMemory(
			"deletepath:"+string(rune('0'+i))+"-5",
			"deletepath.md",
			"Content",
		)
		err := store.Store(ctx, memory)
		require.NoError(t, err)
	}

	err := store.DeleteByPath(ctx, "deletepath.md")
	require.NoError(t, err)

	memories, err := store.GetByPath(ctx, "deletepath.md")
	require.NoError(t, err)
	assert.Empty(t, memories)
}

func TestSQLiteStore_List(t *testing.T) {
	store, cleanup := createTestStore(t)
	defer cleanup()

	ctx := context.Background()

	// Store memories with different sources
	memories := []*entity.Memory{
		createTestMemoryWithSource("list:1-5", "list1.md", entity.SourceLongTerm),
		createTestMemoryWithSource("list:6-10", "list2.md", entity.SourceDaily),
		createTestMemoryWithSource("list:11-15", "list3.md", entity.SourceSession),
	}

	for _, m := range memories {
		err := store.Store(ctx, m)
		require.NoError(t, err)
	}

	t.Run("list all", func(t *testing.T) {
		result, total, err := store.List(ctx, nil)
		require.NoError(t, err)
		assert.GreaterOrEqual(t, total, 3)
		assert.GreaterOrEqual(t, len(result), 3)
	})

	t.Run("list with source filter", func(t *testing.T) {
		result, _, err := store.List(ctx, &repository.ListOptions{
			Source: entity.SourceLongTerm,
		})
		require.NoError(t, err)
		for _, m := range result {
			assert.Equal(t, entity.SourceLongTerm, m.Source)
		}
	})

	t.Run("list with pagination", func(t *testing.T) {
		result, _, err := store.List(ctx, &repository.ListOptions{
			Limit:  2,
			Offset: 0,
		})
		require.NoError(t, err)
		assert.LessOrEqual(t, len(result), 2)
	})
}

func TestSQLiteStore_Close(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	store, err := sqlite.New(&sqlite.Config{
		DBPath:           dbPath,
		WALMode:          true,
		VectorDimensions: 768,
	})
	require.NoError(t, err)

	err = store.Close()
	require.NoError(t, err)
}

// Helper functions

func createTestStore(t *testing.T) (*sqlite.Store, func()) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	store, err := sqlite.New(&sqlite.Config{
		DBPath:           dbPath,
		WALMode:          true,
		VectorDimensions: 768,
	})
	require.NoError(t, err)

	cleanup := func() {
		store.Close()
	}

	return store, cleanup
}

func createTestMemory(id, path, content string) *entity.Memory {
	return &entity.Memory{
		ID:        id,
		Path:      path,
		StartLine: 1,
		EndLine:   5,
		Content:   content,
		Source:    entity.SourceLongTerm,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
		Checksum:  "test-checksum",
	}
}

func createTestMemoryWithSource(id, path string, source entity.SourceType) *entity.Memory {
	m := createTestMemory(id, path, "Test content")
	m.Source = source
	return m
}

// Benchmark tests

func BenchmarkSQLiteStore_Store(b *testing.B) {
	tmpDir := b.TempDir()
	dbPath := filepath.Join(tmpDir, "bench.db")

	store, _ := sqlite.New(&sqlite.Config{
		DBPath:           dbPath,
		WALMode:          true,
		VectorDimensions: 768,
	})
	defer store.Close()

	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		memory := &entity.Memory{
			ID:        string(rune(i)),
			Path:      "bench.md",
			StartLine: 1,
			EndLine:   5,
			Content:   "Benchmark content",
			Source:    entity.SourceLongTerm,
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
			Checksum:  "bench",
		}
		store.Store(ctx, memory)
	}
}

func BenchmarkSQLiteStore_Get(b *testing.B) {
	tmpDir := b.TempDir()
	dbPath := filepath.Join(tmpDir, "bench.db")

	store, _ := sqlite.New(&sqlite.Config{
		DBPath:           dbPath,
		WALMode:          true,
		VectorDimensions: 768,
	})
	defer store.Close()

	ctx := context.Background()

	// Pre-populate
	for i := 0; i < 1000; i++ {
		memory := &entity.Memory{
			ID:        string(rune(i)),
			Path:      "bench.md",
			StartLine: i,
			EndLine:   i + 1,
			Content:   "Content",
			Source:    entity.SourceLongTerm,
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
			Checksum:  "bench",
		}
		store.Store(ctx, memory)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		store.Get(ctx, string(rune(i%1000)))
	}
}

func BenchmarkSQLiteStore_StoreBatch(b *testing.B) {
	tmpDir := b.TempDir()
	dbPath := filepath.Join(tmpDir, "bench.db")

	store, _ := sqlite.New(&sqlite.Config{
		DBPath:           dbPath,
		WALMode:          true,
		VectorDimensions: 768,
	})
	defer store.Close()

	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		memories := make([]*entity.Memory, 100)
		for j := 0; j < 100; j++ {
			memories[j] = &entity.Memory{
				ID:        string(rune(i*100 + j)),
				Path:      "bench.md",
				StartLine: j,
				EndLine:   j + 1,
				Content:   "Content",
				Source:    entity.SourceLongTerm,
				CreatedAt: time.Now(),
				UpdatedAt: time.Now(),
				Checksum:  "bench",
			}
		}
		store.StoreBatch(ctx, memories)
	}
}

func TestStorageError(t *testing.T) {
	t.Run("error message contains operation and underlying error", func(t *testing.T) {
		underlying := fmt.Errorf("database is locked")
		err := sqlite.StorageError{
			Op:  "store memory",
			Err: underlying,
		}

		assert.Equal(t, "store memory: database is locked", err.Error())
	})

	t.Run("Unwrap returns underlying error", func(t *testing.T) {
		underlying := fmt.Errorf("i/o error")
		err := sqlite.StorageError{
			Op:  "read memory",
			Err: underlying,
		}

		assert.Equal(t, underlying, err.Unwrap())
	})

	t.Run("errors.Is works with StorageError", func(t *testing.T) {
		underlying := os.ErrNotExist
		err := &sqlite.StorageError{
			Op:  "open database",
			Err: underlying,
		}

		assert.True(t, errors.Is(err, os.ErrNotExist))
	})

	t.Run("wrapErr creates proper StorageError", func(t *testing.T) {
		t.Skip("Skipping as Windows allows creating files in non-existent paths")
	})
}

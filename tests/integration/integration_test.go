package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/xgsong/MyMemoryGo/internal/application/service"
	"github.com/xgsong/MyMemoryGo/internal/domain/entity"
	"github.com/xgsong/MyMemoryGo/internal/infrastructure/persistence/filestore"
	"github.com/xgsong/MyMemoryGo/internal/infrastructure/persistence/sqlite"
	"github.com/xgsong/MyMemoryGo/internal/interface/api"
)

// mockEmbeddingProvider is a mock implementation for testing.
type mockEmbeddingProvider struct {
	mu         sync.Mutex
	embeddings map[string][]float32
}

func newMockEmbeddingProvider() *mockEmbeddingProvider {
	return &mockEmbeddingProvider{
		embeddings: make(map[string][]float32),
	}
}

func (m *mockEmbeddingProvider) Embed(ctx context.Context, text string) ([]float32, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if emb, ok := m.embeddings[text]; ok {
		return emb, nil
	}
	embedding := make([]float32, 768)
	for i := range embedding {
		embedding[i] = float32((len(text)+i)%100) / 100.0
	}
	m.embeddings[text] = embedding
	return embedding, nil
}

func (m *mockEmbeddingProvider) EmbedBatch(ctx context.Context, texts []string) ([][]float32, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	embeddings := make([][]float32, len(texts))
	for i, text := range texts {
		if emb, ok := m.embeddings[text]; ok {
			embeddings[i] = emb
			continue
		}
		embedding := make([]float32, 768)
		for j := range embedding {
			embedding[j] = float32((len(text)+j)%100) / 100.0
		}
		m.embeddings[text] = embedding
		embeddings[i] = embedding
	}
	return embeddings, nil
}

func (m *mockEmbeddingProvider) Model() string {
	return "mock-embedding-model"
}

func (m *mockEmbeddingProvider) Dimensions() int {
	return 768
}

// TestMain runs all integration tests.
func TestMain(m *testing.M) {
	fmt.Println("Running Integration Tests...")
	fmt.Println("==========================")
	code := m.Run()
	if code == 0 {
		fmt.Println("\n==========================")
		fmt.Println("✅ All integration tests passed!")
	} else {
		fmt.Println("\n==========================")
		fmt.Println("❌ Some integration tests failed!")
	}
	os.Exit(code)
}

// TestSQLiteStorage tests SQLite storage operations.
func TestSQLiteStorage(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	store, err := sqlite.New(&sqlite.Config{
		DBPath:           dbPath,
		WALMode:          true,
		VectorDimensions: 768,
	})
	require.NoError(t, err, "Failed to create SQLite store")
	defer store.Close()

	t.Run("StoreAndRetrieve", func(t *testing.T) {
		memory := &entity.Memory{
			ID:        "test:1-5",
			Path:      "test.md",
			StartLine: 1,
			EndLine:   5,
			Content:   "Test content for integration testing",
			Source:    entity.SourceLongTerm,
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
			Checksum:  "test123",
		}

		err = store.Store(context.Background(), memory)
		require.NoError(t, err, "Failed to store memory")

		retrieved, err := store.Get(context.Background(), "test:1-5")
		require.NoError(t, err, "Failed to retrieve memory")
		assert.Equal(t, memory.Content, retrieved.Content, "Content mismatch")
	})

	t.Run("BatchStore", func(t *testing.T) {
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

		err = store.StoreBatch(context.Background(), memories)
		require.NoError(t, err, "Failed to store batch memories")

		for _, mem := range memories {
			retrieved, err := store.Get(context.Background(), mem.ID)
			require.NoError(t, err, "Failed to retrieve batch memory")
			assert.Equal(t, mem.Content, retrieved.Content, "Batch memory content mismatch")
		}
	})

	t.Run("Delete", func(t *testing.T) {
		err = store.Delete(context.Background(), "test:1-5")
		require.NoError(t, err, "Failed to delete memory")

		_, err = store.Get(context.Background(), "test:1-5")
		assert.Error(t, err, "Memory should have been deleted")
	})
}

// TestFileManager tests file manager operations.
func TestFileManager(t *testing.T) {
	tmpDir := t.TempDir()
	workspaceDir := filepath.Join(tmpDir, "workspace")
	err := os.MkdirAll(workspaceDir, 0755)
	require.NoError(t, err)

	fileMgr, err := filestore.New(&filestore.Config{
		WorkspaceDir:  workspaceDir,
		LongTermFile:  "MEMORY.md",
		DailyDir:      "memory",
		FileExtension: ".md",
	})
	require.NoError(t, err, "Failed to create file manager")
	defer fileMgr.Close()

	t.Run("WriteAndRead", func(t *testing.T) {
		testContent := []byte("# Test Memory\n\nThis is a test.")
		err = fileMgr.Write(context.Background(), "MEMORY.md", testContent)
		require.NoError(t, err, "Failed to write file")

		readContent, err := fileMgr.Read(context.Background(), "MEMORY.md")
		require.NoError(t, err, "Failed to read file")
		assert.Equal(t, testContent, readContent, "Content mismatch")
	})

	t.Run("SourceTypeDetection", func(t *testing.T) {
		tests := []struct {
			path     string
			expected entity.SourceType
		}{
			{"MEMORY.md", entity.SourceLongTerm},
			{"memory/2026-03-08.md", entity.SourceDaily},
			{"other/file.md", entity.SourceSession},
		}

		for _, test := range tests {
			result := fileMgr.DetermineSourceType(test.path)
			assert.Equal(t, test.expected, result, "Source type mismatch for path: %s", test.path)
		}
	})
}

// TestAPIEndpoints tests REST API endpoints.
func TestAPIEndpoints(t *testing.T) {
	tmpDir := t.TempDir()
	workspaceDir := filepath.Join(tmpDir, "workspace")
	dbPath := filepath.Join(tmpDir, "test.db")

	err := os.MkdirAll(workspaceDir, 0755)
	require.NoError(t, err)

	embedProvider := newMockEmbeddingProvider()

	store, err := sqlite.New(&sqlite.Config{
		DBPath:           dbPath,
		WALMode:          true,
		VectorDimensions: 768,
	})
	require.NoError(t, err)
	defer store.Close()

	fileMgr, err := filestore.New(&filestore.Config{
		WorkspaceDir:  workspaceDir,
		LongTermFile:  "MEMORY.md",
		DailyDir:      "memory",
		FileExtension: ".md",
	})
	require.NoError(t, err)
	defer fileMgr.Close()

	memoryApp := service.NewMemoryApplicationService(
		store,
		store,
		embedProvider,
		fileMgr,
	)

	server := api.NewServer(memoryApp)
	testServer := httptest.NewServer(server.Handler())
	defer testServer.Close()

	t.Run("HealthCheck", func(t *testing.T) {
		resp, err := http.Get(testServer.URL + "/health")
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusOK, resp.StatusCode)

		body, err := io.ReadAll(resp.Body)
		require.NoError(t, err)

		var result map[string]interface{}
		err = json.Unmarshal(body, &result)
		require.NoError(t, err)

		assert.Equal(t, "ok", result["status"])
	})

	t.Run("StoreMemory", func(t *testing.T) {
		payload := map[string]interface{}{
			"content": "Test memory content",
			"source":  "daily",
		}
		bodyBytes, _ := json.Marshal(payload)

		resp, err := http.Post(
			testServer.URL+"/api/v1/memories",
			"application/json",
			bytes.NewReader(bodyBytes),
		)
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusCreated, resp.StatusCode)

		body, err := io.ReadAll(resp.Body)
		require.NoError(t, err)

		var result map[string]interface{}
		err = json.Unmarshal(body, &result)
		require.NoError(t, err)

		assert.NotEmpty(t, result["id"])
		assert.Equal(t, "Test memory content", result["content"])
	})

	t.Run("SearchMemories", func(t *testing.T) {
		payload := map[string]interface{}{
			"query": "test",
			"limit": 10,
		}
		bodyBytes, _ := json.Marshal(payload)

		resp, err := http.Post(
			testServer.URL+"/api/v1/search",
			"application/json",
			bytes.NewReader(bodyBytes),
		)
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusOK, resp.StatusCode)

		body, err := io.ReadAll(resp.Body)
		require.NoError(t, err)

		var result map[string]interface{}
		err = json.Unmarshal(body, &result)
		require.NoError(t, err)

		assert.NotNil(t, result["hits"])
	})

	t.Run("SyncIndex", func(t *testing.T) {
		resp, err := http.Post(testServer.URL+"/api/v1/sync", "application/json", nil)
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusOK, resp.StatusCode)

		body, err := io.ReadAll(resp.Body)
		require.NoError(t, err)

		var result map[string]interface{}
		err = json.Unmarshal(body, &result)
		require.NoError(t, err)

		assert.Equal(t, "index synchronized", result["message"])
	})
}

// TestServiceLayer tests application service layer.
func TestServiceLayer(t *testing.T) {
	tmpDir := t.TempDir()
	workspaceDir := filepath.Join(tmpDir, "workspace")
	dbPath := filepath.Join(tmpDir, "test.db")

	err := os.MkdirAll(workspaceDir, 0755)
	require.NoError(t, err)

	embedProvider := newMockEmbeddingProvider()

	store, err := sqlite.New(&sqlite.Config{
		DBPath:           dbPath,
		WALMode:          true,
		VectorDimensions: 768,
	})
	require.NoError(t, err)
	defer store.Close()

	fileMgr, err := filestore.New(&filestore.Config{
		WorkspaceDir:  workspaceDir,
		LongTermFile:  "MEMORY.md",
		DailyDir:      "memory",
		FileExtension: ".md",
	})
	require.NoError(t, err)
	defer fileMgr.Close()

	memoryApp := service.NewMemoryApplicationService(
		store,
		store,
		embedProvider,
		fileMgr,
	)

	t.Run("StoreMemory", func(t *testing.T) {
		ctx := context.Background()
		resp, err := memoryApp.StoreMemory(ctx, &service.StoreMemoryRequest{
			Content: "Test content",
			Source:  entity.SourceDaily,
		})
		require.NoError(t, err)
		assert.NotEmpty(t, resp.Memory.ID)
		assert.Equal(t, "Test content", resp.Memory.Content)
		assert.Equal(t, entity.SourceDaily, resp.Memory.Source)
	})

	t.Run("GetMemory", func(t *testing.T) {
		ctx := context.Background()
		stored, err := memoryApp.StoreMemory(ctx, &service.StoreMemoryRequest{
			Content: "Content to retrieve",
			Source:  entity.SourceLongTerm,
		})
		require.NoError(t, err)

		retrieved, err := memoryApp.GetMemory(ctx, stored.Memory.ID)
		require.NoError(t, err)
		assert.Equal(t, stored.Memory.ID, retrieved.ID)
		assert.Equal(t, "Content to retrieve", retrieved.Content)
	})

	t.Run("ListMemories", func(t *testing.T) {
		ctx := context.Background()
		for i := 0; i < 5; i++ {
			_, err := memoryApp.StoreMemory(ctx, &service.StoreMemoryRequest{
				Content:  fmt.Sprintf("Memory %d", i),
				Source:   entity.SourceDaily,
				Metadata: map[string]string{"index": fmt.Sprintf("%d", i)},
			})
			require.NoError(t, err)
		}

		result, err := memoryApp.ListMemories(ctx, &service.ListMemoriesRequest{})
		require.NoError(t, err)
		assert.GreaterOrEqual(t, result.Total, 2)
		assert.GreaterOrEqual(t, len(result.Memories), 2)
	})

	t.Run("SearchMemories", func(t *testing.T) {
		ctx := context.Background()
		_, err := memoryApp.StoreMemory(ctx, &service.StoreMemoryRequest{
			Content: "Important meeting notes",
			Source:  entity.SourceDaily,
		})
		require.NoError(t, err)

		resp, err := memoryApp.SearchMemories(ctx, &service.SearchMemoriesRequest{
			Query: "meeting",
			Limit: 10,
		})
		require.NoError(t, err)
		assert.GreaterOrEqual(t, len(resp.Results.Hits), 1)
	})

	t.Run("DeleteMemory", func(t *testing.T) {
		ctx := context.Background()
		stored, err := memoryApp.StoreMemory(ctx, &service.StoreMemoryRequest{
			Content: "Memory to delete",
			Source:  entity.SourceSession,
		})
		require.NoError(t, err)

		err = memoryApp.DeleteMemory(ctx, stored.Memory.ID)
		require.NoError(t, err)

		_, err = memoryApp.GetMemory(ctx, stored.Memory.ID)
		assert.Error(t, err)
	})
}

// TestConcurrency tests concurrent operations.
func TestConcurrency(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	store, err := sqlite.New(&sqlite.Config{
		DBPath:           dbPath,
		WALMode:          true,
		VectorDimensions: 768,
	})
	require.NoError(t, err)
	defer store.Close()

	t.Run("ConcurrentWrites", func(t *testing.T) {
		ctx := context.Background()
		done := make(chan bool, 10)

		for i := 0; i < 10; i++ {
			go func(id int) {
				memory := &entity.Memory{
					ID:        fmt.Sprintf("concurrent:%d-1", id),
					Path:      "concurrent.md",
					StartLine: id,
					EndLine:   id + 1,
					Content:   fmt.Sprintf("Concurrent memory %d", id),
					Source:    entity.SourceDaily,
					CreatedAt: time.Now(),
					UpdatedAt: time.Now(),
					Checksum:  fmt.Sprintf("checksum%d", id),
				}
				err := store.Store(ctx, memory)
				assert.NoError(t, err)
				done <- true
			}(i)
		}

		for i := 0; i < 10; i++ {
			<-done
		}

		memories, total, err := store.List(ctx, nil)
		require.NoError(t, err)
		assert.GreaterOrEqual(t, total, 10)
		assert.GreaterOrEqual(t, len(memories), 10)
	})
}

// TestErrorHandling tests error scenarios.
func TestErrorHandling(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	store, err := sqlite.New(&sqlite.Config{
		DBPath:           dbPath,
		WALMode:          true,
		VectorDimensions: 768,
	})
	require.NoError(t, err)
	defer store.Close()

	t.Run("GetNonExistent", func(t *testing.T) {
		_, err := store.Get(context.Background(), "nonexistent:1-5")
		assert.Error(t, err)
	})

	t.Run("DeleteNonExistent", func(t *testing.T) {
		err := store.Delete(context.Background(), "nonexistent:1-5")
		assert.Error(t, err)
	})

	t.Run("EmptyContent", func(t *testing.T) {
		memory := &entity.Memory{
			ID:        "empty:1-5",
			Path:      "empty.md",
			Content:   "",
			Source:    entity.SourceDaily,
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
			Checksum:  "checksum",
		}
		err := store.Store(context.Background(), memory)
		assert.NoError(t, err, "SQLite store accepts empty content")
	})
}

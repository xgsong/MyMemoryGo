package e2e

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
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

// mockEmbeddingProvider for e2e tests.
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

// E2ETestSuite represents an end-to-end test suite.
type E2ETestSuite struct {
	tmpDir        string
	workspaceDir  string
	dbPath        string
	store         *sqlite.Store
	fileMgr       *filestore.Manager
	embedProvider *mockEmbeddingProvider
	server        *api.Server
	testServer    *httptest.Server
}

// Setup initializes the e2e test environment.
func (s *E2ETestSuite) Setup(t *testing.T) {
	t.Helper()

	s.tmpDir = t.TempDir()
	s.workspaceDir = filepath.Join(s.tmpDir, "workspace")
	s.dbPath = filepath.Join(s.tmpDir, "test.db")

	err := os.MkdirAll(s.workspaceDir, 0755)
	require.NoError(t, err)

	s.embedProvider = newMockEmbeddingProvider()

	s.store, err = sqlite.New(&sqlite.Config{
		DBPath:           s.dbPath,
		WALMode:          true,
		VectorDimensions: 768,
	})
	require.NoError(t, err)

	s.fileMgr, err = filestore.New(&filestore.Config{
		WorkspaceDir:  s.workspaceDir,
		LongTermFile:  "MEMORY.md",
		DailyDir:      "memory",
		FileExtension: ".md",
	})
	require.NoError(t, err)

	memoryApp := service.NewMemoryApplicationService(
		s.store,
		s.store,
		s.embedProvider,
		s.fileMgr,
	)

	s.server = api.NewServer(memoryApp)
	s.testServer = httptest.NewServer(s.server.Handler())
}

// Teardown cleans up e2e test resources.
func (s *E2ETestSuite) Teardown(t *testing.T) {
	t.Helper()
	if s.testServer != nil {
		s.testServer.Close()
	}
	if s.store != nil {
		s.store.Close()
	}
	if s.fileMgr != nil {
		s.fileMgr.Close()
	}
}

// TestE2E_MemoryLifecycle tests complete memory lifecycle.
func TestE2E_MemoryLifecycle(t *testing.T) {
	suite := &E2ETestSuite{}
	suite.Setup(t)
	defer suite.Teardown(t)

	client := &http.Client{}

	t.Run("CreateReadUpdateDelete", func(t *testing.T) {
		createPayload := map[string]interface{}{
			"content": "Important project meeting notes",
			"source":  "daily",
		}
		bodyBytes, _ := json.Marshal(createPayload)

		resp, err := client.Post(
			suite.testServer.URL+"/api/v1/memories",
			"application/json",
			bytes.NewReader(bodyBytes),
		)
		require.NoError(t, err)
		defer resp.Body.Close()
		assert.Equal(t, http.StatusCreated, resp.StatusCode)

		var created map[string]interface{}
		body, _ := io.ReadAll(resp.Body)
		json.Unmarshal(body, &created)
		memoryID := created["id"].(string)
		assert.NotEmpty(t, memoryID)

		encodedID := url.PathEscape(memoryID)
		resp, err = client.Get(suite.testServer.URL + "/api/v1/memories/" + encodedID)
		require.NoError(t, err)
		defer resp.Body.Close()
		assert.Equal(t, http.StatusOK, resp.StatusCode)

		var retrieved map[string]interface{}
		body, _ = io.ReadAll(resp.Body)
		json.Unmarshal(body, &retrieved)
		assert.Equal(t, memoryID, retrieved["id"])
		assert.Equal(t, "Important project meeting notes", retrieved["content"])

		req, _ := http.NewRequest("DELETE", suite.testServer.URL+"/api/v1/memories/"+encodedID, nil)
		resp, err = client.Do(req)
		require.NoError(t, err)
		if resp != nil {
			defer resp.Body.Close()
			assert.Equal(t, http.StatusOK, resp.StatusCode)
		}

		resp, err = client.Get(suite.testServer.URL + "/api/v1/memories/" + encodedID)
		require.NoError(t, err)
		defer resp.Body.Close()
		assert.Equal(t, http.StatusNotFound, resp.StatusCode)
	})
}

func mustParseURL(urlStr string) *url.URL {
	u, _ := url.Parse(urlStr)
	return u
}

// TestE2E_HybridSearch tests hybrid search functionality.
func TestE2E_HybridSearch(t *testing.T) {
	suite := &E2ETestSuite{}
	suite.Setup(t)
	defer suite.Teardown(t)

	client := &http.Client{}

	t.Run("StoreMultipleMemories", func(t *testing.T) {
		memories := []map[string]interface{}{
			{"content": "Machine learning model training completed", "source": "daily"},
			{"content": "Team standup meeting discussion", "source": "daily"},
			{"content": "Code review feedback for PR #123", "source": "session"},
			{"content": "Project architecture documentation", "source": "longterm"},
			{"content": "Database optimization strategies", "source": "daily"},
		}

		for _, mem := range memories {
			bodyBytes, _ := json.Marshal(mem)
			resp, err := client.Post(
				suite.testServer.URL+"/api/v1/memories",
				"application/json",
				bytes.NewReader(bodyBytes),
			)
			require.NoError(t, err)
			defer resp.Body.Close()
			assert.Equal(t, http.StatusCreated, resp.StatusCode)
		}
	})

	t.Run("SearchByKeyword", func(t *testing.T) {
		searchPayload := map[string]interface{}{
			"query": "meeting",
			"limit": 10,
		}
		bodyBytes, _ := json.Marshal(searchPayload)

		resp, err := client.Post(
			suite.testServer.URL+"/api/v1/search",
			"application/json",
			bytes.NewReader(bodyBytes),
		)
		require.NoError(t, err)
		defer resp.Body.Close()
		assert.Equal(t, http.StatusOK, resp.StatusCode)

		var result map[string]interface{}
		body, _ := io.ReadAll(resp.Body)
		json.Unmarshal(body, &result)

		assert.NotNil(t, result["hits"])
		hits := result["hits"].([]interface{})
		assert.Greater(t, len(hits), 0)
	})

	t.Run("SearchWithFilter", func(t *testing.T) {
		searchPayload := map[string]interface{}{
			"query":  "code",
			"limit":  10,
			"source": []string{"session"},
		}
		bodyBytes, _ := json.Marshal(searchPayload)

		resp, err := client.Post(
			suite.testServer.URL+"/api/v1/search",
			"application/json",
			bytes.NewReader(bodyBytes),
		)
		require.NoError(t, err)
		defer resp.Body.Close()
		assert.Equal(t, http.StatusOK, resp.StatusCode)
	})
}

// TestE2E_BatchOperations tests batch processing.
func TestE2E_BatchOperations(t *testing.T) {
	suite := &E2ETestSuite{}
	suite.Setup(t)
	defer suite.Teardown(t)

	t.Run("StoreAndListBatch", func(t *testing.T) {
		ctx := context.Background()
		memoryApp := service.NewMemoryApplicationService(
			suite.store,
			suite.store,
			suite.embedProvider,
			suite.fileMgr,
		)

		for i := 0; i < 20; i++ {
			_, err := memoryApp.StoreMemory(ctx, &service.StoreMemoryRequest{
				Content:  fmt.Sprintf("Batch memory item %d", i),
				Source:   entity.SourceDaily,
				Metadata: map[string]string{"batch": fmt.Sprintf("%d", i)},
			})
			require.NoError(t, err)
		}

		result, err := memoryApp.ListMemories(ctx, &service.ListMemoriesRequest{})
		require.NoError(t, err)
		assert.GreaterOrEqual(t, result.Total, 1)
		assert.GreaterOrEqual(t, len(result.Memories), 1)
	})
}

// TestE2E_ErrorScenarios tests error handling.
func TestE2E_ErrorScenarios(t *testing.T) {
	suite := &E2ETestSuite{}
	suite.Setup(t)
	defer suite.Teardown(t)

	client := &http.Client{}

	t.Run("EmptyContent", func(t *testing.T) {
		payload := map[string]interface{}{
			"content": "",
			"source":  "daily",
		}
		bodyBytes, _ := json.Marshal(payload)

		resp, err := client.Post(
			suite.testServer.URL+"/api/v1/memories",
			"application/json",
			bytes.NewReader(bodyBytes),
		)
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	})

	t.Run("MissingContent", func(t *testing.T) {
		payload := map[string]interface{}{
			"source": "daily",
		}
		bodyBytes, _ := json.Marshal(payload)

		resp, err := client.Post(
			suite.testServer.URL+"/api/v1/memories",
			"application/json",
			bytes.NewReader(bodyBytes),
		)
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	})

	t.Run("GetNonExistent", func(t *testing.T) {
		resp, err := client.Get(suite.testServer.URL + "/api/v1/memories/nonexistent:1-5")
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusNotFound, resp.StatusCode)
	})
}

// TestE2E_ConcurrentAccess tests concurrent API access.
func TestE2E_ConcurrentAccess(t *testing.T) {
	suite := &E2ETestSuite{}
	suite.Setup(t)
	defer suite.Teardown(t)

	t.Run("ConcurrentStores", func(t *testing.T) {
		done := make(chan bool, 10)

		for i := 0; i < 10; i++ {
			go func(id int) {
				client := &http.Client{}
				payload := map[string]interface{}{
					"content": fmt.Sprintf("Concurrent memory %d", id),
					"source":  "daily",
				}
				bodyBytes, _ := json.Marshal(payload)

				resp, err := client.Post(
					suite.testServer.URL+"/api/v1/memories",
					"application/json",
					bytes.NewReader(bodyBytes),
				)
				assert.NoError(t, err)
				if resp != nil {
					defer resp.Body.Close()
					assert.Equal(t, http.StatusCreated, resp.StatusCode)
				}
				done <- true
			}(i)
		}

		for i := 0; i < 10; i++ {
			<-done
		}

		ctx := context.Background()
		memoryApp := service.NewMemoryApplicationService(
			suite.store,
			suite.store,
			suite.embedProvider,
			suite.fileMgr,
		)

		result, err := memoryApp.ListMemories(ctx, &service.ListMemoriesRequest{})
		require.NoError(t, err)
		assert.GreaterOrEqual(t, result.Total, 1)
	})
}

// TestE2E_FileOperations tests file-based operations.
func TestE2E_FileOperations(t *testing.T) {
	suite := &E2ETestSuite{}
	suite.Setup(t)
	defer suite.Teardown(t)

	t.Run("WriteAndReadFile", func(t *testing.T) {
		ctx := context.Background()
		testContent := []byte("# Test File\n\nThis is a test file content.")

		err := suite.fileMgr.Write(ctx, "test.md", testContent)
		require.NoError(t, err)

		readContent, err := suite.fileMgr.Read(ctx, "test.md")
		require.NoError(t, err)
		assert.Equal(t, testContent, readContent)
	})

	t.Run("ListFiles", func(t *testing.T) {
		files, err := suite.fileMgr.List(context.Background(), "*.md")
		require.NoError(t, err)
		assert.GreaterOrEqual(t, len(files), 1)
	})
}

// TestE2E_MemorySourceTypes tests different memory source types.
func TestE2E_MemorySourceTypes(t *testing.T) {
	suite := &E2ETestSuite{}
	suite.Setup(t)
	defer suite.Teardown(t)

	ctx := context.Background()
	memoryApp := service.NewMemoryApplicationService(
		suite.store,
		suite.store,
		suite.embedProvider,
		suite.fileMgr,
	)

	t.Run("LongTermMemory", func(t *testing.T) {
		resp, err := memoryApp.StoreMemory(ctx, &service.StoreMemoryRequest{
			Content: "Long-term knowledge base",
			Source:  entity.SourceLongTerm,
		})
		require.NoError(t, err)
		assert.Equal(t, entity.SourceLongTerm, resp.Memory.Source)
	})

	t.Run("DailyMemory", func(t *testing.T) {
		resp, err := memoryApp.StoreMemory(ctx, &service.StoreMemoryRequest{
			Content: "Daily journal entry",
			Source:  entity.SourceDaily,
		})
		require.NoError(t, err)
		assert.Equal(t, entity.SourceDaily, resp.Memory.Source)
	})

	t.Run("SessionMemory", func(t *testing.T) {
		resp, err := memoryApp.StoreMemory(ctx, &service.StoreMemoryRequest{
			Content: "Session-specific notes",
			Source:  entity.SourceSession,
		})
		require.NoError(t, err)
		assert.Equal(t, entity.SourceSession, resp.Memory.Source)
	})

	t.Run("ListBySourceType", func(t *testing.T) {
		result, err := memoryApp.ListMemories(ctx, &service.ListMemoriesRequest{
			Source: entity.SourceDaily,
		})
		require.NoError(t, err)
		for _, mem := range result.Memories {
			assert.Equal(t, entity.SourceDaily, mem.Source)
		}
	})
}

// TestE2E_MetadataOperations tests metadata handling.
func TestE2E_MetadataOperations(t *testing.T) {
	suite := &E2ETestSuite{}
	suite.Setup(t)
	defer suite.Teardown(t)

	ctx := context.Background()
	memoryApp := service.NewMemoryApplicationService(
		suite.store,
		suite.store,
		suite.embedProvider,
		suite.fileMgr,
	)

	t.Run("StoreWithMetadata", func(t *testing.T) {
		resp, err := memoryApp.StoreMemory(ctx, &service.StoreMemoryRequest{
			Content: "Memory with metadata",
			Source:  entity.SourceDaily,
			Metadata: map[string]string{
				"project": "MyMemoryGo",
				"tag":     "important",
			},
		})
		require.NoError(t, err)
		assert.Equal(t, "MyMemoryGo", resp.Memory.Metadata["project"])
		assert.Equal(t, "important", resp.Memory.Metadata["tag"])
	})
}

// TestE2E_Performance tests basic performance scenarios.
func TestE2E_Performance(t *testing.T) {
	suite := &E2ETestSuite{}
	suite.Setup(t)
	defer suite.Teardown(t)

	ctx := context.Background()
	memoryApp := service.NewMemoryApplicationService(
		suite.store,
		suite.store,
		suite.embedProvider,
		suite.fileMgr,
	)

	t.Run("BulkStorePerformance", func(t *testing.T) {
		start := time.Now()

		for i := 0; i < 50; i++ {
			_, err := memoryApp.StoreMemory(ctx, &service.StoreMemoryRequest{
				Content: fmt.Sprintf("Performance test memory %d", i),
				Source:  entity.SourceDaily,
			})
			require.NoError(t, err)
		}

		duration := time.Since(start)
		t.Logf("Stored 50 memories in %v", duration)
		assert.Less(t, duration, 10*time.Second)
	})

	t.Run("SearchPerformance", func(t *testing.T) {
		start := time.Now()

		resp, err := memoryApp.SearchMemories(ctx, &service.SearchMemoriesRequest{
			Query: "performance",
			Limit: 10,
		})
		require.NoError(t, err)

		duration := time.Since(start)
		t.Logf("Search completed in %v, found %d results", duration, len(resp.Results.Hits))
		assert.Less(t, duration, 5*time.Second)
	})
}

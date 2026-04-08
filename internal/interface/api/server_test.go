// Package api_test tests the REST API server.
package api_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/xgsong/MyMemoryGo/internal/application/service"
	"github.com/xgsong/MyMemoryGo/internal/infrastructure/persistence/filestore"
	"github.com/xgsong/MyMemoryGo/internal/infrastructure/persistence/sqlite"
	"github.com/xgsong/MyMemoryGo/internal/interface/api"
)

// mockEmbeddingProvider is a mock implementation of the embedding repository for testing.
type mockEmbeddingProvider struct {
	embeddings map[string][]float32
}

func newMockEmbeddingProvider() *mockEmbeddingProvider {
	return &mockEmbeddingProvider{
		embeddings: make(map[string][]float32),
	}
}

func (m *mockEmbeddingProvider) Embed(ctx context.Context, text string) ([]float32, error) {
	// Return cached embedding if exists
	if emb, ok := m.embeddings[text]; ok {
		return emb, nil
	}
	// Generate a deterministic mock embedding based on text length
	embedding := make([]float32, 768)
	for i := range embedding {
		embedding[i] = float32((len(text)+i)%100) / 100.0
	}
	m.embeddings[text] = embedding
	return embedding, nil
}

func (m *mockEmbeddingProvider) EmbedBatch(ctx context.Context, texts []string) ([][]float32, error) {
	embeddings := make([][]float32, len(texts))
	for i, text := range texts {
		emb, err := m.Embed(ctx, text)
		if err != nil {
			return nil, err
		}
		embeddings[i] = emb
	}
	return embeddings, nil
}

func (m *mockEmbeddingProvider) Model() string {
	return "mock-embedding-model"
}

func (m *mockEmbeddingProvider) Dimensions() int {
	return 768
}

// TestHealthCheck tests the /health endpoint.
func TestHealthCheck(t *testing.T) {
	server := setupTestServer(t)
	defer cleanupTestServer(t, server)

	req := httptest.NewRequest("GET", "/health", nil)
	rec := httptest.NewRecorder()

	server.Handler().ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)

	var resp map[string]interface{}
	err := json.Unmarshal(rec.Body.Bytes(), &resp)
	require.NoError(t, err)

	assert.Equal(t, "ok", resp["status"])
}

// TestReadinessCheck tests the /ready endpoint.
func TestReadinessCheck(t *testing.T) {
	server := setupTestServer(t)
	defer cleanupTestServer(t, server)

	req := httptest.NewRequest("GET", "/ready", nil)
	rec := httptest.NewRecorder()

	server.Handler().ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)

	var resp map[string]interface{}
	err := json.Unmarshal(rec.Body.Bytes(), &resp)
	require.NoError(t, err)

	assert.Equal(t, "ready", resp["status"])
}

// TestStoreMemory tests POST /api/v1/memories.
func TestStoreMemory(t *testing.T) {
	server := setupTestServer(t)
	defer cleanupTestServer(t, server)

	// Create request
	body := map[string]interface{}{
		"content": "Test memory content",
		"source":  "daily",
	}
	bodyBytes, err := json.Marshal(body)
	require.NoError(t, err)

	req := httptest.NewRequest("POST", "/api/v1/memories", bytes.NewReader(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	server.Handler().ServeHTTP(rec, req)

	t.Logf("Response status: %d", rec.Code)
	t.Logf("Response body: %s", rec.Body.String())

	assert.Equal(t, http.StatusCreated, rec.Code)

	var resp map[string]interface{}
	err = json.Unmarshal(rec.Body.Bytes(), &resp)
	require.NoError(t, err)

	assert.NotEmpty(t, resp["id"])
	assert.Equal(t, "Test memory content", resp["content"])
}

// TestGetMemory tests GET /api/v1/memories/:id.
func TestGetMemory(t *testing.T) {
	server := setupTestServer(t)
	defer cleanupTestServer(t, server)

	// First store a memory via API
	body := map[string]interface{}{
		"content": "Test memory for retrieval",
		"source":  "daily",
	}
	bodyBytes, err := json.Marshal(body)
	require.NoError(t, err)

	req := httptest.NewRequest("POST", "/api/v1/memories", bytes.NewReader(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	server.Handler().ServeHTTP(rec, req)

	// Check if store was successful
	if rec.Code != http.StatusCreated {
		t.Skipf("Store memory failed with status %d, skipping get test", rec.Code)
	}

	var storeResp map[string]interface{}
	err = json.Unmarshal(rec.Body.Bytes(), &storeResp)
	require.NoError(t, err)

	memoryID, ok := storeResp["id"].(string)
	if !ok || memoryID == "" {
		t.Skip("Memory ID not found in response, skipping get test")
	}

	// Now get the memory (URL encode the ID which contains ':' character)
	t.Logf("Getting memory with ID: %s", memoryID)
	encodedID := url.PathEscape(memoryID)
	req = httptest.NewRequest("GET", "/api/v1/memories/"+encodedID, nil)
	rec = httptest.NewRecorder()

	server.Handler().ServeHTTP(rec, req)

	t.Logf("Get response status: %d", rec.Code)
	t.Logf("Get response body: %s", rec.Body.String())

	assert.Equal(t, http.StatusOK, rec.Code)

	var resp map[string]interface{}
	err = json.Unmarshal(rec.Body.Bytes(), &resp)
	require.NoError(t, err)

	assert.Equal(t, memoryID, resp["id"])
}

// TestListMemories tests GET /api/v1/memories.
func TestListMemories(t *testing.T) {
	server := setupTestServer(t)
	defer cleanupTestServer(t, server)

	// Store some test memories via API
	for i := 0; i < 3; i++ {
		body := map[string]interface{}{
			"content": "Test memory",
			"source":  "daily",
		}
		bodyBytes, _ := json.Marshal(body)
		req := httptest.NewRequest("POST", "/api/v1/memories", bytes.NewReader(bodyBytes))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		server.Handler().ServeHTTP(rec, req)
	}

	// List memories
	req := httptest.NewRequest("GET", "/api/v1/memories?limit=10", nil)
	rec := httptest.NewRecorder()

	server.Handler().ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)

	var resp map[string]interface{}
	err := json.Unmarshal(rec.Body.Bytes(), &resp)
	require.NoError(t, err)

	assert.NotNil(t, resp["memories"])
	assert.NotNil(t, resp["total"])
}

// TestDeleteMemory tests DELETE /api/v1/memories/:id.
func TestDeleteMemory(t *testing.T) {
	server := setupTestServer(t)
	defer cleanupTestServer(t, server)

	// First store a memory via API
	body := map[string]interface{}{
		"content": "Test memory to delete",
		"source":  "daily",
	}
	bodyBytes, _ := json.Marshal(body)
	req := httptest.NewRequest("POST", "/api/v1/memories", bytes.NewReader(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	server.Handler().ServeHTTP(rec, req)

	var storeResp map[string]interface{}
	json.Unmarshal(rec.Body.Bytes(), &storeResp)
	memoryID := storeResp["id"].(string)

	req = httptest.NewRequest("DELETE", "/api/v1/memories/"+url.PathEscape(memoryID), nil)
	rec = httptest.NewRecorder()

	server.Handler().ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)

	// Verify memory is deleted
	req = httptest.NewRequest("GET", "/api/v1/memories/"+memoryID, nil)
	rec = httptest.NewRecorder()

	server.Handler().ServeHTTP(rec, req)

	assert.Equal(t, http.StatusNotFound, rec.Code)
}

// TestSearchMemories tests POST /api/v1/search.
func TestSearchMemories(t *testing.T) {
	server := setupTestServer(t)
	defer cleanupTestServer(t, server)

	// Store a test memory via API
	body := map[string]interface{}{
		"content": "Important meeting notes",
		"source":  "daily",
	}
	bodyBytes, _ := json.Marshal(body)
	req := httptest.NewRequest("POST", "/api/v1/memories", bytes.NewReader(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	server.Handler().ServeHTTP(rec, req)

	// Search for memories
	body = map[string]interface{}{
		"query": "meeting",
		"limit": 10,
	}
	bodyBytes, _ = json.Marshal(body)

	req = httptest.NewRequest("POST", "/api/v1/search", bytes.NewReader(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	rec = httptest.NewRecorder()

	server.Handler().ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)

	var resp map[string]interface{}
	err := json.Unmarshal(rec.Body.Bytes(), &resp)
	require.NoError(t, err)

	assert.NotNil(t, resp["hits"])
}

// TestSyncIndex tests POST /api/v1/sync.
func TestSyncIndex(t *testing.T) {
	server := setupTestServer(t)
	defer cleanupTestServer(t, server)

	req := httptest.NewRequest("POST", "/api/v1/sync", nil)
	rec := httptest.NewRecorder()

	server.Handler().ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)

	var resp map[string]interface{}
	err := json.Unmarshal(rec.Body.Bytes(), &resp)
	require.NoError(t, err)

	assert.Equal(t, "index synchronized", resp["message"])
}

// setupTestServer creates a test API server.
func setupTestServer(t *testing.T) *api.Server {
	t.Helper()

	// Create temporary directory
	tmpDir := t.TempDir()
	workspaceDir := filepath.Join(tmpDir, "workspace")
	dbPath := filepath.Join(tmpDir, "test.db")

	// Create workspace directory
	err := os.MkdirAll(workspaceDir, 0755)
	require.NoError(t, err)

	// Initialize mock embedding provider (no external service required)
	embedProvider := newMockEmbeddingProvider()

	// Initialize SQLite store
	storeConfig := &sqlite.Config{
		DBPath:           dbPath,
		WALMode:          true,
		VectorDimensions: 768,
	}
	store, err := sqlite.New(storeConfig)
	require.NoError(t, err)

	// Initialize file manager
	fileConfig := &filestore.Config{
		WorkspaceDir:  workspaceDir,
		LongTermFile:  "MEMORY.md",
		DailyDir:      "memory",
		FileExtension: ".md",
	}
	fileMgr, err := filestore.New(fileConfig)
	require.NoError(t, err)

	// Initialize application service
	memoryApp := service.NewMemoryApplicationService(
		store,
		store, // SearchRepository (same implementation)
		embedProvider,
		fileMgr,
	)

	// Create API server
	server := api.NewServer(memoryApp)

	// Store references for cleanup
	t.Cleanup(func() {
		store.Close()
		fileMgr.Close()
	})

	return server
}

// cleanupTestServer cleans up test server resources.
func cleanupTestServer(t *testing.T, server *api.Server) {
	// Cleanup is handled by t.Cleanup in setupTestServer
}

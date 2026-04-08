package embedding

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockProvider is a mock embedding provider for testing.
type mockProvider struct {
	embeddings map[string][]float32
	callCount  int
}

func newMockProvider() *mockProvider {
	return &mockProvider{
		embeddings: make(map[string][]float32),
	}
}

func (m *mockProvider) Embed(ctx context.Context, text string) ([]float32, error) {
	m.callCount++
	if emb, ok := m.embeddings[text]; ok {
		return emb, nil
	}
	embedding := []float32{float32(len(text))}
	m.embeddings[text] = embedding
	return embedding, nil
}

func (m *mockProvider) EmbedBatch(ctx context.Context, texts []string) ([][]float32, error) {
	result := make([][]float32, len(texts))
	for i, text := range texts {
		emb, err := m.Embed(ctx, text)
		if err != nil {
			return nil, err
		}
		result[i] = emb
	}
	return result, nil
}

func (m *mockProvider) Model() string {
	return "mock-model"
}

func (m *mockProvider) Dimensions() int {
	return 1
}

func TestCachedProvider_Embed(t *testing.T) {
	mock := newMockProvider()
	cached := NewCachedProvider(mock, 100)

	ctx := context.Background()

	// First call - cache miss
	emb1, err := cached.Embed(ctx, "test")
	require.NoError(t, err)
	assert.Equal(t, []float32{4}, emb1)
	assert.Equal(t, 1, mock.callCount)

	// Second call - cache hit
	emb2, err := cached.Embed(ctx, "test")
	require.NoError(t, err)
	assert.Equal(t, []float32{4}, emb2)
	assert.Equal(t, 1, mock.callCount) // Should not increase

	// Different text - cache miss
	emb3, err := cached.Embed(ctx, "test2")
	require.NoError(t, err)
	assert.Equal(t, []float32{5}, emb3)
	assert.Equal(t, 2, mock.callCount)
}

func TestCachedProvider_EmbedBatch(t *testing.T) {
	mock := newMockProvider()
	cached := NewCachedProvider(mock, 100)

	ctx := context.Background()

	// First call - cache miss
	embeddings1, err := cached.EmbedBatch(ctx, []string{"a", "b", "c"})
	require.NoError(t, err)
	require.Len(t, embeddings1, 3)
	assert.Equal(t, 3, mock.callCount)

	// Second call - all cache hits
	embeddings2, err := cached.EmbedBatch(ctx, []string{"a", "b", "c"})
	require.NoError(t, err)
	assert.Equal(t, embeddings1, embeddings2)
	assert.Equal(t, 3, mock.callCount) // Should not increase

	// Partial cache hit
	embeddings3, err := cached.EmbedBatch(ctx, []string{"a", "d"})
	require.NoError(t, err)
	require.Len(t, embeddings3, 2)
	assert.Equal(t, 4, mock.callCount) // Only "d" should be a miss
}

func TestCachedProvider_Len(t *testing.T) {
	mock := newMockProvider()
	cached := NewCachedProvider(mock, 100)

	ctx := context.Background()

	assert.Equal(t, 0, cached.Len())

	_, _ = cached.Embed(ctx, "a")
	assert.Equal(t, 1, cached.Len())

	_, _ = cached.Embed(ctx, "b")
	assert.Equal(t, 2, cached.Len())
}

func TestCachedProvider_Clear(t *testing.T) {
	mock := newMockProvider()
	cached := NewCachedProvider(mock, 100)

	ctx := context.Background()

	_, _ = cached.Embed(ctx, "a")
	_, _ = cached.Embed(ctx, "b")
	assert.Equal(t, 2, cached.Len())

	cached.Clear()
	assert.Equal(t, 0, cached.Len())
}

func TestCachedProvider_ModelAndDimensions(t *testing.T) {
	mock := newMockProvider()
	cached := NewCachedProvider(mock, 100)

	assert.Equal(t, "mock-model", cached.Model())
	assert.Equal(t, 1, cached.Dimensions())
}

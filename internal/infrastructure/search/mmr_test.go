package search_test

import (
	"math"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/xgsong/MyMemoryGo/internal/domain/entity"
	"github.com/xgsong/MyMemoryGo/internal/infrastructure/search"
	"github.com/xgsong/MyMemoryGo/internal/pkg/vector"
)

func TestNewMMRReranker(t *testing.T) {
	reranker := search.NewMMRReranker()
	assert.NotNil(t, reranker)
	// Verify Embeddings map is usable (sync.Map can't be copied, test via operation)
	_, ok := reranker.Embeddings.Load("nonexistent")
	assert.False(t, ok) // empty map, should not find anything
}

func TestMMRReranker_SetEmbedding(t *testing.T) {
	reranker := search.NewMMRReranker()

	embedding := []float32{0.1, 0.2, 0.3}
	reranker.SetEmbedding("test-id", embedding)

	// Instead of testing internal storage, test that reranking works
	hits := []*entity.SearchHit{
		{Entry: &entity.Entry{ID: "test-id", Score: 0.9}},
	}
	result := reranker.Rerank(hits, 0.7)
	assert.Len(t, result, 1)
}

func TestMMRReranker_Rerank_Empty(t *testing.T) {
	reranker := search.NewMMRReranker()

	result := reranker.Rerank([]*entity.SearchHit{}, 0.7)
	assert.Empty(t, result)

	result = reranker.Rerank(nil, 0.7)
	assert.Nil(t, result)
}

func TestMMRReranker_Rerank_SingleHit(t *testing.T) {
	reranker := search.NewMMRReranker()
	reranker.SetEmbedding("hit1", []float32{1.0, 0.0, 0.0})

	hits := []*entity.SearchHit{
		{Entry: &entity.Entry{ID: "hit1", Score: 0.9}},
	}

	result := reranker.Rerank(hits, 0.7)
	assert.Len(t, result, 1)
	assert.Equal(t, "hit1", result[0].ID)
}

func TestMMRReranker_Rerank_LambdaOne(t *testing.T) {
	reranker := search.NewMMRReranker()

	hits := []*entity.SearchHit{
		{Entry: &entity.Entry{ID: "hit1", Score: 0.9}},
		{Entry: &entity.Entry{ID: "hit2", Score: 0.8}},
		{Entry: &entity.Entry{ID: "hit3", Score: 0.7}},
	}

	result := reranker.Rerank(hits, 1.0)
	assert.Len(t, result, 3)
	assert.Equal(t, "hit1", result[0].ID)
	assert.Equal(t, "hit2", result[1].ID)
	assert.Equal(t, "hit3", result[2].ID)
}

func TestMMRReranker_Rerank_LambdaZero(t *testing.T) {
	reranker := search.NewMMRReranker()

	reranker.SetEmbedding("hit1", []float32{1.0, 0.0, 0.0})
	reranker.SetEmbedding("hit2", []float32{0.0, 1.0, 0.0})
	reranker.SetEmbedding("hit3", []float32{0.0, 0.0, 1.0})

	hits := []*entity.SearchHit{
		{Entry: &entity.Entry{ID: "hit1", Score: 0.9}},
		{Entry: &entity.Entry{ID: "hit2", Score: 0.8}},
		{Entry: &entity.Entry{ID: "hit3", Score: 0.7}},
	}

	result := reranker.Rerank(hits, 0.0)
	assert.Len(t, result, 3)
	assert.Equal(t, "hit1", result[0].ID)
}

func TestMMRReranker_Rerank_Diversity(t *testing.T) {
	reranker := search.NewMMRReranker()

	reranker.SetEmbedding("hit1", []float32{1.0, 0.0, 0.0})
	reranker.SetEmbedding("hit2", []float32{0.9, 0.1, 0.0})
	reranker.SetEmbedding("hit3", []float32{0.0, 0.0, 1.0})

	hits := []*entity.SearchHit{
		{Entry: &entity.Entry{ID: "hit1", Score: 0.9}},
		{Entry: &entity.Entry{ID: "hit2", Score: 0.85}},
		{Entry: &entity.Entry{ID: "hit3", Score: 0.8}},
	}

	result := reranker.Rerank(hits, 0.5)
	assert.Len(t, result, 3)
	assert.Equal(t, "hit1", result[0].ID)
}

func TestMMRReranker_Rerank_NoEmbeddings(t *testing.T) {
	reranker := search.NewMMRReranker()

	hits := []*entity.SearchHit{
		{Entry: &entity.Entry{ID: "hit1", Score: 0.9}},
		{Entry: &entity.Entry{ID: "hit2", Score: 0.8}},
		{Entry: &entity.Entry{ID: "hit3", Score: 0.7}},
	}

	result := reranker.Rerank(hits, 0.7)
	assert.Len(t, result, 3)
	assert.Equal(t, "hit1", result[0].ID)
}

func TestCosineSimilarity(t *testing.T) {
	tests := []struct {
		name     string
		a        []float32
		b        []float32
		expected float64
		delta    float64
	}{
		{
			name:     "identical vectors",
			a:        []float32{1.0, 0.0, 0.0},
			b:        []float32{1.0, 0.0, 0.0},
			expected: 1.0,
			delta:    0.001,
		},
		{
			name:     "orthogonal vectors",
			a:        []float32{1.0, 0.0, 0.0},
			b:        []float32{0.0, 1.0, 0.0},
			expected: 0.0,
			delta:    0.001,
		},
		{
			name:     "opposite vectors",
			a:        []float32{1.0, 0.0, 0.0},
			b:        []float32{-1.0, 0.0, 0.0},
			expected: -1.0,
			delta:    0.001,
		},
		{
			name:     "45 degree angle",
			a:        []float32{1.0, 0.0, 0.0},
			b:        []float32{1.0, 1.0, 0.0},
			expected: 0.707,
			delta:    0.01,
		},
		{
			name:     "different lengths",
			a:        []float32{1.0, 0.0},
			b:        []float32{1.0, 0.0, 0.0},
			expected: 0.0,
			delta:    0.001,
		},
		{
			name:     "zero vector a",
			a:        []float32{0.0, 0.0, 0.0},
			b:        []float32{1.0, 0.0, 0.0},
			expected: 0.0,
			delta:    0.001,
		},
		{
			name:     "zero vector b",
			a:        []float32{1.0, 0.0, 0.0},
			b:        []float32{0.0, 0.0, 0.0},
			expected: 0.0,
			delta:    0.001,
		},
		{
			name:     "both zero vectors",
			a:        []float32{0.0, 0.0, 0.0},
			b:        []float32{0.0, 0.0, 0.0},
			expected: 0.0,
			delta:    0.001,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := vector.CosineSimilarity(tt.a, tt.b)
			assert.InDelta(t, tt.expected, result, tt.delta)
		})
	}
}

func TestEuclideanDistance(t *testing.T) {
	tests := []struct {
		name     string
		a        []float32
		b        []float32
		expected float64
		delta    float64
	}{
		{
			name:     "identical vectors",
			a:        []float32{1.0, 2.0, 3.0},
			b:        []float32{1.0, 2.0, 3.0},
			expected: 0.0,
			delta:    0.001,
		},
		{
			name:     "unit distance",
			a:        []float32{0.0, 0.0, 0.0},
			b:        []float32{1.0, 0.0, 0.0},
			expected: 1.0,
			delta:    0.001,
		},
		{
			name:     "3D diagonal",
			a:        []float32{0.0, 0.0, 0.0},
			b:        []float32{1.0, 1.0, 1.0},
			expected: math.Sqrt(3),
			delta:    0.001,
		},
		{
			name:     "different lengths",
			a:        []float32{1.0, 0.0},
			b:        []float32{1.0, 0.0, 0.0},
			expected: math.Inf(1),
			delta:    0.001,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := vector.EuclideanDistance(tt.a, tt.b)
			assert.InDelta(t, tt.expected, result, tt.delta)
		})
	}
}

func TestNormalizeVector(t *testing.T) {
	tests := []struct {
		name     string
		input    []float32
		expected []float32
		delta    float64
	}{
		{
			name:     "unit vector",
			input:    []float32{1.0, 0.0, 0.0},
			expected: []float32{1.0, 0.0, 0.0},
			delta:    0.001,
		},
		{
			name:     "scale down",
			input:    []float32{2.0, 0.0, 0.0},
			expected: []float32{1.0, 0.0, 0.0},
			delta:    0.001,
		},
		{
			name:     "3D normalize",
			input:    []float32{1.0, 1.0, 1.0},
			expected: []float32{0.577, 0.577, 0.577},
			delta:    0.01,
		},
		{
			name:     "zero vector",
			input:    []float32{0.0, 0.0, 0.0},
			expected: []float32{0.0, 0.0, 0.0},
			delta:    0.001,
		},
		{
			name:     "empty vector",
			input:    []float32{},
			expected: []float32{},
			delta:    0.001,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := vector.Normalize(tt.input)
			if len(tt.input) == 0 {
				assert.Empty(t, result)
				return
			}
			require.Len(t, result, len(tt.expected))
			for i := range result {
				assert.InDelta(t, tt.expected[i], result[i], tt.delta)
			}
		})
	}
}

func TestNormalizeVector_PreservesDirection(t *testing.T) {
	tests := []struct {
		name  string
		input []float32
	}{
		{"positive quadrant", []float32{1.0, 1.0, 1.0}},
		{"negative quadrant", []float32{-1.0, -1.0, -1.0}},
		{"mixed signs", []float32{1.0, -2.0, 3.0}},
		{"large values", []float32{1000.0, 2000.0, 3000.0}},
		{"small values", []float32{0.001, 0.002, 0.003}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			normalized := vector.Normalize(tt.input)

			for i := range normalized {
				if tt.input[i] > 0 {
					assert.Greater(t, normalized[i], float32(0))
				} else if tt.input[i] < 0 {
					assert.Less(t, normalized[i], float32(0))
				} else {
					assert.Equal(t, float32(0), normalized[i])
				}
			}

			var norm float64
			for _, v := range normalized {
				norm += float64(v) * float64(v)
			}
			if len(normalized) > 0 && norm > 0 {
				assert.InDelta(t, 1.0, math.Sqrt(norm), 0.001)
			}
		})
	}
}

func TestMMRReranker_Rerank_MultipleIterations(t *testing.T) {
	reranker := search.NewMMRReranker()

	reranker.SetEmbedding("hit1", []float32{1.0, 0.0, 0.0})
	reranker.SetEmbedding("hit2", []float32{0.9, 0.1, 0.0})
	reranker.SetEmbedding("hit3", []float32{0.0, 1.0, 0.0})
	reranker.SetEmbedding("hit4", []float32{0.1, 0.9, 0.0})
	reranker.SetEmbedding("hit5", []float32{0.0, 0.0, 1.0})

	hits := []*entity.SearchHit{
		{Entry: &entity.Entry{ID: "hit1", Score: 0.95}},
		{Entry: &entity.Entry{ID: "hit2", Score: 0.90}},
		{Entry: &entity.Entry{ID: "hit3", Score: 0.85}},
		{Entry: &entity.Entry{ID: "hit4", Score: 0.80}},
		{Entry: &entity.Entry{ID: "hit5", Score: 0.75}},
	}

	result := reranker.Rerank(hits, 0.5)
	assert.Len(t, result, 5)

	ids := make(map[string]bool)
	for _, hit := range result {
		ids[hit.ID] = true
	}
	assert.True(t, ids["hit1"])
	assert.True(t, ids["hit2"])
	assert.True(t, ids["hit3"])
	assert.True(t, ids["hit4"])
	assert.True(t, ids["hit5"])
}

func TestMMRReranker_Rerank_WithNilEmbedding(t *testing.T) {
	reranker := search.NewMMRReranker()

	reranker.SetEmbedding("hit1", []float32{1.0, 0.0, 0.0})
	reranker.SetEmbedding("hit3", []float32{0.0, 1.0, 0.0})

	hits := []*entity.SearchHit{
		{Entry: &entity.Entry{ID: "hit1", Score: 0.9}},
		{Entry: &entity.Entry{ID: "hit2", Score: 0.8}},
		{Entry: &entity.Entry{ID: "hit3", Score: 0.7}},
	}

	result := reranker.Rerank(hits, 0.7)
	assert.Len(t, result, 3)
}

func BenchmarkMMRReranker_Rerank(b *testing.B) {
	reranker := search.NewMMRReranker()

	hits := make([]*entity.SearchHit, 100)
	for i := 0; i < 100; i++ {
		id := string(rune('a'+i%26)) + string(rune('a'+(i/26)%26))
		embedding := []float32{float32(i % 10), float32((i + 1) % 10), float32((i + 2) % 10)}
		reranker.SetEmbedding(id, embedding)
		hits[i] = &entity.SearchHit{
			Entry: &entity.Entry{
				ID:    id,
				Score: float64(100-i) / 100.0,
			},
		}
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		reranker.Rerank(hits, 0.7)
	}
}

func BenchmarkCosineSimilarity(b *testing.B) {
	a := []float32{1.0, 2.0, 3.0, 4.0, 5.0, 6.0, 7.0, 8.0, 9.0, 10.0}
	c := []float32{10.0, 9.0, 8.0, 7.0, 6.0, 5.0, 4.0, 3.0, 2.0, 1.0}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		vector.CosineSimilarity(a, c)
	}
}

func BenchmarkEuclideanDistance(b *testing.B) {
	a := []float32{1.0, 2.0, 3.0, 4.0, 5.0, 6.0, 7.0, 8.0, 9.0, 10.0}
	c := []float32{10.0, 9.0, 8.0, 7.0, 6.0, 5.0, 4.0, 3.0, 2.0, 1.0}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		vector.EuclideanDistance(a, c)
	}
}

func BenchmarkNormalizeVector(b *testing.B) {
	vec := []float32{1.0, 2.0, 3.0, 4.0, 5.0, 6.0, 7.0, 8.0, 9.0, 10.0}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		vector.Normalize(vec)
	}
}

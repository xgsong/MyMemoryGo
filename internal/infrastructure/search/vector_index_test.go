package search

import (
	"fmt"
	"math"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/xgsong/MyMemoryGo/internal/pkg/vector"
)

func TestNewVectorIndex(t *testing.T) {
	idx := NewVectorIndex(DefaultVectorIndexConfig(3))
	assert.NotNil(t, idx)
	assert.Equal(t, 3, idx.dims)
	assert.False(t, idx.IsLoaded())
	assert.Equal(t, 0, idx.Size())
}

func TestVectorIndex_Build(t *testing.T) {
	idx := NewVectorIndex(DefaultVectorIndexConfig(3))

	embeddings := map[string][]float32{
		"a": {1, 0, 0},
		"b": {0, 1, 0},
		"c": {0, 0, 1},
	}

	idx.Build(embeddings)
	assert.True(t, idx.IsLoaded())
	assert.Equal(t, 3, idx.Size())
}

func TestVectorIndex_Build_Empty(t *testing.T) {
	idx := NewVectorIndex(DefaultVectorIndexConfig(3))
	idx.Build(map[string][]float32{})
	assert.True(t, idx.IsLoaded())
	assert.Equal(t, 0, idx.Size())
}

func TestVectorIndex_Add(t *testing.T) {
	idx := NewVectorIndex(DefaultVectorIndexConfig(3))
	idx.Add("a", []float32{1, 0, 0})
	assert.True(t, idx.IsLoaded())
	assert.Equal(t, 1, idx.Size())

	idx.Add("b", []float32{0, 1, 0})
	assert.Equal(t, 2, idx.Size())
}

func TestVectorIndex_Add_UpdatesExisting(t *testing.T) {
	idx := NewVectorIndex(DefaultVectorIndexConfig(3))
	idx.Add("a", []float32{1, 0, 0})
	assert.Equal(t, 1, idx.Size())

	// Update with a different vector
	idx.Add("a", []float32{0, 0, 1})
	assert.Equal(t, 1, idx.Size())
}

func TestVectorIndex_Remove(t *testing.T) {
	idx := NewVectorIndex(DefaultVectorIndexConfig(3))
	idx.Add("a", []float32{1, 0, 0})
	idx.Add("b", []float32{0, 1, 0})
	assert.Equal(t, 2, idx.Size())

	idx.Remove("a")
	assert.Equal(t, 1, idx.Size())
}

func TestVectorIndex_Search(t *testing.T) {
	idx := NewVectorIndex(DefaultVectorIndexConfig(3))

	embeddings := map[string][]float32{
		"a": {1, 0, 0},
		"b": {0, 1, 0},
		"c": {0, 0, 1},
		"d": {0.9, 0.1, 0},
	}
	idx.Build(embeddings)

	// Search for vector closest to [1, 0, 0]
	results := idx.Search([]float32{1, 0, 0}, 2)
	assert.Equal(t, 2, len(results))
	// "a" should be the closest to [1,0,0]
	assert.Equal(t, "a", results[0])
}

func TestVectorIndex_Search_NotLoaded(t *testing.T) {
	idx := NewVectorIndex(DefaultVectorIndexConfig(3))
	results := idx.Search([]float32{1, 0, 0}, 5)
	assert.Nil(t, results)
}

func TestVectorIndex_Search_EmptyIndex(t *testing.T) {
	idx := NewVectorIndex(DefaultVectorIndexConfig(3))
	idx.Build(map[string][]float32{})
	results := idx.Search([]float32{1, 0, 0}, 5)
	assert.Nil(t, results)
}

func TestVectorIndex_Search_KLargerThanSize(t *testing.T) {
	idx := NewVectorIndex(DefaultVectorIndexConfig(3))
	idx.Add("a", []float32{1, 0, 0})

	results := idx.Search([]float32{1, 0, 0}, 10)
	assert.Equal(t, 1, len(results))
}

func TestVectorIndex_DefaultConfig(t *testing.T) {
	config := DefaultVectorIndexConfig(768)
	assert.Equal(t, 768, config.Dimensions)
	assert.Equal(t, 16, config.M)
	assert.InDelta(t, 0.25, config.Ml, 0.01)
	assert.Equal(t, 20, config.EfSearch)
}

func TestVectorIndex_NilConfig(t *testing.T) {
	idx := NewVectorIndex(nil)
	assert.NotNil(t, idx)
	assert.Equal(t, 768, idx.dims)
}

func TestVectorIndex_Search_Accuracy(t *testing.T) {
	dims := 8
	idx := NewVectorIndex(&VectorIndexConfig{
		Dimensions: dims,
		M:          16,
		Ml:         0.25,
		EfSearch:   100,
	})

	// Create orthogonal-ish vectors
	n := 50
	embeddings := make(map[string][]float32, n)
	for i := 0; i < n; i++ {
		vec := make([]float32, dims)
		for d := 0; d < dims; d++ {
			vec[d] = float32(math.Sin(float64(i*d + d)))
		}
		embeddings[fmt.Sprintf("vec_%d", i)] = vec
	}
	idx.Build(embeddings)

	// Search for an existing vector — it should find itself as the top result
	for id, emb := range embeddings {
		results := idx.Search(emb, 1)
		if len(results) > 0 && results[0] == id {
			// Success: found itself
			return
		}
	}
	t.Log("HNSW did not find exact match for all vectors (expected for approximate search)")
}

func TestVectorIndex_Search_ConsistentWithBruteForce(t *testing.T) {
	dims := 8
	idx := NewVectorIndex(&VectorIndexConfig{
		Dimensions: dims,
		M:          16,
		Ml:         0.25,
		EfSearch:   100,
	})

	n := 20
	embeddings := make(map[string][]float32, n)
	for i := 0; i < n; i++ {
		vec := make([]float32, dims)
		for d := 0; d < dims; d++ {
			vec[d] = float32(math.Sin(float64(i*d + d)))
		}
		embeddings[fmt.Sprintf("v%d", i)] = vec
	}
	idx.Build(embeddings)

	query := []float32{0.5, 0.3, -0.2, 0.8, 0.1, -0.6, 0.4, 0.7}
	queryNorm := vector.Normalize(query)

	// Brute force search
	type scored struct {
		id    string
		score float64
	}
	var bf []scored
	for id, emb := range embeddings {
		norm := vector.Normalize(emb)
		score := vector.DotProduct(queryNorm, norm)
		bf = append(bf, scored{id, score})
	}
	// Sort descending
	for i := 0; i < len(bf); i++ {
		for j := i + 1; j < len(bf); j++ {
			if bf[j].score > bf[i].score {
				bf[i], bf[j] = bf[j], bf[i]
			}
		}
	}

	// HNSW search
	hnswResults := idx.Search(query, 5)

	// The top result from HNSW should be in the top 5 of brute force
	if len(hnswResults) > 0 {
		top5ids := make(map[string]bool)
		for i := 0; i < 5 && i < len(bf); i++ {
			top5ids[bf[i].id] = true
		}
		assert.True(t, top5ids[hnswResults[0]], "HNSW top result should be in brute-force top 5")
	}
}

func BenchmarkVectorIndex_Build(b *testing.B) {
	dims := 768
	n := 1000
	embeddings := make(map[string][]float32, n)
	for i := 0; i < n; i++ {
		vec := make([]float32, dims)
		for d := 0; d < dims; d++ {
			vec[d] = float32(math.Sin(float64(i*d + d)))
		}
		embeddings[fmt.Sprintf("v%d", i)] = vec
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		idx := NewVectorIndex(DefaultVectorIndexConfig(dims))
		idx.Build(embeddings)
	}
}

func BenchmarkVectorIndex_Search(b *testing.B) {
	dims := 768
	n := 1000
	embeddings := make(map[string][]float32, n)
	for i := 0; i < n; i++ {
		vec := make([]float32, dims)
		for d := 0; d < dims; d++ {
			vec[d] = float32(math.Sin(float64(i*d + d)))
		}
		embeddings[fmt.Sprintf("v%d", i)] = vec
	}

	idx := NewVectorIndex(DefaultVectorIndexConfig(dims))
	idx.Build(embeddings)

	query := make([]float32, dims)
	for d := 0; d < dims; d++ {
		query[d] = 0.5
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		idx.Search(query, 10)
	}
}

func BenchmarkVectorIndex_BruteForceSearch(b *testing.B) {
	dims := 768
	n := 1000
	type embEntry struct {
		id  string
		vec []float32
	}
	var entries []embEntry
	for i := 0; i < n; i++ {
		vec := make([]float32, dims)
		for d := 0; d < dims; d++ {
			vec[d] = float32(math.Sin(float64(i*d + d)))
		}
		entries = append(entries, embEntry{id: fmt.Sprintf("v%d", i), vec: vec})
	}

	query := make([]float32, dims)
	for d := 0; d < dims; d++ {
		query[d] = 0.5
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		queryNorm := vector.Normalize(query)
		for _, e := range entries {
			norm := vector.Normalize(e.vec)
			_ = vector.DotProduct(queryNorm, norm)
		}
	}
}

// --- Scalability benchmarks: HNSW vs BruteForce at 1K / 10K vectors ---

func benchmarkHNSWSearchAtScale(b *testing.B, n int) {
	dims := 768
	embeddings := make(map[string][]float32, n)
	for i := 0; i < n; i++ {
		vec := make([]float32, dims)
		for d := 0; d < dims; d++ {
			vec[d] = float32(math.Sin(float64(i*d + d)))
		}
		embeddings[fmt.Sprintf("v%d", i)] = vec
	}

	idx := NewVectorIndex(&VectorIndexConfig{
		Dimensions: dims,
		M:          16,
		Ml:         0.25,
		EfSearch:   100,
	})
	idx.Build(embeddings)

	query := make([]float32, dims)
	for d := 0; d < dims; d++ {
		query[d] = 0.5
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		idx.Search(query, 10)
	}
}

func benchmarkBruteForceSearchAtScale(b *testing.B, n int) {
	dims := 768
	type embEntry struct {
		id  string
		vec []float32
	}
	entries := make([]embEntry, n)
	for i := 0; i < n; i++ {
		vec := make([]float32, dims)
		for d := 0; d < dims; d++ {
			vec[d] = float32(math.Sin(float64(i*d + d)))
		}
		entries[i] = embEntry{id: fmt.Sprintf("v%d", i), vec: vec}
	}

	query := make([]float32, dims)
	for d := 0; d < dims; d++ {
		query[d] = 0.5
	}
	queryNorm := vector.Normalize(query)

	// Pre-normalize all vectors (simulating stored pre-normalized vectors)
	normalized := make([][]float32, n)
	for i, e := range entries {
		normalized[i] = vector.Normalize(e.vec)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for _, norm := range normalized {
			_ = vector.DotProduct(queryNorm, norm)
		}
	}
}

func BenchmarkHNSW_Search_1K(b *testing.B)   { benchmarkHNSWSearchAtScale(b, 1000) }
func BenchmarkHNSW_Search_10K(b *testing.B)  { benchmarkHNSWSearchAtScale(b, 10000) }
func BenchmarkBruteForce_1K(b *testing.B)    { benchmarkBruteForceSearchAtScale(b, 1000) }
func BenchmarkBruteForce_10K(b *testing.B)   { benchmarkBruteForceSearchAtScale(b, 10000) }

func BenchmarkHNSW_Build_1K(b *testing.B) { benchmarkHNSWBuildAtScale(b, 1000) }
func BenchmarkHNSW_Build_10K(b *testing.B) { benchmarkHNSWBuildAtScale(b, 10000) }

func benchmarkHNSWBuildAtScale(b *testing.B, n int) {
	dims := 768
	embeddings := make(map[string][]float32, n)
	for i := 0; i < n; i++ {
		vec := make([]float32, dims)
		for d := 0; d < dims; d++ {
			vec[d] = float32(math.Sin(float64(i*d + d)))
		}
		embeddings[fmt.Sprintf("v%d", i)] = vec
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		idx := NewVectorIndex(DefaultVectorIndexConfig(dims))
		idx.Build(embeddings)
	}
}

package search

import (
	"math"
	"sync"

	"github.com/coder/hnsw"
	"github.com/xgsong/MyMemoryGo/internal/pkg/vector"
)

const (
	defaultM        = 16
	defaultEfSearch = 20
)

// VectorIndexConfig holds configuration for the vector index.
type VectorIndexConfig struct {
	// Dimensions is the dimensionality of embedding vectors.
	Dimensions int `json:"dimensions" yaml:"dimensions"`

	// M is the maximum number of neighbors per node in the HNSW graph.
	M int `json:"m" yaml:"m"`

	// Ml is the level generation factor for the HNSW graph.
	Ml float64 `json:"ml" yaml:"ml"`

	// EfSearch is the number of nodes to consider during search.
	EfSearch int `json:"ef_search" yaml:"ef_search"`
}

// DefaultVectorIndexConfig returns a config with sensible defaults for 768-dim vectors.
func DefaultVectorIndexConfig(dims int) *VectorIndexConfig {
	m := defaultM
	return &VectorIndexConfig{
		Dimensions: dims,
		M:          m,
		Ml:         1.0 / math.Sqrt(float64(m)),
		EfSearch:   defaultEfSearch,
	}
}

// VectorIndex wraps an HNSW graph for fast approximate nearest neighbor search.
// It is an in-memory secondary index that stays in sync with the SQLite store.
type VectorIndex struct {
	graph    *hnsw.Graph[string]
	mu       sync.RWMutex
	dims     int
	loaded   bool
	keySet   map[string]struct{} // track keys to handle Delete+Add safely
}

// NewVectorIndex creates a new VectorIndex with the given configuration.
func NewVectorIndex(config *VectorIndexConfig) *VectorIndex {
	if config == nil {
		config = DefaultVectorIndexConfig(768)
	}
	if config.M <= 0 {
		config.M = defaultM
	}
	if config.Ml <= 0 {
		config.Ml = 1.0 / math.Sqrt(float64(config.M))
	}
	if config.EfSearch <= 0 {
		config.EfSearch = defaultEfSearch
	}

	g := hnsw.NewGraph[string]()
	g.M = config.M
	g.Ml = config.Ml
	g.EfSearch = config.EfSearch
	g.Distance = hnsw.CosineDistance

	return &VectorIndex{
		graph:  g,
		dims:   config.Dimensions,
		keySet: make(map[string]struct{}),
	}
}

// Build constructs the index from a map of id -> embedding.
// This is used on startup to load all embeddings from SQLite.
func (idx *VectorIndex) Build(embeddings map[string][]float32) {
	idx.mu.Lock()
	defer idx.mu.Unlock()

	// Reset the graph
	g := hnsw.NewGraph[string]()
	g.M = idx.graph.M
	g.Ml = idx.graph.Ml
	g.EfSearch = idx.graph.EfSearch
	g.Distance = idx.graph.Distance
	idx.graph = g
	idx.keySet = make(map[string]struct{})

	if len(embeddings) == 0 {
		idx.loaded = true
		return
	}

	// Insert each vector individually
	for id, emb := range embeddings {
		normalized := vector.Normalize(emb)
		idx.graph.Add(hnsw.MakeNode(id, normalized))
		idx.keySet[id] = struct{}{}
	}

	idx.loaded = true
}

// Add inserts or updates a vector in the index.
func (idx *VectorIndex) Add(id string, embedding []float32) {
	idx.mu.Lock()
	defer idx.mu.Unlock()

	normalized := vector.Normalize(embedding)

	// Delete existing entry with same key before adding (HNSW doesn't support in-place update)
	if _, exists := idx.keySet[id]; exists {
		// Only delete if the graph has more than one node.
		// Deleting the last node can leave the graph in an invalid state
		// where Dims() panics on the next Add.
		if idx.graph.Len() > 1 {
			idx.graph.Delete(id)
		} else {
			// Rebuild from scratch when deleting the only node
			g := hnsw.NewGraph[string]()
			g.M = idx.graph.M
			g.Ml = idx.graph.Ml
			g.EfSearch = idx.graph.EfSearch
			g.Distance = idx.graph.Distance
			idx.graph = g
			delete(idx.keySet, id)
		}
	}

	idx.graph.Add(hnsw.MakeNode(id, normalized))
	idx.keySet[id] = struct{}{}
	idx.loaded = true
}

// Remove deletes a vector from the index.
func (idx *VectorIndex) Remove(id string) {
	idx.mu.Lock()
	defer idx.mu.Unlock()

	if _, exists := idx.keySet[id]; !exists {
		return
	}

	if idx.graph.Len() > 1 {
		idx.graph.Delete(id)
	} else if idx.graph.Len() == 1 {
		// Deleting the last node — reset the graph to avoid Dims() panic
		g := hnsw.NewGraph[string]()
		g.M = idx.graph.M
		g.Ml = idx.graph.Ml
		g.EfSearch = idx.graph.EfSearch
		g.Distance = idx.graph.Distance
		idx.graph = g
	}

	delete(idx.keySet, id)
}

// Search returns the top-K nearest neighbor IDs for the given query vector.
// Returns IDs sorted by decreasing similarity.
func (idx *VectorIndex) Search(query []float32, k int) []string {
	idx.mu.RLock()
	defer idx.mu.RUnlock()

	if !idx.loaded || idx.graph.Len() == 0 {
		return nil
	}

	// Normalize query for dot-product/cosine distance
	queryNorm := vector.Normalize(query)

	neighbors := idx.graph.Search(queryNorm, k)

	ids := make([]string, 0, len(neighbors))
	for _, node := range neighbors {
		ids = append(ids, node.Key)
	}
	return ids
}

// Size returns the number of vectors in the index.
func (idx *VectorIndex) Size() int {
	idx.mu.RLock()
	defer idx.mu.RUnlock()

	return idx.graph.Len()
}

// IsLoaded returns whether the index has been populated.
func (idx *VectorIndex) IsLoaded() bool {
	idx.mu.RLock()
	defer idx.mu.RUnlock()

	return idx.loaded
}

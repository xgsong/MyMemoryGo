package search

import (
	"math"
	"sync"

	"github.com/xgsong/MyMemoryGo/internal/domain/entity"
	"github.com/xgsong/MyMemoryGo/internal/pkg/vector"
)

const (
	DefaultMaxEmbeddings = 10000
	EvictionThreshold   = 0.9
)

type MMRReranker struct {
	Embeddings    sync.Map
	maxEmbeddings int
	evictionCount int64
	mu            sync.Mutex
}

type embeddingEntry struct {
	embedding []float32
	lastUsed  int64
}

func NewMMRReranker() *MMRReranker {
	return &MMRReranker{
		maxEmbeddings: DefaultMaxEmbeddings,
	}
}

func NewMMRRerankerWithMax(max int) *MMRReranker {
	if max <= 0 {
		max = DefaultMaxEmbeddings
	}
	return &MMRReranker{
		maxEmbeddings: max,
	}
}

func (r *MMRReranker) evictOldest() {
	r.mu.Lock()
	defer r.mu.Unlock()

	evictCount := int(float64(r.maxEmbeddings) * 0.2)
	if evictCount < 100 {
		evictCount = 100
	}

	type keyTime struct {
		key  string
		time int64
	}
	var entries []keyTime

	r.Embeddings.Range(func(key, value interface{}) bool {
		if k, ok := key.(string); ok {
			if entry, ok := value.(*embeddingEntry); ok {
				entries = append(entries, keyTime{key: k, time: entry.lastUsed})
			}
		}
		return true
	})

	for i := 0; i < len(entries); i++ {
		for j := i + 1; j < len(entries); j++ {
			if entries[i].time > entries[j].time {
				entries[i], entries[j] = entries[j], entries[i]
			}
		}
	}

	for i := 0; i < evictCount && i < len(entries); i++ {
		r.Embeddings.Delete(entries[i].key)
	}

	r.evictionCount += int64(evictCount)
}

func (r *MMRReranker) SetEmbedding(id string, embedding []float32) {
	var count int
	r.Embeddings.Range(func(_, _ interface{}) bool {
		count++
		return true
	})

	if count >= int(float64(r.maxEmbeddings)*EvictionThreshold) {
		r.evictOldest()
	}

	entry := &embeddingEntry{
		embedding: make([]float32, len(embedding)),
		lastUsed:  r.evictionCount,
	}
	copy(entry.embedding, embedding)
	r.Embeddings.Store(id, entry)
}

func (r *MMRReranker) Rerank(hits []*entity.SearchHit, lambda float64) []*entity.SearchHit {
	if len(hits) == 0 {
		return hits
	}

	if lambda >= 1.0 {
		return hits
	}

	if lambda <= 0.0 {
		lambda = 0.01
	}

	// Pre-populate embeddings from hits for MMR diversity computation
	for _, hit := range hits {
		if hit.Embedding != nil {
			r.SetEmbedding(hit.ID, hit.Embedding)
		}
	}

	selected := make([]*entity.SearchHit, 0, len(hits))
	remaining := make([]*entity.SearchHit, len(hits))
	copy(remaining, hits)

	selected = append(selected, remaining[0])
	remaining = remaining[1:]

	for len(remaining) > 0 && len(selected) < len(hits) {
		bestIdx := 0
		bestScore := math.Inf(-1)

		for i, candidate := range remaining {
			relevance := candidate.Score

			maxSimilarity := 0.0
			for _, sel := range selected {
				similarity := r.cosineSimilarity(candidate.ID, sel.ID)
				if similarity > maxSimilarity {
					maxSimilarity = similarity
				}
			}

			mmrScore := lambda*relevance - (1-lambda)*maxSimilarity

			if mmrScore > bestScore {
				bestScore = mmrScore
				bestIdx = i
			}
		}

		selected = append(selected, remaining[bestIdx])
		remaining = append(remaining[:bestIdx], remaining[bestIdx+1:]...)
	}

	return selected
}

func (r *MMRReranker) cosineSimilarity(id1, id2 string) float64 {
	emb1Val, ok1 := r.Embeddings.Load(id1)
	emb2Val, ok2 := r.Embeddings.Load(id2)

	if !ok1 || !ok2 {
		return 0.0
	}

	entry1, ok1 := emb1Val.(*embeddingEntry)
	entry2, ok2 := emb2Val.(*embeddingEntry)

	if ok1 && ok2 {
		// Embeddings are pre-normalized, so dot product equals cosine similarity
		return vector.DotProduct(entry1.embedding, entry2.embedding)
	}

	emb1, ok1 := emb1Val.([]float32)
	emb2, ok2 := emb2Val.([]float32)

	if !ok1 || !ok2 {
		return 0.0
	}

	return vector.DotProduct(emb1, emb2)
}

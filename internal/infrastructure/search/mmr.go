package search

import (
	"math"
	"sync"

	"github.com/xgsong/MyMemoryGo/internal/domain/entity"
	"github.com/xgsong/MyMemoryGo/internal/pkg/vector"
)

type MMRReranker struct {
	Embeddings sync.Map
}

func NewMMRReranker() *MMRReranker {
	return &MMRReranker{}
}

func (r *MMRReranker) SetEmbedding(id string, embedding []float32) {
	r.Embeddings.Store(id, embedding)
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

	emb1, ok1 := emb1Val.([]float32)
	emb2, ok2 := emb2Val.([]float32)

	if !ok1 || !ok2 {
		return 0.0
	}

	return vector.CosineSimilarity(emb1, emb2)
}

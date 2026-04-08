// Package search provides search implementations for memory retrieval.
// This file contains result merging and filtering functions for hybrid search.
package search

import (
	"sort"

	"github.com/xgsong/MyMemoryGo/internal/domain/entity"
)

// mergeResults merges vector and full-text search results with weighted scoring.
// It deduplicates results by ID and combines scores based on the provided weights.
func (e *HybridEngine) mergeResults(vectorHits, fulltextHits []*entity.SearchHit, vectorWeight, fulltextWeight float64) []*entity.SearchHit {
	merged := make(map[string]*entity.SearchHit)

	// Add vector search results
	for _, hit := range vectorHits {
		hit.Score = hit.Score * vectorWeight
		merged[hit.ID] = hit
	}

	// Merge full-text search results
	for _, hit := range fulltextHits {
		if existing, ok := merged[hit.ID]; ok {
			// Combine scores
			existing.Score += hit.Score * fulltextWeight
		} else {
			// New result
			hit.Score = hit.Score * fulltextWeight
			merged[hit.ID] = hit
		}
	}

	// Convert to slice and sort
	result := make([]*entity.SearchHit, 0, len(merged))
	for _, hit := range merged {
		result = append(result, hit)
	}

	// Sort by score descending
	sort.Slice(result, func(i, j int) bool {
		return result[i].Score > result[j].Score
	})

	return result
}

// filterAndLimit filters results by minimum score and limits the number of results.
// Results are assumed to be pre-sorted by score in descending order.
func (e *HybridEngine) filterAndLimit(hits []*entity.SearchHit, minScore float64, limit int) []*entity.SearchHit {
	var result []*entity.SearchHit
	for _, hit := range hits {
		if hit.Score >= minScore {
			result = append(result, hit)
			if len(result) >= limit {
				break
			}
		}
	}
	return result
}

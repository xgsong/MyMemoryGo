package sqlite

import (
	"sort"

	"github.com/xgsong/MyMemoryGo/internal/domain/entity"
)

func (s *Store) mergeSearchResults(vectorHits, fulltextHits []*entity.SearchHit, vectorWeight, fulltextWeight float64) []*entity.SearchHit {
	merged := make(map[string]*entity.SearchHit)

	for _, hit := range vectorHits {
		hit.Score = hit.Score * vectorWeight
		merged[hit.ID] = hit
	}

	for _, hit := range fulltextHits {
		if existing, ok := merged[hit.ID]; ok {
			existing.Score += hit.Score * fulltextWeight
		} else {
			hit.Score = hit.Score * fulltextWeight
			merged[hit.ID] = hit
		}
	}

	result := make([]*entity.SearchHit, 0, len(merged))
	for _, hit := range merged {
		result = append(result, hit)
	}

	return result
}

func (s *Store) filterBySource(hits []*entity.SearchHit, sources []entity.SourceType) []*entity.SearchHit {
	sourceSet := make(map[entity.SourceType]bool)
	for _, src := range sources {
		sourceSet[src] = true
	}

	var filtered []*entity.SearchHit
	for _, hit := range hits {
		if sourceSet[hit.Source] {
			filtered = append(filtered, hit)
		}
	}
	return filtered
}

func truncateContent(content string, maxLen int) string {
	if len(content) <= maxLen {
		return content
	}
	return content[:maxLen] + "..."
}

func (s *Store) filterAndLimitHits(hits []*entity.SearchHit, minScore float64, limit int) []*entity.SearchHit {
	sortedHits := make([]*entity.SearchHit, len(hits))
	copy(sortedHits, hits)

	sort.Slice(sortedHits, func(i, j int) bool {
		return sortedHits[i].Score > sortedHits[j].Score
	})

	var result []*entity.SearchHit
	for _, hit := range sortedHits {
		if hit.Score >= minScore {
			result = append(result, hit)
			if len(result) >= limit {
				break
			}
		}
	}
	return result
}


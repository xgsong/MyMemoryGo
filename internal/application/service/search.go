package service

import (
	"context"
	"time"

	"github.com/xgsong/MyMemoryGo/internal/domain/entity"
	"github.com/xgsong/MyMemoryGo/internal/domain/repository"
	"github.com/xgsong/MyMemoryGo/internal/pkg/log"
)

// RerankOptions configures MMR (Maximal Marginal Relevance) reranking.
type RerankOptions struct {
	Enabled bool
	Lambda  float64 // 0.0-1.0, default 0.7. Higher favors relevance, lower favors diversity.
}

// DecayOptions configures temporal decay scoring.
type DecayOptions struct {
	Enabled  bool
	HalfLife time.Duration // Time for score to reduce by half, default 720h.
}

// SearchMemoriesRequest contains parameters for searching memories.
type SearchMemoriesRequest struct {
	Query        string
	Limit        int
	MinScore     float64
	SourceFilter []entity.SourceType
	Rerank       *RerankOptions // MMR reranking configuration
	Decay        *DecayOptions  // Temporal decay configuration
}

// SearchMemoriesResponse contains search results.
type SearchMemoriesResponse struct {
	Results  *entity.SearchResult
	Duration time.Duration
}

// SearchMemories performs hybrid search combining vector and full-text search.
// It uses the SearchRepository for actual search operations.
func (s *MemoryApplicationService) SearchMemories(ctx context.Context, req *SearchMemoriesRequest) (*SearchMemoriesResponse, error) {
	logger := log.GetLogger(ctx).With("query", req.Query, "limit", req.Limit)

	logger.InfoContext(ctx, "starting hybrid search")

	s.writeMutex.RLock()
	defer s.writeMutex.RUnlock()

	// Build search options using builder pattern
	builder := repository.NewSearchOptionsBuilder().
		WithLimit(req.Limit).
		WithMinScore(req.MinScore).
		WithSourceFilter(req.SourceFilter)

	// Apply rerank options if enabled
	if req.Rerank != nil && req.Rerank.Enabled {
		lambda := req.Rerank.Lambda
		if lambda == 0 {
			lambda = 0.7 // default lambda
		}
		builder = builder.WithMMR(lambda)
		logger.DebugContext(ctx, "MMR reranking enabled", "lambda", lambda)
	}

	// Apply decay options if enabled
	if req.Decay != nil && req.Decay.Enabled {
		halfLife := req.Decay.HalfLife
		if halfLife == 0 {
			halfLife = 720 * time.Hour // default half-life
		}
		builder = builder.WithDecay(halfLife)
		logger.DebugContext(ctx, "temporal decay enabled", "half_life", halfLife.String())
	}

	opts := builder.Build()

	// Use SearchRepository for hybrid search
	result, err := s.searchRepo.Search(ctx, req.Query, opts)
	if err != nil {
		logger.ErrorContext(ctx, "hybrid search failed", "error", err)
		return nil, err
	}

	logger.InfoContext(ctx, "search completed", "result_count", len(result.Hits), "duration_ms", result.Duration.Milliseconds())

	return &SearchMemoriesResponse{
		Results:  result,
		Duration: result.Duration,
	}, nil
}

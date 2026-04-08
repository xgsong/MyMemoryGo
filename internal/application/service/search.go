package service

import (
	"context"
	"time"

	"github.com/xgsong/MyMemoryGo/internal/domain/entity"
	"github.com/xgsong/MyMemoryGo/internal/domain/repository"
	"github.com/xgsong/MyMemoryGo/internal/pkg/errors"
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

	// Validate request parameters first
	if req.Query == "" {
		return nil, errors.New(errors.CodeInvalidInput, "query cannot be empty")
	}
	if req.Limit < 0 {
		return nil, errors.New(errors.CodeInvalidInput, "limit cannot be negative")
	}
	if req.MinScore < 0 || req.MinScore > 1 {
		return nil, errors.New(errors.CodeInvalidInput, "min_score must be between 0 and 1")
	}

	// Build search options using builder pattern
	builder := repository.NewSearchOptionsBuilder().
		WithLimit(req.Limit).
		WithMinScore(req.MinScore).
		WithSourceFilter(req.SourceFilter)

	// Apply rerank options if enabled
	if req.Rerank != nil && req.Rerank.Enabled {
		lambda := req.Rerank.Lambda
		if lambda < 0 || lambda > 1 {
			return nil, errors.New(errors.CodeInvalidInput, "lambda must be between 0 and 1")
		}
		builder = builder.WithMMR(lambda)
		logger.DebugContext(ctx, "MMR reranking enabled", "lambda", lambda)
	}

	// Apply decay options if enabled
	if req.Decay != nil && req.Decay.Enabled {
		halfLife := req.Decay.HalfLife
		if halfLife < 0 {
			return nil, errors.New(errors.CodeInvalidInput, "half_life cannot be negative")
		}
		if halfLife == 0 {
			halfLife = 720 * time.Hour // default half-life
		}
		builder = builder.WithDecay(halfLife)
		logger.DebugContext(ctx, "temporal decay enabled", "half_life", halfLife.String())
	}

	// Generate query embedding for vector search
	queryEmbedding, embedErr := s.embeddingRepo.Embed(ctx, req.Query)
	if embedErr != nil {
		logger.WarnContext(ctx, "failed to generate query embedding, vector search will be degraded", "error", embedErr)
	}

	opts, err := builder.Build()
	if err != nil {
		return nil, errors.WrapOp(errors.CodeInvalidInput, "SearchMemories", "invalid search options", err)
	}
	opts.QueryEmbedding = queryEmbedding

	// Use SearchRepository for hybrid search
	result, err := s.searchRepo.Search(ctx, req.Query, opts)
	if err != nil {
		logger.ErrorContext(ctx, "hybrid search failed", "error", err)
		return nil, errors.WrapOp(errors.CodeDatabase, "SearchMemories", "hybrid search failed", err)
	}

	logger.InfoContext(ctx, "search completed", "result_count", len(result.Hits), "duration_ms", result.Duration.Milliseconds())

	return &SearchMemoriesResponse{
		Results:  result,
		Duration: result.Duration,
	}, nil
}

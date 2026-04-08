// Package search provides search implementations for memory retrieval.
// This package implements hybrid search combining vector and full-text search.
package search

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/xgsong/MyMemoryGo/internal/domain/entity"
	"github.com/xgsong/MyMemoryGo/internal/domain/repository"
)

// HybridSearchConfig holds configuration for the hybrid search engine.
type HybridSearchConfig struct {
	// VectorWeight is the weight for vector search results (default: 0.7).
	VectorWeight float64 `json:"vector_weight" yaml:"vector_weight"`

	// FulltextWeight is the weight for full-text search results (default: 0.3).
	FulltextWeight float64 `json:"fulltext_weight" yaml:"fulltext_weight"`

	// DefaultLimit is the default maximum number of results (default: 10).
	DefaultLimit int `json:"default_limit" yaml:"default_limit"`

	// MinScore is the minimum relevance score (default: 0.5).
	MinScore float64 `json:"min_score" yaml:"min_score"`
}

// DefaultHybridSearchConfig returns a Config with sensible defaults.
func DefaultHybridSearchConfig() *HybridSearchConfig {
	return &HybridSearchConfig{
		VectorWeight:   0.7,
		FulltextWeight: 0.3,
		DefaultLimit:   10,
		MinScore:       0.5,
	}
}

// HybridEngine implements hybrid search combining vector and full-text search.
type HybridEngine struct {
	config       *HybridSearchConfig
	vectorRepo   repository.SearchRepository
	fulltextRepo repository.SearchRepository
	embedder     repository.EmbeddingRepository
	reranker     Reranker
	decayCalc    DecayCalculator
	mu           sync.RWMutex
}

// Reranker defines the interface for search result reranking.
type Reranker interface {
	Rerank(hits []*entity.SearchHit, lambda float64) []*entity.SearchHit
}

// DecayCalculator defines the interface for temporal decay calculation.
type DecayCalculator interface {
	Apply(hits []*entity.SearchHit, halfLife time.Duration)
}

// NewHybridEngine creates a new hybrid search engine.
func NewHybridEngine(
	config *HybridSearchConfig,
	vectorRepo repository.SearchRepository,
	fulltextRepo repository.SearchRepository,
	embedder repository.EmbeddingRepository,
) *HybridEngine {
	if config == nil {
		config = DefaultHybridSearchConfig()
	}

	return &HybridEngine{
		config:       config,
		vectorRepo:   vectorRepo,
		fulltextRepo: fulltextRepo,
		embedder:     embedder,
	}
}

// SetReranker sets the reranker for the engine.
func (e *HybridEngine) SetReranker(reranker Reranker) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.reranker = reranker
}

// SetDecayCalculator sets the decay calculator for the engine.
func (e *HybridEngine) SetDecayCalculator(calc DecayCalculator) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.decayCalc = calc
}

// Search performs hybrid search combining vector and full-text search.
func (e *HybridEngine) Search(ctx context.Context, query string, opts *repository.SearchOptions) (*entity.SearchResult, error) {
	startTime := time.Now()

	// Set default options
	if opts == nil {
		opts = repository.DefaultSearchOptions()
	}
	if opts.Limit == 0 {
		opts.Limit = e.config.DefaultLimit
	}
	// MinScore already has default from DefaultSearchOptions if not set
	// VectorWeight already has default from DefaultSearchOptions if not set
	// FulltextWeight already has default from DefaultSearchOptions if not set
	// Ensure all pointer fields are initialized
	if opts.MinScore == nil {
		minScore := e.config.MinScore
		opts.MinScore = &minScore
	}
	if opts.VectorWeight == nil {
		vectorWeight := e.config.VectorWeight
		opts.VectorWeight = &vectorWeight
	}
	if opts.FulltextWeight == nil {
		fulltextWeight := e.config.FulltextWeight
		opts.FulltextWeight = &fulltextWeight
	}
	if opts.MMRLambda == nil {
		mmrLambda := 0.7
		opts.MMRLambda = &mmrLambda
	}

	// Generate query embedding
	queryVec, err := e.embedder.Embed(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("embed query: %w", err)
	}

	// Perform concurrent searches
	var wg sync.WaitGroup
	var vectorHits, fulltextHits []*entity.SearchHit
	var vectorErr, fulltextErr error

	// Double the limit for each search to ensure enough candidates
	searchLimit := opts.Limit * 2

	wg.Add(2)
	go func() {
		defer wg.Done()
		if e.vectorRepo != nil {
			vectorHits, vectorErr = e.vectorRepo.SearchVector(ctx, queryVec, &repository.SearchOptions{
				Limit:    searchLimit,
				MinScore: opts.MinScore,
			})
		}
	}()

	go func() {
		defer wg.Done()
		if e.fulltextRepo != nil {
			fulltextHits, fulltextErr = e.fulltextRepo.SearchFulltext(ctx, query, &repository.SearchOptions{
				Limit: searchLimit,
			})
		}
	}()
	wg.Wait()

	// Handle errors (fulltext failure is non-fatal)
	if vectorErr != nil {
		return nil, fmt.Errorf("vector search: %w", vectorErr)
	}
	if fulltextErr != nil {
		// Log but continue
		fmt.Printf("fulltext search error: %v\n", fulltextErr)
	}

	// Merge and deduplicate results
	merged := e.mergeResults(vectorHits, fulltextHits, *opts.VectorWeight, *opts.FulltextWeight)

	// Apply temporal decay if enabled
	e.mu.RLock()
	decayCalc := e.decayCalc
	e.mu.RUnlock()
	
	if opts.UseDecay && decayCalc != nil && opts.DecayHalfLife > 0 {
		decayCalc.Apply(merged, opts.DecayHalfLife)
	}

	// Apply MMR reranking if enabled
	e.mu.RLock()
	reranker := e.reranker
	e.mu.RUnlock()
	
	if opts.UseMMR && reranker != nil {
		lambda := *opts.MMRLambda
		merged = reranker.Rerank(merged, lambda)
	}

	// Filter by minimum score and limit
	result := e.filterAndLimit(merged, *opts.MinScore, opts.Limit)

	return &entity.SearchResult{
		Hits:     result,
		Total:    len(merged),
		Duration: time.Since(startTime),
		Query:    query,
	}, nil
}

// SearchVector performs vector-only search.
func (e *HybridEngine) SearchVector(ctx context.Context, embedding []float32, opts *repository.SearchOptions) ([]*entity.SearchHit, error) {
	if e.vectorRepo == nil {
		return nil, fmt.Errorf("vector repository not configured")
	}

	if opts == nil {
		opts = &repository.SearchOptions{}
	}
	if opts.Limit == 0 {
		opts.Limit = e.config.DefaultLimit
	}

	return e.vectorRepo.SearchVector(ctx, embedding, opts)
}

// SearchFulltext performs full-text-only search.
func (e *HybridEngine) SearchFulltext(ctx context.Context, query string, opts *repository.SearchOptions) ([]*entity.SearchHit, error) {
	if e.fulltextRepo == nil {
		return nil, fmt.Errorf("fulltext repository not configured")
	}

	if opts == nil {
		opts = &repository.SearchOptions{}
	}
	if opts.Limit == 0 {
		opts.Limit = e.config.DefaultLimit
	}

	return e.fulltextRepo.SearchFulltext(ctx, query, opts)
}

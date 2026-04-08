// Package repository defines the interfaces for data persistence and retrieval.
// These interfaces abstract the underlying storage implementation, following
// the Dependency Inversion Principle of DDD.
package repository

import (
	"context"
	"time"

	"github.com/xgsong/MyMemoryGo/internal/domain/entity"
	"github.com/xgsong/MyMemoryGo/internal/pkg/errors"
)

// MemoryRepository defines the interface for memory persistence operations.
// Implementations should handle both file-based storage and indexing.
type MemoryRepository interface {
	// Store saves a memory entry to the repository.
	// It should persist the memory to both the file system and the search index.
	Store(ctx context.Context, memory *entity.Memory) error

	// StoreBatch saves multiple memory entries in a single transaction.
	// This is more efficient than calling Store multiple times.
	StoreBatch(ctx context.Context, memories []*entity.Memory) error

	// Get retrieves a memory entry by its ID.
	// Returns ErrNotFound if the memory does not exist.
	Get(ctx context.Context, id string) (*entity.Memory, error)

	// GetByPath retrieves all memory entries for a given file path.
	GetByPath(ctx context.Context, path string) ([]*entity.Memory, error)

	// Delete removes a memory entry by ID.
	// It should remove from both file system and index.
	Delete(ctx context.Context, id string) error

	// DeleteByPath removes all memory entries for a given file path.
	DeleteByPath(ctx context.Context, path string) error

	// List retrieves all memory entries matching the given criteria.
	List(ctx context.Context, opts *ListOptions) ([]*entity.Memory, int, error)
}

// ListOptions defines parameters for listing memories.
type ListOptions struct {
	// Source filters by memory source type.
	Source entity.SourceType

	// PathPrefix filters by path prefix.
	PathPrefix string

	// Limit limits the number of results.
	Limit int

	// Offset is used for pagination.
	Offset int

	// OrderBy specifies the field to order by.
	OrderBy string

	// OrderDirection specifies ASC or DESC.
	OrderDirection string
}

// SearchRepository defines the interface for search operations.
// This is separated from MemoryRepository to allow different indexing strategies.
type SearchRepository interface {
	// Search performs a hybrid search combining vector and full-text search.
	Search(ctx context.Context, query string, opts *SearchOptions) (*entity.SearchResult, error)

	// SearchVector performs vector similarity search only.
	SearchVector(ctx context.Context, embedding []float32, opts *SearchOptions) ([]*entity.SearchHit, error)

	// SearchFulltext performs full-text keyword search only.
	SearchFulltext(ctx context.Context, query string, opts *SearchOptions) ([]*entity.SearchHit, error)

	// Index adds or updates memory entries in the search index.
	Index(ctx context.Context, memories []*entity.Memory) error

	// RemoveFromIndex removes memory entries from the search index.
	RemoveFromIndex(ctx context.Context, ids []string) error
}

// SearchOptions defines parameters for search operations.
type SearchOptions struct {
	// Limit is the maximum number of results to return.
	Limit int

	// MinScore is the minimum relevance score (0.0 to 1.0).
	// Pointer type to distinguish between "not set" and explicit 0.
	MinScore *float64

	// SourceFilter restricts search to specific source types.
	SourceFilter []entity.SourceType

	// UseMMR enables Maximal Marginal Relevance reranking.
	UseMMR bool

	// MMRLambda is the MMR parameter (0.0 to 1.0).
	// Higher values favor relevance, lower values favor diversity.
	// Pointer type to distinguish between "not set" and explicit 0.
	MMRLambda *float64

	// UseDecay enables temporal decay scoring.
	UseDecay bool

	// DecayHalfLife is the time for score to reduce by half.
	DecayHalfLife time.Duration

	// VectorWeight is the weight for vector search results (default 0.7).
	// Pointer type to distinguish between "not set" and explicit 0.
	VectorWeight *float64

	// FulltextWeight is the weight for full-text search results (default 0.3).
	// Pointer type to distinguish between "not set" and explicit 0.
	FulltextWeight *float64

	// QueryEmbedding is the pre-computed embedding for the search query.
	// If nil, vector search will use a fallback score.
	QueryEmbedding []float32
}

// DefaultSearchOptions returns SearchOptions with sensible defaults.
func DefaultSearchOptions() *SearchOptions {
	minScore := 0.5
	vectorWeight := 0.7
	fulltextWeight := 0.3
	return &SearchOptions{
		Limit:          10,
		MinScore:       &minScore,
		VectorWeight:   &vectorWeight,
		FulltextWeight: &fulltextWeight,
	}
}

// SearchOptionsBuilder provides a fluent API for constructing SearchOptions.
type SearchOptionsBuilder struct {
	opts *SearchOptions
}

// NewSearchOptionsBuilder creates a new builder with default options.
func NewSearchOptionsBuilder() *SearchOptionsBuilder {
	return &SearchOptionsBuilder{opts: DefaultSearchOptions()}
}

// WithLimit sets the result limit.
func (b *SearchOptionsBuilder) WithLimit(limit int) *SearchOptionsBuilder {
	b.opts.Limit = limit
	return b
}

// WithMinScore sets the minimum score.
func (b *SearchOptionsBuilder) WithMinScore(score float64) *SearchOptionsBuilder {
	b.opts.MinScore = &score
	return b
}

// WithSourceFilter sets the source filter.
func (b *SearchOptionsBuilder) WithSourceFilter(sources []entity.SourceType) *SearchOptionsBuilder {
	b.opts.SourceFilter = sources
	return b
}

// WithMMR enables MMR with the given lambda.
func (b *SearchOptionsBuilder) WithMMR(lambda float64) *SearchOptionsBuilder {
	b.opts.UseMMR = true
	b.opts.MMRLambda = &lambda
	return b
}

// WithDecay enables temporal decay with the given half-life.
func (b *SearchOptionsBuilder) WithDecay(halfLife time.Duration) *SearchOptionsBuilder {
	b.opts.UseDecay = true
	b.opts.DecayHalfLife = halfLife
	return b
}

// WithWeights sets the search weights.
func (b *SearchOptionsBuilder) WithWeights(vector, fulltext float64) *SearchOptionsBuilder {
	b.opts.VectorWeight = &vector
	b.opts.FulltextWeight = &fulltext
	return b
}

// Build returns the constructed SearchOptions.
// It validates the options before returning.
func (b *SearchOptionsBuilder) Build() (*SearchOptions, error) {
	if b.opts.Limit < 0 {
		return nil, errors.New(errors.CodeInvalidInput, "limit cannot be negative")
	}
	if *b.opts.MinScore < 0 || *b.opts.MinScore > 1 {
		return nil, errors.New(errors.CodeInvalidInput, "min_score must be between 0 and 1")
	}
	if *b.opts.VectorWeight < 0 || *b.opts.VectorWeight > 1 {
		return nil, errors.New(errors.CodeInvalidInput, "vector_weight must be between 0 and 1")
	}
	if *b.opts.FulltextWeight < 0 || *b.opts.FulltextWeight > 1 {
		return nil, errors.New(errors.CodeInvalidInput, "fulltext_weight must be between 0 and 1")
	}
	weightsSum := *b.opts.VectorWeight + *b.opts.FulltextWeight
	if weightsSum < 0.99 || weightsSum > 1.01 { // Allow small floating point error
		return nil, errors.New(errors.CodeInvalidInput, "vector_weight and fulltext_weight must sum to 1")
	}
	if b.opts.MMRLambda != nil {
		if *b.opts.MMRLambda < 0 || *b.opts.MMRLambda > 1 {
			return nil, errors.New(errors.CodeInvalidInput, "mmr_lambda must be between 0 and 1")
		}
	}
	return b.opts, nil
}

// EmbeddingRepository defines the interface for embedding operations.
// This abstracts the embedding provider to allow different implementations.
type EmbeddingRepository interface {
	// Embed generates an embedding for a single text.
	Embed(ctx context.Context, text string) ([]float32, error)

	// EmbedBatch generates embeddings for multiple texts in a single request.
	EmbedBatch(ctx context.Context, texts []string) ([][]float32, error)

	// Model returns the name of the embedding model being used.
	Model() string

	// Dimensions returns the dimensionality of the embeddings.
	Dimensions() int
}

// FileRepository defines the interface for file system operations.
// This abstracts file I/O to enable testing and different storage backends.
type FileRepository interface {
	// Read reads the content of a file.
	Read(ctx context.Context, path string) ([]byte, error)

	// Write writes content to a file.
	Write(ctx context.Context, path string, content []byte) error

	// Append appends content to an existing file.
	Append(ctx context.Context, path string, content []byte) error

	// Delete removes a file.
	Delete(ctx context.Context, path string) error

	// Exists checks if a file exists.
	Exists(ctx context.Context, path string) (bool, error)

	// List lists files matching a pattern.
	List(ctx context.Context, pattern string) ([]string, error)

	// Watch starts watching a file or directory for changes.
	Watch(ctx context.Context, path string, handler FileChangeHandler) error
}

// FileChangeHandler is a callback function for file change events.
type FileChangeHandler func(event FileChangeEvent)

// FileChangeEvent represents a file system change event.
type FileChangeEvent struct {
	// Path is the file path that changed.
	Path string

	// Operation is the type of change (create, modify, delete).
	Operation FileOperation

	// Timestamp is when the change occurred.
	Timestamp time.Time
}

// FileOperation represents the type of file change.
type FileOperation string

const (
	// FileOperationCreate indicates a file was created.
	FileOperationCreate FileOperation = "create"

	// FileOperationModify indicates a file was modified.
	FileOperationModify FileOperation = "modify"

	// FileOperationDelete indicates a file was deleted.
	FileOperationDelete FileOperation = "delete"
)

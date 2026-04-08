package sqlite

import (
	"context"
	"fmt"
	"time"

	"github.com/xgsong/MyMemoryGo/internal/domain/entity"
	"github.com/xgsong/MyMemoryGo/internal/domain/repository"
	"github.com/xgsong/MyMemoryGo/internal/pkg/vector"
)

func (s *Store) Search(ctx context.Context, query string, opts *repository.SearchOptions) (*entity.SearchResult, error) {
	startTime := time.Now()

	if opts == nil {
		opts = repository.DefaultSearchOptions()
	}
	if opts.Limit == 0 {
		opts.Limit = 10
	}
	if opts.MinScore == 0 {
		opts.MinScore = 0.5
	}
	if opts.VectorWeight == 0 {
		opts.VectorWeight = 0.7
	}
	if opts.FulltextWeight == 0 {
		opts.FulltextWeight = 0.3
	}

	type searchResult struct {
		hits []*entity.SearchHit
		err  error
	}

	vectorCh := make(chan searchResult, 1)
	fulltextCh := make(chan searchResult, 1)

	go func() {
		hits, err := s.SearchVector(ctx, nil, opts)
		vectorCh <- searchResult{hits: hits, err: err}
	}()

	go func() {
		hits, err := s.SearchFulltext(ctx, query, opts)
		fulltextCh <- searchResult{hits: hits, err: err}
	}()

	vectorRes := <-vectorCh
	fulltextRes := <-fulltextCh

	if vectorRes.err != nil {
		return nil, fmt.Errorf("vector search: %w", vectorRes.err)
	}

	if fulltextRes.err != nil {
		fulltextRes.hits = nil
	}

	merged := s.mergeSearchResults(vectorRes.hits, fulltextRes.hits, opts.VectorWeight, opts.FulltextWeight)

	if len(opts.SourceFilter) > 0 {
		merged = s.filterBySource(merged, opts.SourceFilter)
	}

	result := s.filterAndLimitHits(merged, opts.MinScore, opts.Limit)

	return &entity.SearchResult{
		Hits:     result,
		Total:    len(merged),
		Duration: time.Since(startTime),
		Query:    query,
	}, nil
}

func (s *Store) SearchVector(ctx context.Context, embedding []float32, opts *repository.SearchOptions) ([]*entity.SearchHit, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if opts == nil {
		opts = repository.DefaultSearchOptions()
	}
	if opts.Limit == 0 {
		opts.Limit = 10
	}

	query := `
		SELECT id, path, start_line, end_line, content, embedding, source, created_at
		FROM memories
		WHERE embedding IS NOT NULL
		ORDER BY created_at DESC
	`

	rows, err := s.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("query memories: %w", err)
	}
	defer rows.Close()

	var hits []*entity.SearchHit

	for rows.Next() {
		var id, path, content, source string
		var startLine, endLine int
		var embeddingBlob []byte
		var createdAt int64

		err := rows.Scan(&id, &path, &startLine, &endLine, &content, &embeddingBlob, &source, &createdAt)
		if err != nil {
			return nil, fmt.Errorf("scan row: %w", err)
		}

		memEmbedding, err := deserializeEmbedding(embeddingBlob)
		if err != nil {
			continue
		}

		var score float64
		if embedding != nil && len(embedding) > 0 {
			score = vector.CosineSimilarity(embedding, memEmbedding)
		} else {
			score = 0.5
		}

		snippet := truncateContent(content, 200)

		hits = append(hits, &entity.SearchHit{
			Entry: &entity.Entry{
				ID:        id,
				Path:      path,
				StartLine: startLine,
				EndLine:   endLine,
				Snippet:   snippet,
				Score:     score,
				Source:    entity.SourceType(source),
				Timestamp: time.Unix(createdAt, 0),
			},
			Embedding: memEmbedding,
		})
	}

	return hits, rows.Err()
}

func (s *Store) SearchFulltext(ctx context.Context, query string, opts *repository.SearchOptions) ([]*entity.SearchHit, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if opts == nil {
		opts = repository.DefaultSearchOptions()
	}
	if opts.Limit == 0 {
		opts.Limit = 10
	}

	ftsQuery := `
		SELECT m.id, m.path, m.start_line, m.end_line, m.content, m.embedding, m.source, m.created_at, f.rank
		FROM memories_fts f
		JOIN memories m ON f.id = m.id
		WHERE memories_fts MATCH ?
		ORDER BY f.rank
		LIMIT ?
	`

	rows, err := s.db.QueryContext(ctx, ftsQuery, query, opts.Limit*2)
	if err != nil {
		return nil, nil
	}
	defer rows.Close()

	var hits []*entity.SearchHit

	for rows.Next() {
		var id, path, content, source string
		var startLine, endLine int
		var embeddingBlob []byte
		var createdAt int64
		var rank float64

		err := rows.Scan(&id, &path, &startLine, &endLine, &content, &embeddingBlob, &source, &createdAt, &rank)
		if err != nil {
			continue
		}

		score := 1.0 / (1.0 + rank/10.0)

		snippet := truncateContent(content, 200)

		var embedding []float32
		if embeddingBlob != nil {
			embedding, _ = deserializeEmbedding(embeddingBlob)
		}

		hits = append(hits, &entity.SearchHit{
			Entry: &entity.Entry{
				ID:        id,
				Path:      path,
				StartLine: startLine,
				EndLine:   endLine,
				Snippet:   snippet,
				Score:     score,
				Source:    entity.SourceType(source),
				Timestamp: time.Unix(createdAt, 0),
			},
			Embedding: embedding,
		})
	}

	return hits, rows.Err()
}

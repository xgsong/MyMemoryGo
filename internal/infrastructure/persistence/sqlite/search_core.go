package sqlite

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/xgsong/MyMemoryGo/internal/domain/entity"
	"github.com/xgsong/MyMemoryGo/internal/domain/repository"
	"github.com/xgsong/MyMemoryGo/internal/pkg/errors"
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

	type searchResult struct {
		hits []*entity.SearchHit
		err  error
	}

	vectorCh := make(chan searchResult, 1)
	fulltextCh := make(chan searchResult, 1)

	go func() {
		hits, err := s.SearchVector(ctx, opts.QueryEmbedding, opts)
		vectorCh <- searchResult{hits: hits, err: err}
	}()

	go func() {
		hits, err := s.SearchFulltext(ctx, query, opts)
		fulltextCh <- searchResult{hits: hits, err: err}
	}()

	vectorRes := <-vectorCh
	fulltextRes := <-fulltextCh

	if vectorRes.err != nil {
		return nil, errors.WrapOp(errors.CodeDatabase, "Search", "vector search failed", vectorRes.err)
	}

	if fulltextRes.err != nil {
		fulltextRes.hits = nil
	}

	merged := s.mergeSearchResults(vectorRes.hits, fulltextRes.hits, *opts.VectorWeight, *opts.FulltextWeight)

	if len(opts.SourceFilter) > 0 {
		merged = s.filterBySource(merged, opts.SourceFilter)
	}

	result := s.filterAndLimitHits(merged, *opts.MinScore, opts.Limit)

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

	// Normalize query embedding once for dot-product search
	// (stored embeddings are pre-normalized, so dot product = cosine similarity)
	var queryNorm []float32
	if embedding != nil && len(embedding) > 0 {
		queryNorm = vector.Normalize(embedding)
	}

	// Try HNSW index search first (O(log n) vs O(n) brute force)
	if s.vectorIdx != nil && s.vectorIdx.IsLoaded() && s.vectorIdx.Size() > 0 && queryNorm != nil {
		hits, err := s.searchVectorHNSW(ctx, queryNorm, opts)
		if err == nil && len(hits) > 0 {
			return hits, nil
		}
		// Fall through to brute force on HNSW error
	}

	// Brute force fallback: full table scan with TopK heap
	return s.searchVectorBruteForce(ctx, queryNorm, opts)
}

// searchVectorHNSW uses the HNSW index for fast approximate nearest neighbor search.
func (s *Store) searchVectorHNSW(ctx context.Context, queryNorm []float32, opts *repository.SearchOptions) ([]*entity.SearchHit, error) {
	searchLimit := opts.Limit * 2

	// Get candidate IDs from HNSW index
	candidateIDs := s.vectorIdx.Search(queryNorm, searchLimit)
	if len(candidateIDs) == 0 {
		return nil, nil
	}

	// Fetch full memory data for candidates from SQLite
	placeholders := make([]string, len(candidateIDs))
	args := make([]interface{}, len(candidateIDs))
	for i, id := range candidateIDs {
		placeholders[i] = "?"
		args[i] = id
	}

	query := fmt.Sprintf(`
		SELECT id, path, start_line, end_line, content, embedding, source, created_at
		FROM memories
		WHERE id IN (%s)
	`, strings.Join(placeholders, ","))

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, errors.WrapOp(errors.CodeDatabase, "searchVectorHNSW", "query candidates failed", err)
	}
	defer rows.Close()

	// Build a map of candidate data
	type candidateData struct {
		id        string
		path      string
		startLine int
		endLine   int
		content   string
		source    string
		createdAt int64
		embedding []float32
	}
	candidateMap := make(map[string]*candidateData, len(candidateIDs))
	for rows.Next() {
		var id, path, content, source string
		var startLine, endLine int
		var embeddingBlob []byte
		var createdAt int64

		if err := rows.Scan(&id, &path, &startLine, &endLine, &content, &embeddingBlob, &source, &createdAt); err != nil {
			continue
		}

		memEmbedding, err := deserializeEmbedding(embeddingBlob)
		if err != nil {
			continue
		}

		candidateMap[id] = &candidateData{
			id: id, path: path, startLine: startLine, endLine: endLine,
			content: content, source: source, createdAt: createdAt,
			embedding: memEmbedding,
		}
	}
	if err := rows.Err(); err != nil {
		return nil, errors.WrapOp(errors.CodeDatabase, "searchVectorHNSW", "iterate rows failed", err)
	}

	// Compute dot-product scores and build results
	var hits []*entity.SearchHit
	for _, id := range candidateIDs {
		c, ok := candidateMap[id]
		if !ok {
			continue
		}

		score := vector.DotProduct(queryNorm, c.embedding)
		snippet := truncateContent(c.content, 200)

		hits = append(hits, &entity.SearchHit{
			Entry: &entity.Entry{
				ID:        c.id,
				Path:      c.path,
				StartLine: c.startLine,
				EndLine:   c.endLine,
				Snippet:   snippet,
				Score:     score,
				Source:    entity.SourceType(c.source),
				Timestamp: time.Unix(c.createdAt, 0),
			},
			Embedding: c.embedding,
		})
	}

	return hits, nil
}

// searchVectorBruteForce performs a brute-force vector search using TopK heap.
func (s *Store) searchVectorBruteForce(ctx context.Context, queryNorm []float32, opts *repository.SearchOptions) ([]*entity.SearchHit, error) {
	query := `
		SELECT id, path, start_line, end_line, content, embedding, source, created_at
		FROM memories
		WHERE embedding IS NOT NULL
		ORDER BY created_at DESC
	`

	rows, err := s.db.QueryContext(ctx, query)
	if err != nil {
		return nil, errors.WrapOp(errors.CodeDatabase, "SearchVector", "query memories failed", err)
	}
	defer rows.Close()

	// Use TopK heap to avoid collecting and sorting all results
	searchLimit := opts.Limit * 2
	topK := vector.NewTopKHeap(searchLimit)

	// Store row data for top-K candidates
	type rowData struct {
		id        string
		path      string
		startLine int
		endLine   int
		content   string
		source    string
		createdAt int64
		embedding []float32
		score     float64
	}
	var candidates []rowData

	for rows.Next() {
		var id, path, content, source string
		var startLine, endLine int
		var embeddingBlob []byte
		var createdAt int64

		err := rows.Scan(&id, &path, &startLine, &endLine, &content, &embeddingBlob, &source, &createdAt)
		if err != nil {
			return nil, errors.WrapOp(errors.CodeDatabase, "SearchVector", "scan row failed", err)
		}

		memEmbedding, err := deserializeEmbedding(embeddingBlob)
		if err != nil {
			continue
		}

		var score float64
		if queryNorm != nil {
			score = vector.DotProduct(queryNorm, memEmbedding)
		} else {
			score = 0.5
		}

		// Only keep candidate if it qualifies for top-K
		if !topK.Full() || score > topK.MinScore() {
			topK.Add(id, score)
			candidates = append(candidates, rowData{
				id: id, path: path, startLine: startLine, endLine: endLine,
				content: content, source: source, createdAt: createdAt,
				embedding: memEmbedding, score: score,
			})
		}
	}
	if err := rows.Err(); err != nil {
		return nil, errors.WrapOp(errors.CodeDatabase, "SearchVector", "iterate rows failed", err)
	}

	// Build result from top-K items
	topItems := topK.Items()
	itemMap := make(map[string]rowData, len(candidates))
	for _, c := range candidates {
		itemMap[c.id] = c
	}

	var hits []*entity.SearchHit
	for _, item := range topItems {
		if c, ok := itemMap[item.ID]; ok {
			snippet := truncateContent(c.content, 200)
			hits = append(hits, &entity.SearchHit{
				Entry: &entity.Entry{
					ID:        c.id,
					Path:      c.path,
					StartLine: c.startLine,
					EndLine:   c.endLine,
					Snippet:   snippet,
					Score:     item.Score,
					Source:    entity.SourceType(c.source),
					Timestamp: time.Unix(c.createdAt, 0),
				},
				Embedding: c.embedding,
			})
		}
	}

	return hits, nil
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
		return nil, errors.WrapOp(errors.CodeDatabase, "SearchFulltext", "fulltext search query failed", err)
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

package sqlite

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/xgsong/MyMemoryGo/internal/domain/entity"
	"github.com/xgsong/MyMemoryGo/internal/domain/errors"
	"github.com/xgsong/MyMemoryGo/internal/domain/repository"
)

// Get retrieves a memory entry by its ID.
func (s *Store) Get(ctx context.Context, id string) (*entity.Memory, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var memory entity.Memory
	var source, metadataJSON string
	var embeddingBlob []byte
	var createdAt, updatedAt int64

	err := s.stmtGet.QueryRowContext(ctx, id).Scan(
		&memory.ID,
		&memory.Path,
		&memory.StartLine,
		&memory.EndLine,
		&memory.Content,
		&embeddingBlob,
		&source,
		&createdAt,
		&updatedAt,
		&memory.Checksum,
		&metadataJSON,
	)

	if err == sql.ErrNoRows {
		return nil, errors.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("query memory: %w", err)
	}

	// Deserialize fields
	if err := populateMemoryFromDB(&memory, source, metadataJSON, embeddingBlob, createdAt, updatedAt); err != nil {
		return nil, fmt.Errorf("populate memory: %w", err)
	}

	return &memory, nil
}

// GetByPath retrieves all memory entries for a given file path.
func (s *Store) GetByPath(ctx context.Context, path string) ([]*entity.Memory, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	rows, err := s.stmtGetByPath.QueryContext(ctx, path)
	if err != nil {
		return nil, fmt.Errorf("query memories by path: %w", err)
	}
	defer rows.Close()

	var memories []*entity.Memory
	for rows.Next() {
		memory, err := scanMemory(rows)
		if err != nil {
			return nil, err
		}
		memories = append(memories, memory)
	}

	return memories, rows.Err()
}

// List retrieves all memory entries matching the given criteria.
func (s *Store) List(ctx context.Context, opts *repository.ListOptions) ([]*entity.Memory, int, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	// Set defaults
	if opts == nil {
		opts = &repository.ListOptions{}
	}
	if opts.Limit == 0 {
		opts.Limit = 100
	}

	source := ""
	if opts.Source != "" {
		source = string(opts.Source)
	}

	pathPrefix := ""
	if opts.PathPrefix != "" {
		pathPrefix = opts.PathPrefix
	}

	rows, err := s.stmtList.QueryContext(ctx, source, source, pathPrefix, pathPrefix, opts.Limit, opts.Offset)
	if err != nil {
		return nil, 0, fmt.Errorf("query memories: %w", err)
	}
	defer rows.Close()

	var memories []*entity.Memory
	for rows.Next() {
		memory, err := scanMemory(rows)
		if err != nil {
			return nil, 0, err
		}
		memories = append(memories, memory)
	}

	// Get total count (approximate)
	var total int
	err = s.db.QueryRow("SELECT COUNT(*) FROM memories").Scan(&total)
	if err != nil {
		return nil, 0, fmt.Errorf("count memories: %w", err)
	}

	return memories, total, rows.Err()
}

// populateMemoryFromDB populates a Memory entity with deserialized fields from the database.
func populateMemoryFromDB(memory *entity.Memory, source, metadataJSON string, embeddingBlob []byte, createdAt, updatedAt int64) error {
	memory.Source = entity.SourceType(source)
	memory.CreatedAt = time.Unix(createdAt, 0)
	memory.UpdatedAt = time.Unix(updatedAt, 0)

	if embeddingBlob != nil {
		var err error
		memory.Embedding, err = deserializeEmbedding(embeddingBlob)
		if err != nil {
			return fmt.Errorf("deserialize embedding: %w", err)
		}
	}

	if metadataJSON != "" {
		if err := json.Unmarshal([]byte(metadataJSON), &memory.Metadata); err != nil {
			return fmt.Errorf("deserialize metadata: %w", err)
		}
	}

	return nil
}

// scanMemory scans a row into a Memory entity.
func scanMemory(rows *sql.Rows) (*entity.Memory, error) {
	var memory entity.Memory
	var source, metadataJSON string
	var embeddingBlob []byte
	var createdAt, updatedAt int64

	err := rows.Scan(
		&memory.ID,
		&memory.Path,
		&memory.StartLine,
		&memory.EndLine,
		&memory.Content,
		&embeddingBlob,
		&source,
		&createdAt,
		&updatedAt,
		&memory.Checksum,
		&metadataJSON,
	)
	if err != nil {
		return nil, fmt.Errorf("scan memory: %w", err)
	}

	// Deserialize fields
	if err := populateMemoryFromDB(&memory, source, metadataJSON, embeddingBlob, createdAt, updatedAt); err != nil {
		return nil, err
	}

	return &memory, nil
}

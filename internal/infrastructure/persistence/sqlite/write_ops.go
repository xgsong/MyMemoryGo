package sqlite

import (
	"context"
	"encoding/json"

	"github.com/xgsong/MyMemoryGo/internal/domain/entity"
	"github.com/xgsong/MyMemoryGo/internal/pkg/errors"
	"github.com/xgsong/MyMemoryGo/internal/pkg/vector"
)

// prepareMemoryForStorage serializes embedding and metadata for storage.
func prepareMemoryForStorage(memory *entity.Memory) (embeddingBlob, metadataJSON []byte, err error) {
	if memory.Embedding != nil {
		// Normalize embedding before storage for faster dot-product search
		normalized := vector.Normalize(memory.Embedding)
		embeddingBlob, err = serializeEmbedding(normalized)
		if err != nil {
			return nil, nil, errors.WrapOp(errors.CodeDatabase, "prepareMemoryForStorage", "serialize embedding failed", err)
		}
	}

	if memory.Metadata != nil {
		metadataJSON, err = json.Marshal(memory.Metadata)
		if err != nil {
			return nil, nil, errors.WrapOp(errors.CodeDatabase, "prepareMemoryForStorage", "serialize metadata failed", err)
		}
	}

	return embeddingBlob, metadataJSON, nil
}

// Store saves a memory entry to the repository.
func (s *Store) Store(ctx context.Context, memory *entity.Memory) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	embeddingBlob, metadataJSON, err := prepareMemoryForStorage(memory)
	if err != nil {
		return err
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return errors.WrapOp(errors.CodeDatabase, "Store", "begin transaction failed", err)
	}
	defer tx.Rollback()

	_, err = tx.StmtContext(ctx, s.stmtStore).ExecContext(ctx,
		memory.ID,
		memory.Path,
		memory.StartLine,
		memory.EndLine,
		memory.Content,
		embeddingBlob,
		string(memory.Source),
		memory.CreatedAt.Unix(),
		memory.UpdatedAt.Unix(),
		memory.Checksum,
		string(metadataJSON),
	)
	if err != nil {
		return errors.WrapOp(errors.CodeDatabase, "Store", "insert memory failed", err)
	}

	_, err = tx.ExecContext(ctx,
		"INSERT OR REPLACE INTO memories_fts(id, path, content, source) VALUES (?, ?, ?, ?)",
		memory.ID,
		memory.Path,
		memory.Content,
		string(memory.Source),
	)
	if err != nil {
		return errors.WrapOp(errors.CodeDatabase, "Store", "update FTS index failed", err)
	}

	if err := tx.Commit(); err != nil {
		return errors.WrapOp(errors.CodeDatabase, "Store", "commit failed", err)
	}

	// Update HNSW vector index after successful commit
	if s.vectorIdx != nil && memory.Embedding != nil {
		s.vectorIdx.Add(memory.ID, memory.Embedding)
	}

	return nil
}

// StoreBatch saves multiple memory entries in a single transaction.
func (s *Store) StoreBatch(ctx context.Context, memories []*entity.Memory) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return errors.WrapOp(errors.CodeDatabase, "StoreBatch", "begin transaction failed", err)
	}
	defer tx.Rollback()

	for _, memory := range memories {
		embeddingBlob, metadataJSON, err := prepareMemoryForStorage(memory)
		if err != nil {
			return err
		}

		_, err = tx.ExecContext(ctx,
			`INSERT OR REPLACE INTO memories
			(id, path, start_line, end_line, content, embedding, source, created_at, updated_at, checksum, metadata)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			memory.ID,
			memory.Path,
			memory.StartLine,
			memory.EndLine,
			memory.Content,
			embeddingBlob,
			string(memory.Source),
			memory.CreatedAt.Unix(),
			memory.UpdatedAt.Unix(),
			memory.Checksum,
			string(metadataJSON),
		)
		if err != nil {
			return errors.WrapOp(errors.CodeDatabase, "StoreBatch", "insert memory failed", err)
		}

		_, err = tx.ExecContext(ctx,
			"INSERT OR REPLACE INTO memories_fts(id, path, content, source) VALUES (?, ?, ?, ?)",
			memory.ID,
			memory.Path,
			memory.Content,
			string(memory.Source),
		)
		if err != nil {
			return errors.WrapOp(errors.CodeDatabase, "StoreBatch", "update FTS index failed", err)
		}
	}

	if err := tx.Commit(); err != nil {
		return errors.WrapOp(errors.CodeDatabase, "StoreBatch", "commit failed", err)
	}

	// Update HNSW vector index after successful commit
	for _, memory := range memories {
		if s.vectorIdx != nil && memory.Embedding != nil {
			s.vectorIdx.Add(memory.ID, memory.Embedding)
		}
	}

	return nil
}

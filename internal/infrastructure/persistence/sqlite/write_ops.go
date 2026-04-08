package sqlite

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/xgsong/MyMemoryGo/internal/domain/entity"
)

// prepareMemoryForStorage serializes embedding and metadata for storage.
func prepareMemoryForStorage(memory *entity.Memory) (embeddingBlob, metadataJSON []byte, err error) {
	if memory.Embedding != nil {
		embeddingBlob, err = serializeEmbedding(memory.Embedding)
		if err != nil {
			return nil, nil, fmt.Errorf("serialize embedding for %s: %w", memory.ID, err)
		}
	}

	if memory.Metadata != nil {
		metadataJSON, err = json.Marshal(memory.Metadata)
		if err != nil {
			return nil, nil, fmt.Errorf("serialize metadata for %s: %w", memory.ID, err)
		}
	}

	return embeddingBlob, metadataJSON, nil
}

// Store saves a memory entry to the repository.
func (s *Store) Store(ctx context.Context, memory *entity.Memory) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Serialize memory for storage
	embeddingBlob, metadataJSON, err := prepareMemoryForStorage(memory)
	if err != nil {
		return err
	}

	// Begin transaction
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer tx.Rollback()

	// Insert memory
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
		return fmt.Errorf("insert memory: %w", err)
	}

	// Update FTS index
	_, err = tx.ExecContext(ctx,
		"INSERT OR REPLACE INTO memories_fts(id, path, content, source) VALUES (?, ?, ?, ?)",
		memory.ID,
		memory.Path,
		memory.Content,
		string(memory.Source),
	)
	if err != nil {
		return fmt.Errorf("update FTS index: %w", err)
	}

	return tx.Commit()
}

// StoreBatch saves multiple memory entries in a single transaction.
func (s *Store) StoreBatch(ctx context.Context, memories []*entity.Memory) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer tx.Rollback()

	for _, memory := range memories {
		// Serialize memory for storage
		embeddingBlob, metadataJSON, err := prepareMemoryForStorage(memory)
		if err != nil {
			return err
		}

		// Insert memory
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
			return fmt.Errorf("insert memory %s: %w", memory.ID, err)
		}

		// Update FTS index
		_, err = tx.ExecContext(ctx,
			"INSERT OR REPLACE INTO memories_fts(id, path, content, source) VALUES (?, ?, ?, ?)",
			memory.ID,
			memory.Path,
			memory.Content,
			string(memory.Source),
		)
		if err != nil {
			return fmt.Errorf("update FTS index: %w", err)
		}
	}

	return tx.Commit()
}

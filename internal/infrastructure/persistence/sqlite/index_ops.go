package sqlite

import (
	"context"
	"fmt"

	"github.com/xgsong/MyMemoryGo/internal/domain/entity"
)

func (s *Store) Index(ctx context.Context, memories []*entity.Memory) error {
	if len(memories) == 0 {
		return nil
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer tx.Rollback()

	for _, mem := range memories {
		_, err := tx.ExecContext(ctx,
			`INSERT OR REPLACE INTO memories_fts (id, path, content, source) VALUES (?, ?, ?, ?)`,
			mem.ID, mem.Path, mem.Content, string(mem.Source),
		)
		if err != nil {
			return fmt.Errorf("update fts index for %s: %w", mem.ID, err)
		}
	}

	return tx.Commit()
}

func (s *Store) RemoveFromIndex(ctx context.Context, ids []string) error {
	if len(ids) == 0 {
		return nil
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer tx.Rollback()

	for _, id := range ids {
		_, err := tx.ExecContext(ctx, `DELETE FROM memories_fts WHERE id = ?`, id)
		if err != nil {
			return fmt.Errorf("remove from fts index %s: %w", id, err)
		}
	}

	return tx.Commit()
}

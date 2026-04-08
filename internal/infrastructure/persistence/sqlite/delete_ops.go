package sqlite

import (
	"context"
	"fmt"

	"github.com/xgsong/MyMemoryGo/internal/domain/errors"
)

// Delete removes a memory entry by ID.
func (s *Store) Delete(ctx context.Context, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer tx.Rollback()

	// Delete from memories table
	result, err := tx.StmtContext(ctx, s.stmtDelete).ExecContext(ctx, id)
	if err != nil {
		return fmt.Errorf("delete memory: %w", err)
	}

	// Check if memory existed
	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("get rows affected: %w", err)
	}
	if rows == 0 {
		return errors.ErrNotFound
	}

	// Delete from FTS index
	_, err = tx.ExecContext(ctx, "DELETE FROM memories_fts WHERE id = ?", id)
	if err != nil {
		return fmt.Errorf("delete from FTS: %w", err)
	}

	return tx.Commit()
}

// DeleteByPath removes all memory entries for a given file path.
func (s *Store) DeleteByPath(ctx context.Context, path string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer tx.Rollback()

	// Delete from memories table
	_, err = tx.StmtContext(ctx, s.stmtDeleteByPath).ExecContext(ctx, path)
	if err != nil {
		return fmt.Errorf("delete memories: %w", err)
	}

	// Delete from FTS index
	_, err = tx.ExecContext(ctx, "DELETE FROM memories_fts WHERE path = ?", path)
	if err != nil {
		return fmt.Errorf("delete from FTS: %w", err)
	}

	return tx.Commit()
}

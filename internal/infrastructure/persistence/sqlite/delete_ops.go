package sqlite

import (
	"context"

	"github.com/xgsong/MyMemoryGo/internal/pkg/errors"
)

// Delete removes a memory entry by ID.
func (s *Store) Delete(ctx context.Context, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return errors.WrapOp(errors.CodeDatabase, "Delete", "begin transaction failed", err)
	}
	defer tx.Rollback()

	result, err := tx.StmtContext(ctx, s.stmtDelete).ExecContext(ctx, id)
	if err != nil {
		return errors.WrapOp(errors.CodeDatabase, "Delete", "delete memory failed", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return errors.WrapOp(errors.CodeDatabase, "Delete", "get rows affected failed", err)
	}
	if rows == 0 {
		return errors.ErrNotFound
	}

	_, err = tx.ExecContext(ctx, "DELETE FROM memories_fts WHERE id = ?", id)
	if err != nil {
		return errors.WrapOp(errors.CodeDatabase, "Delete", "delete from FTS failed", err)
	}

	if err := tx.Commit(); err != nil {
		return errors.WrapOp(errors.CodeDatabase, "Delete", "commit failed", err)
	}

	// Update HNSW vector index after successful delete
	if s.vectorIdx != nil {
		s.vectorIdx.Remove(id)
	}

	return nil
}

// DeleteByPath removes all memory entries for a given file path.
func (s *Store) DeleteByPath(ctx context.Context, path string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Query IDs to be deleted for HNSW index update
	var deletedIDs []string
	if s.vectorIdx != nil {
		idRows, err := s.db.QueryContext(ctx, "SELECT id FROM memories WHERE path = ?", path)
		if err == nil {
			for idRows.Next() {
				var id string
				if idRows.Scan(&id) == nil {
					deletedIDs = append(deletedIDs, id)
				}
			}
			idRows.Close()
		}
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return errors.WrapOp(errors.CodeDatabase, "DeleteByPath", "begin transaction failed", err)
	}
	defer tx.Rollback()

	_, err = tx.StmtContext(ctx, s.stmtDeleteByPath).ExecContext(ctx, path)
	if err != nil {
		return errors.WrapOp(errors.CodeDatabase, "DeleteByPath", "delete memories failed", err)
	}

	_, err = tx.ExecContext(ctx, "DELETE FROM memories_fts WHERE path = ?", path)
	if err != nil {
		return errors.WrapOp(errors.CodeDatabase, "DeleteByPath", "delete from FTS failed", err)
	}

	if err := tx.Commit(); err != nil {
		return errors.WrapOp(errors.CodeDatabase, "DeleteByPath", "commit failed", err)
	}

	// Update HNSW vector index after successful delete
	for _, id := range deletedIDs {
		s.vectorIdx.Remove(id)
	}

	return nil
}

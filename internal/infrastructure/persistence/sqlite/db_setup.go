package sqlite

import (
	"database/sql"
	"fmt"
	"math"

	"github.com/xgsong/MyMemoryGo/internal/pkg/errors"
)

func createSchema(db *sql.DB, vectorDims int) error {
	schema := `
	-- Metadata table
	CREATE TABLE IF NOT EXISTS metadata (
		path TEXT PRIMARY KEY,
		size INTEGER NOT NULL,
		mtime INTEGER NOT NULL,
		checksum TEXT NOT NULL,
		indexed_at INTEGER NOT NULL
	);

	-- Memory entries table
	CREATE TABLE IF NOT EXISTS memories (
		id TEXT PRIMARY KEY,
		path TEXT NOT NULL,
		start_line INTEGER NOT NULL,
		end_line INTEGER NOT NULL,
		content TEXT NOT NULL,
		embedding BLOB,
		source TEXT NOT NULL,
		created_at INTEGER NOT NULL,
		updated_at INTEGER NOT NULL,
		checksum TEXT NOT NULL,
		metadata TEXT
	);

	-- Embedding cache table
	CREATE TABLE IF NOT EXISTS embedding_cache (
		content_hash TEXT PRIMARY KEY,
		embedding BLOB NOT NULL,
		provider TEXT NOT NULL,
		model TEXT NOT NULL,
		created_at INTEGER NOT NULL
	);

	-- Full-text search index
	CREATE VIRTUAL TABLE IF NOT EXISTS memories_fts USING fts5(
		id,
		path,
		content,
		source,
		tokenize = 'porter unicode61'
	);

	-- Indexes for performance
	CREATE INDEX IF NOT EXISTS idx_memories_path ON memories(path);
	CREATE INDEX IF NOT EXISTS idx_memories_source ON memories(source);
	CREATE INDEX IF NOT EXISTS idx_memories_created_at ON memories(created_at);
	CREATE INDEX IF NOT EXISTS idx_embedding_cache_provider ON embedding_cache(provider, model);
	`

	_, err := db.Exec(schema)
	return err
}

func (s *Store) prepareStatements() error {
	var err error

	s.stmtStore, err = s.db.Prepare(`
		INSERT OR REPLACE INTO memories
		(id, path, start_line, end_line, content, embedding, source, created_at, updated_at, checksum, metadata)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`)
	if err != nil {
		return err
	}

	s.stmtGet, err = s.db.Prepare(`
		SELECT id, path, start_line, end_line, content, embedding, source, created_at, updated_at, checksum, metadata
		FROM memories WHERE id = ?
	`)
	if err != nil {
		return err
	}

	s.stmtGetByPath, err = s.db.Prepare(`
		SELECT id, path, start_line, end_line, content, embedding, source, created_at, updated_at, checksum, metadata
		FROM memories WHERE path = ? ORDER BY start_line
	`)
	if err != nil {
		return err
	}

	s.stmtDelete, err = s.db.Prepare(`DELETE FROM memories WHERE id = ?`)
	if err != nil {
		return err
	}

	s.stmtDeleteByPath, err = s.db.Prepare(`DELETE FROM memories WHERE path = ?`)
	if err != nil {
		return err
	}

	s.stmtList, err = s.db.Prepare(`
		SELECT id, path, start_line, end_line, content, embedding, source, created_at, updated_at, checksum, metadata
		FROM memories
		WHERE (? = '' OR source = ?)
		  AND (? = '' OR path LIKE ? || '%')
		ORDER BY created_at DESC
		LIMIT ? OFFSET ?
	`)
	if err != nil {
		return err
	}

	s.stmtSearchFulltext, err = s.db.Prepare(`
		SELECT m.id, m.path, m.start_line, m.end_line, m.content, m.source, m.created_at,
		       bm25(memories_fts) as rank
		FROM memories_fts fts
		JOIN memories m ON fts.id = m.id
		WHERE memories_fts MATCH ?
		ORDER BY rank
		LIMIT ?
	`)
	if err != nil {
		return err
	}

	return nil
}

func serializeEmbedding(embedding []float32) ([]byte, error) {
	data := make([]byte, len(embedding)*4)
	for i, v := range embedding {
		bits := float32ToBits(v)
		data[i*4] = byte(bits)
		data[i*4+1] = byte(bits >> 8)
		data[i*4+2] = byte(bits >> 16)
		data[i*4+3] = byte(bits >> 24)
	}
	return data, nil
}

func deserializeEmbedding(data []byte) ([]float32, error) {
	if len(data)%4 != 0 {
		return nil, errors.WrapOp(errors.CodeInvalidInput, "deserializeEmbedding", "invalid embedding data length", fmt.Errorf("length: %d", len(data)))
	}

	embedding := make([]float32, len(data)/4)
	for i := range embedding {
		bits := uint32(data[i*4]) | uint32(data[i*4+1])<<8 | uint32(data[i*4+2])<<16 | uint32(data[i*4+3])<<24
		embedding[i] = bitsToFloat32(bits)
	}

	return embedding, nil
}

func float32ToBits(f float32) uint32 {
	return math.Float32bits(f)
}

func bitsToFloat32(bits uint32) float32 {
	return math.Float32frombits(bits)
}

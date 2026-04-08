// Package sqlite provides SQLite-based storage implementation for memories.
package sqlite

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"sync"

	"github.com/xgsong/MyMemoryGo/internal/infrastructure/search"
	"github.com/xgsong/MyMemoryGo/internal/pkg/errors"
	"github.com/xgsong/MyMemoryGo/internal/pkg/vector"
	_ "modernc.org/sqlite" // Pure Go SQLite driver (no CGO required)
)

// Config holds configuration for SQLite storage.
type Config struct {
	// DBPath is the path to the SQLite database file.
	DBPath string `json:"db_path" yaml:"db_path"`

	// WALMode enables Write-Ahead Logging for better performance.
	WALMode bool `json:"wal_mode" yaml:"wal_mode"`

	// VectorDimensions is the dimensionality of embedding vectors.
	VectorDimensions int `json:"vector_dimensions" yaml:"vector_dimensions"`
}

// DefaultConfig returns a Config with sensible defaults.
func DefaultConfig() *Config {
	home, _ := os.UserHomeDir()
	return &Config{
		DBPath:           filepath.Join(home, ".memory", "memory.db"),
		WALMode:          true,
		VectorDimensions: 768,
	}
}

// Store implements repository interfaces using SQLite.
type Store struct {
	db         *sql.DB
	config     *Config
	mu         sync.RWMutex
	vectorIdx  *search.VectorIndex

	// Prepared statements for performance
	stmtStore          *sql.Stmt
	stmtGet            *sql.Stmt
	stmtGetByPath      *sql.Stmt
	stmtDelete         *sql.Stmt
	stmtDeleteByPath   *sql.Stmt
	stmtList           *sql.Stmt
	stmtSearchVector   *sql.Stmt
	stmtSearchFulltext *sql.Stmt
}

// New creates a new SQLite storage instance.
func New(config *Config) (*Store, error) {
	if config == nil {
		config = DefaultConfig()
	}

	// Ensure directory exists
	dir := filepath.Dir(config.DBPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, wrapErr("create directory", err)
	}

	// Open database
	db, err := sql.Open("sqlite", config.DBPath)
	if err != nil {
		return nil, wrapErr("open database", err)
	}

	// Enable WAL mode if configured
	if config.WALMode {
		if _, err := db.Exec("PRAGMA journal_mode=WAL"); err != nil {
			db.Close()
			return nil, wrapErr("enable WAL mode", err)
		}
	}

	// Create schema
	if err := createSchema(db, config.VectorDimensions); err != nil {
		db.Close()
		return nil, wrapErr("create schema", err)
	}

	store := &Store{
		db:     db,
		config: config,
	}

	// Prepare statements
	if err := store.prepareStatements(); err != nil {
		db.Close()
		return nil, wrapErr("prepare statements", err)
	}

	// Run migrations if needed
	if err := store.runMigrations(context.Background()); err != nil {
		db.Close()
		return nil, wrapErr("run migrations", err)
	}

	// Initialize HNSW vector index from existing data
	if err := store.loadVectorIndex(context.Background()); err != nil {
		db.Close()
		return nil, wrapErr("load vector index", err)
	}

	return store, nil
}

// Close closes the database connection and prepared statements.
func (s *Store) Close() error {
	// Close prepared statements
	statements := []*sql.Stmt{
		s.stmtStore,
		s.stmtGet,
		s.stmtGetByPath,
		s.stmtDelete,
		s.stmtDeleteByPath,
		s.stmtList,
		s.stmtSearchFulltext,
	}

	for _, stmt := range statements {
		if stmt != nil {
			stmt.Close()
		}
	}

	return s.db.Close()
}

// SchemaVersion returns the current schema version from the metadata table.
func (s *Store) SchemaVersion() int {
	var version int
	err := s.db.QueryRow("SELECT size FROM metadata WHERE path = '__schema_version'").Scan(&version)
	if err != nil {
		return 0
	}
	return version
}

// runMigrations executes necessary database migrations based on the current schema version.
func (s *Store) runMigrations(ctx context.Context) error {
	version := s.SchemaVersion()
	if version < 2 {
		if err := s.migrateNormalizedEmbeddings(ctx); err != nil {
			return err
		}
	}
	return nil
}

// migrateNormalizedEmbeddings normalizes all existing embeddings in the database.
func (s *Store) migrateNormalizedEmbeddings(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	rows, err := s.db.QueryContext(ctx, "SELECT id, embedding FROM memories WHERE embedding IS NOT NULL")
	if err != nil {
		return errors.WrapOp(errors.CodeDatabase, "migrateNormalizedEmbeddings", "query embeddings failed", err)
	}
	defer rows.Close()

	type idEmbedding struct {
		id        string
		embedding []byte
	}
	var toUpdate []idEmbedding

	for rows.Next() {
		var id string
		var blob []byte
		if err := rows.Scan(&id, &blob); err != nil {
			continue
		}

		emb, err := deserializeEmbedding(blob)
		if err != nil {
			continue
		}

		normalized := vector.Normalize(emb)
		normalizedBlob, err := serializeEmbedding(normalized)
		if err != nil {
			continue
		}

		toUpdate = append(toUpdate, idEmbedding{id: id, embedding: normalizedBlob})
	}
	if err := rows.Err(); err != nil {
		return errors.WrapOp(errors.CodeDatabase, "migrateNormalizedEmbeddings", "iterate rows failed", err)
	}

	if len(toUpdate) == 0 {
		return nil
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return errors.WrapOp(errors.CodeDatabase, "migrateNormalizedEmbeddings", "begin transaction failed", err)
	}
	defer tx.Rollback()

	for _, item := range toUpdate {
		_, err := tx.ExecContext(ctx, "UPDATE memories SET embedding = ? WHERE id = ?", item.embedding, item.id)
		if err != nil {
			return errors.WrapOp(errors.CodeDatabase, "migrateNormalizedEmbeddings", "update embedding failed", err)
		}
	}

	_, err = tx.ExecContext(ctx, "INSERT OR REPLACE INTO metadata (path, size, mtime, checksum, indexed_at) VALUES ('__schema_version', 2, 0, 'normalized_embeddings', 0)")
	if err != nil {
		return errors.WrapOp(errors.CodeDatabase, "migrateNormalizedEmbeddings", "update schema version failed", err)
	}

	return tx.Commit()
}

// loadVectorIndex loads all embeddings from SQLite and builds the HNSW vector index.
func (s *Store) loadVectorIndex(ctx context.Context) error {
	s.vectorIdx = search.NewVectorIndex(search.DefaultVectorIndexConfig(s.config.VectorDimensions))

	rows, err := s.db.QueryContext(ctx, "SELECT id, embedding FROM memories WHERE embedding IS NOT NULL")
	if err != nil {
		return errors.WrapOp(errors.CodeDatabase, "loadVectorIndex", "query embeddings failed", err)
	}
	defer rows.Close()

	embeddings := make(map[string][]float32)
	for rows.Next() {
		var id string
		var blob []byte
		if err := rows.Scan(&id, &blob); err != nil {
			continue
		}
		emb, err := deserializeEmbedding(blob)
		if err != nil {
			continue
		}
		embeddings[id] = emb
	}
	if err := rows.Err(); err != nil {
		return errors.WrapOp(errors.CodeDatabase, "loadVectorIndex", "iterate rows failed", err)
	}

	s.vectorIdx.Build(embeddings)
	return nil
}

// VectorIndex returns the underlying HNSW vector index for external access.
func (s *Store) VectorIndex() *search.VectorIndex {
	return s.vectorIdx
}

// wrapErr wraps an error with context.
func wrapErr(op string, err error) error {
	return &StorageError{Op: op, Err: err}
}

// StorageError represents a storage-related error.
type StorageError struct {
	Op  string
	Err error
}

func (e *StorageError) Error() string {
	return e.Op + ": " + e.Err.Error()
}

func (e *StorageError) Unwrap() error {
	return e.Err
}

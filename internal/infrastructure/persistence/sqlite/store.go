// Package sqlite provides SQLite-based storage implementation for memories.
package sqlite

import (
	"database/sql"
	"os"
	"path/filepath"
	"sync"

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
	db     *sql.DB
	config *Config
	mu     sync.RWMutex

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

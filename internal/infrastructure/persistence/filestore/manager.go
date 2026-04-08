// Package filestore provides file-based storage implementation for memories.
package filestore

import (
	"os"
	"path/filepath"
	"sync"

	"github.com/fsnotify/fsnotify"
	"github.com/xgsong/MyMemoryGo/internal/domain/repository"
)

// Config holds configuration for file-based memory storage.
type Config struct {
	// WorkspaceDir is the root directory for memory files.
	WorkspaceDir string `json:"workspace_dir" yaml:"workspace_dir"`

	// LongTermFile is the filename for long-term memories (default: MEMORY.md).
	LongTermFile string `json:"longterm_file" yaml:"longterm_file"`

	// DailyDir is the directory name for daily logs (default: memory).
	DailyDir string `json:"daily_dir" yaml:"daily_dir"`

	// FileExtension is the file extension (default: .md).
	FileExtension string `json:"file_extension" yaml:"file_extension"`
}

// DefaultConfig returns a Config with sensible defaults.
func DefaultConfig() *Config {
	home, _ := os.UserHomeDir()
	return &Config{
		WorkspaceDir:  filepath.Join(home, ".memory", "workspace"),
		LongTermFile:  "MEMORY.md",
		DailyDir:      "memory",
		FileExtension: ".md",
	}
}

// Manager implements repository.FileRepository for Markdown-based memory storage.
type Manager struct {
	config   *Config
	watcher  *fsnotify.Watcher
	handlers []repository.FileChangeHandler
	mu       sync.RWMutex
}

// New creates a new file manager instance.
func New(config *Config) (*Manager, error) {
	if config == nil {
		config = DefaultConfig()
	}

	// Ensure workspace directory exists
	if err := os.MkdirAll(config.WorkspaceDir, 0755); err != nil {
		return nil, wrapErr("create workspace directory", err)
	}

	// Ensure daily directory exists
	dailyDir := filepath.Join(config.WorkspaceDir, config.DailyDir)
	if err := os.MkdirAll(dailyDir, 0755); err != nil {
		return nil, wrapErr("create daily directory", err)
	}

	return &Manager{
		config:   config,
		handlers: make([]repository.FileChangeHandler, 0),
	}, nil
}

// Close closes the file manager and releases resources.
func (fm *Manager) Close() error {
	fm.mu.Lock()
	defer fm.mu.Unlock()

	if fm.watcher != nil {
		return fm.watcher.Close()
	}
	return nil
}

// wrapErr wraps an error with context.
func wrapErr(op string, err error) error {
	return &FileError{Op: op, Err: err}
}

// FileError represents a file-related error.
type FileError struct {
	Op  string
	Err error
}

func (e *FileError) Error() string {
	return e.Op + ": " + e.Err.Error()
}

func (e *FileError) Unwrap() error {
	return e.Err
}

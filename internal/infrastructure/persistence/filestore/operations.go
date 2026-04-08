package filestore

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/xgsong/MyMemoryGo/internal/domain/errors"
)

// Read reads the content of a file.
func (fm *Manager) Read(ctx context.Context, path string) ([]byte, error) {
	fm.mu.RLock()
	defer fm.mu.RUnlock()

	// Resolve path
	fullPath := fm.resolvePath(path)

	// Check if file exists
	if _, err := os.Stat(fullPath); os.IsNotExist(err) {
		return nil, errors.ErrNotFound
	}

	// Read file
	content, err := os.ReadFile(fullPath)
	if err != nil {
		return nil, fmt.Errorf("read file: %w", err)
	}

	return content, nil
}

// Write writes content to a file.
func (fm *Manager) Write(ctx context.Context, path string, content []byte) error {
	fm.mu.Lock()
	defer fm.mu.Unlock()

	// Resolve path
	fullPath := fm.resolvePath(path)

	// Ensure directory exists
	dir := filepath.Dir(fullPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("create directory: %w", err)
	}

	// Write file
	if err := os.WriteFile(fullPath, content, 0644); err != nil {
		return fmt.Errorf("write file: %w", err)
	}

	return nil
}

// Append appends content to an existing file.
func (fm *Manager) Append(ctx context.Context, path string, content []byte) error {
	fm.mu.Lock()
	defer fm.mu.Unlock()

	// Resolve path
	fullPath := fm.resolvePath(path)

	// Check if file exists
	exists := fm.exists(fullPath)

	// Open file in append mode
	file, err := os.OpenFile(fullPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("open file: %w", err)
	}
	defer file.Close()

	// Add newline if file exists and content doesn't start with newline
	if exists && len(content) > 0 && content[0] != '\n' {
		if _, err := file.WriteString("\n"); err != nil {
			return fmt.Errorf("write newline: %w", err)
		}
	}

	// Write content
	if _, err := file.Write(content); err != nil {
		return fmt.Errorf("append content: %w", err)
	}

	return nil
}

// Delete removes a file.
func (fm *Manager) Delete(ctx context.Context, path string) error {
	fm.mu.Lock()
	defer fm.mu.Unlock()

	// Resolve path
	fullPath := fm.resolvePath(path)

	// Check if file exists
	if _, err := os.Stat(fullPath); os.IsNotExist(err) {
		return errors.ErrNotFound
	}

	// Delete file
	if err := os.Remove(fullPath); err != nil {
		return fmt.Errorf("delete file: %w", err)
	}

	return nil
}

// Exists checks if a file exists.
func (fm *Manager) Exists(ctx context.Context, path string) (bool, error) {
	fm.mu.RLock()
	defer fm.mu.RUnlock()

	fullPath := fm.resolvePath(path)
	return fm.exists(fullPath), nil
}

// List lists files matching a pattern.
func (fm *Manager) List(ctx context.Context, pattern string) ([]string, error) {
	fm.mu.RLock()
	defer fm.mu.RUnlock()

	// Build search path
	searchPath := filepath.Join(fm.config.WorkspaceDir, pattern)

	// Find matching files
	var files []string
	err := filepath.Walk(fm.config.WorkspaceDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}

		// Check if file matches pattern
		matched, err := filepath.Match(searchPath, path)
		if err != nil {
			return err
		}
		if matched {
			// Return relative path
			relPath, err := filepath.Rel(fm.config.WorkspaceDir, path)
			if err != nil {
				return err
			}
			files = append(files, relPath)
		}

		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("list files: %w", err)
	}

	return files, nil
}

// Helper methods

func (fm *Manager) resolvePath(path string) string {
	// If path is absolute, use as-is
	if filepath.IsAbs(path) {
		return path
	}

	// If path is relative to workspace
	return filepath.Join(fm.config.WorkspaceDir, path)
}

func (fm *Manager) exists(path string) bool {
	_, err := os.Stat(path)
	return !os.IsNotExist(err)
}

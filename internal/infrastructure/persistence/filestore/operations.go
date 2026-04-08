package filestore

import (
	"context"
	"os"
	"path/filepath"

	"github.com/xgsong/MyMemoryGo/internal/pkg/errors"
)

// Read reads content of a file.
func (fm *Manager) Read(ctx context.Context, path string) ([]byte, error) {
	fm.mu.RLock()
	defer fm.mu.RUnlock()

	fullPath := fm.resolvePath(path)

	if _, err := os.Stat(fullPath); os.IsNotExist(err) {
		return nil, errors.ErrNotFound
	}

	content, err := os.ReadFile(fullPath)
	if err != nil {
		return nil, errors.WrapOp(errors.CodeFilesystem, "Read", "read file failed", err)
	}

	return content, nil
}

// Write writes content to a file.
func (fm *Manager) Write(ctx context.Context, path string, content []byte) error {
	fm.mu.Lock()
	defer fm.mu.Unlock()

	fullPath := fm.resolvePath(path)

	dir := filepath.Dir(fullPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return errors.WrapOp(errors.CodeFilesystem, "Write", "create directory failed", err)
	}

	if err := os.WriteFile(fullPath, content, 0644); err != nil {
		return errors.WrapOp(errors.CodeFilesystem, "Write", "write file failed", err)
	}

	return nil
}

// Append appends content to an existing file.
func (fm *Manager) Append(ctx context.Context, path string, content []byte) error {
	fm.mu.Lock()
	defer fm.mu.Unlock()

	fullPath := fm.resolvePath(path)

	exists := fm.exists(fullPath)

	file, err := os.OpenFile(fullPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return errors.WrapOp(errors.CodeFilesystem, "Append", "open file failed", err)
	}
	defer file.Close()

	if exists && len(content) > 0 && content[0] != '\n' {
		if _, err := file.WriteString("\n"); err != nil {
			return errors.WrapOp(errors.CodeFilesystem, "Append", "write newline failed", err)
		}
	}

	if _, err := file.Write(content); err != nil {
		return errors.WrapOp(errors.CodeFilesystem, "Append", "append content failed", err)
	}

	return nil
}

// Delete removes a file.
func (fm *Manager) Delete(ctx context.Context, path string) error {
	fm.mu.Lock()
	defer fm.mu.Unlock()

	fullPath := fm.resolvePath(path)

	if _, err := os.Stat(fullPath); os.IsNotExist(err) {
		return errors.ErrNotFound
	}

	if err := os.Remove(fullPath); err != nil {
		return errors.WrapOp(errors.CodeFilesystem, "Delete", "delete file failed", err)
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

	searchPath := filepath.Join(fm.config.WorkspaceDir, pattern)

	var files []string
	err := filepath.Walk(fm.config.WorkspaceDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}

		matched, err := filepath.Match(searchPath, path)
		if err != nil {
			return err
		}
		if matched {
			relPath, err := filepath.Rel(fm.config.WorkspaceDir, path)
			if err != nil {
				return err
			}
			files = append(files, relPath)
		}

		return nil
	})

	if err != nil {
		return nil, errors.WrapOp(errors.CodeFilesystem, "List", "list files failed", err)
	}

	return files, nil
}

// Helper methods

func (fm *Manager) resolvePath(path string) string {
	// Clean the path to remove any .. or . elements
	cleaned := filepath.Clean(path)

	// If path is absolute or tries to escape workspace, force it within workspace
	if filepath.IsAbs(cleaned) {
		// Check if the absolute path is within the workspace
		absWorkspace, err := filepath.Abs(fm.config.WorkspaceDir)
		if err != nil {
			absWorkspace = fm.config.WorkspaceDir
		}
		if !isSubPath(absWorkspace, cleaned) {
			// Path traversal attempt — confine to workspace
			return filepath.Join(absWorkspace, filepath.Base(cleaned))
		}
		return cleaned
	}

	// For relative paths, join with workspace and verify it stays within
	joined := filepath.Join(fm.config.WorkspaceDir, cleaned)
	absWorkspace, err := filepath.Abs(fm.config.WorkspaceDir)
	if err != nil {
		absWorkspace = fm.config.WorkspaceDir
	}
	if !isSubPath(absWorkspace, joined) {
		// Path traversal attempt — confine to workspace
		return filepath.Join(absWorkspace, filepath.Base(cleaned))
	}

	return joined
}

// isSubPath checks if target is within base directory.
func isSubPath(base, target string) bool {
	// Ensure both paths are absolute and cleaned
	absBase, err := filepath.Abs(base)
	if err != nil {
		return false
	}
	absTarget, err := filepath.Abs(target)
	if err != nil {
		return false
	}
	// Ensure base ends with separator for proper prefix matching
	if len(absBase) > 0 && absBase[len(absBase)-1] != filepath.Separator {
		absBase += string(filepath.Separator)
	}
	return len(absTarget) >= len(absBase) && absTarget[:len(absBase)] == absBase
}

func (fm *Manager) exists(path string) bool {
	_, err := os.Stat(path)
	return !os.IsNotExist(err)
}

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

	fullPath, err := fm.resolvePath(path)
	if err != nil {
		return nil, err
	}

	if _, err := os.Stat(fullPath); os.IsNotExist(err) {
		return nil, errors.ErrNotFound
	}

	content, err := os.ReadFile(fullPath)
	if err != nil {
		return nil, errors.WrapOp(errors.CodeFilesystem, "Read", "read file failed", err)
	}

	return content, nil
}

// Write writes content to a file using atomic write (write to temp file then rename).
func (fm *Manager) Write(ctx context.Context, path string, content []byte) error {
	fm.mu.Lock()
	defer fm.mu.Unlock()

	fullPath, err := fm.resolvePath(path)
	if err != nil {
		return err
	}

	dir := filepath.Dir(fullPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return errors.WrapOp(errors.CodeFilesystem, "Write", "create directory failed", err)
	}

	// Write to a temporary file first, then rename for atomicity
	tmpFile, err := os.CreateTemp(dir, ".tmp-")
	if err != nil {
		return errors.WrapOp(errors.CodeFilesystem, "Write", "create temp file failed", err)
	}
	tmpPath := tmpFile.Name()

	if _, err := tmpFile.Write(content); err != nil {
		tmpFile.Close()
		os.Remove(tmpPath)
		return errors.WrapOp(errors.CodeFilesystem, "Write", "write temp file failed", err)
	}

	if err := tmpFile.Close(); err != nil {
		os.Remove(tmpPath)
		return errors.WrapOp(errors.CodeFilesystem, "Write", "close temp file failed", err)
	}

	if err := os.Rename(tmpPath, fullPath); err != nil {
		os.Remove(tmpPath)
		return errors.WrapOp(errors.CodeFilesystem, "Write", "rename temp file failed", err)
	}

	return nil
}

// Append appends content to an existing file.
func (fm *Manager) Append(ctx context.Context, path string, content []byte) error {
	fm.mu.Lock()
	defer fm.mu.Unlock()

	fullPath, err := fm.resolvePath(path)
	if err != nil {
		return err
	}

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

	fullPath, err := fm.resolvePath(path)
	if err != nil {
		return err
	}

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

	fullPath, err := fm.resolvePath(path)
	if err != nil {
		return false, err
	}
	return fm.exists(fullPath), nil
}

// List lists files matching a pattern.
func (fm *Manager) List(ctx context.Context, pattern string) ([]string, error) {
	fm.mu.RLock()
	defer fm.mu.RUnlock()

	// Validate pattern stays within workspace
	fullPattern, err := fm.resolvePath(pattern)
	if err != nil {
		return nil, err
	}

	var files []string
	err = filepath.Walk(fm.config.WorkspaceDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}

		matched, matchErr := filepath.Match(fullPattern, path)
		if matchErr != nil {
			return matchErr
		}
		if matched {
			relPath, relErr := filepath.Rel(fm.config.WorkspaceDir, path)
			if relErr != nil {
				return relErr
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

// resolvePath resolves and validates a path to ensure it stays within the workspace.
// Returns an error if the path attempts to traverse outside the workspace.
func (fm *Manager) resolvePath(path string) (string, error) {
	// Clean the path to remove any .. or . elements
	cleaned := filepath.Clean(path)

	absWorkspace, err := filepath.Abs(fm.config.WorkspaceDir)
	if err != nil {
		return "", errors.WrapOp(errors.CodeFilesystem, "resolvePath", "resolve workspace path failed", err)
	}

	var resolved string
	if filepath.IsAbs(cleaned) {
		resolved = cleaned
	} else {
		resolved = filepath.Join(absWorkspace, cleaned)
	}

	// Evaluate symlinks on the workspace to get the real base
	evalWorkspace := evalSymlinksSafe(absWorkspace)

	// Evaluate symlinks on the resolved path; for non-existent paths,
	// walk up to find the deepest existing ancestor and evaluate from there
	evalResolved := evalSymlinksSafe(resolved)

	if !isSubPath(evalWorkspace, evalResolved) {
		return "", errors.New(errors.CodeFilesystem, "path traversal detected: path escapes workspace")
	}

	return resolved, nil
}

// evalSymlinksSafe evaluates symlinks on a path. If the path doesn't exist,
// it walks up to find the deepest existing ancestor, evaluates symlinks there,
// then appends the remaining non-existent components.
func evalSymlinksSafe(path string) string {
	eval, err := filepath.EvalSymlinks(path)
	if err == nil {
		return eval
	}
	// Path doesn't exist — walk up to find an existing ancestor
	dir := filepath.Dir(path)
	base := filepath.Base(path)
	for {
		evalDir, dirErr := filepath.EvalSymlinks(dir)
		if dirErr == nil {
			return filepath.Join(evalDir, base)
		}
		base = filepath.Join(filepath.Base(dir), base)
		dir = filepath.Dir(dir)
		if dir == "/" || dir == "." {
			// Reached root without finding existing path, return original
			return path
		}
	}
}

// isSubPath checks if target is within base directory (or equals base).
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
	// Target equals base is also valid
	if absTarget == absBase {
		return true
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

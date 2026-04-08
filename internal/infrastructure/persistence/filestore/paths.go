package filestore

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/xgsong/MyMemoryGo/internal/domain/entity"
)

// GetLongTermPath returns the path to the long-term memory file.
func (fm *Manager) GetLongTermPath() string {
	return fm.config.LongTermFile
}

// GetDailyPath returns the path to a daily log file for the given date.
func (fm *Manager) GetDailyPath(date time.Time) string {
	filename := date.Format("2006-01-02") + fm.config.FileExtension
	return filepath.Join(fm.config.DailyDir, filename)
}

// ParseDailyPath extracts the date from a daily log path.
func (fm *Manager) ParseDailyPath(path string) (time.Time, error) {
	// Normalize path
	path = filepath.ToSlash(path)

	// Extract filename
	filename := filepath.Base(path)

	// Remove extension
	name := strings.TrimSuffix(filename, fm.config.FileExtension)

	// Parse date
	date, err := time.Parse("2006-01-02", name)
	if err != nil {
		return time.Time{}, wrapErr("parse date from path", err)
	}

	return date, nil
}

// IsLongTerm checks if the path is for long-term memory.
func (fm *Manager) IsLongTerm(path string) bool {
	return filepath.Base(path) == fm.config.LongTermFile
}

// IsDaily checks if the path is a daily log.
func (fm *Manager) IsDaily(path string) bool {
	dir := filepath.Dir(path)
	if filepath.Base(dir) != fm.config.DailyDir {
		return false
	}

	filename := filepath.Base(path)
	if filepath.Ext(filename) != fm.config.FileExtension {
		return false
	}

	name := strings.TrimSuffix(filename, fm.config.FileExtension)
	_, err := time.Parse("2006-01-02", name)
	return err == nil
}

// DetermineSourceType determines the memory source type from the path.
func (fm *Manager) DetermineSourceType(path string) entity.SourceType {
	if fm.IsLongTerm(path) {
		return entity.SourceLongTerm
	}
	if fm.IsDaily(path) {
		return entity.SourceDaily
	}
	return entity.SourceSession
}

// ListLongTerm lists all long-term memory files.
func (fm *Manager) ListLongTerm(ctx context.Context) ([]string, error) {
	longTermPath := filepath.Join(fm.config.WorkspaceDir, fm.config.LongTermFile)

	if !fm.exists(longTermPath) {
		return []string{}, nil
	}

	return []string{fm.config.LongTermFile}, nil
}

// ListDaily lists all daily log files.
func (fm *Manager) ListDaily(ctx context.Context) ([]string, error) {
	dailyDir := filepath.Join(fm.config.WorkspaceDir, fm.config.DailyDir)

	var files []string
	err := filepath.Walk(dailyDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}

		// Get relative path
		relPath, err := filepath.Rel(fm.config.WorkspaceDir, path)
		if err != nil {
			return err
		}

		// Check if it's a daily file
		if fm.IsDaily(relPath) {
			files = append(files, relPath)
		}

		return nil
	})

	if err != nil {
		return nil, wrapErr("list daily files", err)
	}

	return files, nil
}

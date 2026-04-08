package service

import (
	"context"
	"fmt"
	"time"

	"github.com/xgsong/MyMemoryGo/internal/domain/entity"
	domainService "github.com/xgsong/MyMemoryGo/internal/domain/service"
	"github.com/xgsong/MyMemoryGo/internal/pkg/log"
)

// SyncIndex synchronizes the search index with the file system.
// It re-reads all markdown files and updates the index.
func (s *MemoryApplicationService) SyncIndex(ctx context.Context) error {
	logger := log.GetLogger(ctx)
	logger.InfoContext(ctx, "starting index synchronization")

	// List all markdown files (read-only, no lock needed)
	pattern := "memory/*.md"
	files, err := s.fileRepo.List(ctx, pattern)
	if err != nil {
		return fmt.Errorf("failed to list memory files: %w", err)
	}

	// Also check MEMORY.md
	longTermPath := "MEMORY.md"
	exists, err := s.fileRepo.Exists(ctx, longTermPath)
	if err != nil {
		return fmt.Errorf("failed to check MEMORY.md: %w", err)
	}
	if exists {
		files = append(files, longTermPath)
	}

	logger.InfoContext(ctx, "files to sync", "file_count", len(files))

	// Process each file (embedding generation happens without the lock)
	syncedCount := 0
	errorCount := 0
	for _, file := range files {
		if err := s.syncFile(ctx, file); err != nil {
			logger.WarnContext(ctx, "failed to sync file", "file", file, "error", err)
			errorCount++
		} else {
			syncedCount++
		}
	}

	logger.InfoContext(ctx, "index synchronization completed", "synced", syncedCount, "errors", errorCount)

	return nil
}

// syncFile processes a single file for index synchronization.
func (s *MemoryApplicationService) syncFile(ctx context.Context, path string) error {
	// Read file content
	content, err := s.fileRepo.Read(ctx, path)
	if err != nil {
		return err
	}

	// Determine source type
	source := domainService.DetermineSourceType(path)

	// Split content into chunks (simplified: treat entire content as one memory)
	text := string(content)
	if text == "" {
		return nil
	}

	// Calculate line numbers from content
	startLine := 1
	endLine := 1
	for _, ch := range text {
		if ch == '\n' {
			endLine++
		}
	}
	// Adjust: trailing newline should not count as an extra line
	if len(text) > 0 && text[len(text)-1] == '\n' && endLine > 1 {
		endLine--
	}

	// Create memory entity
	now := time.Now()
	memory := &entity.Memory{
		ID:        domainService.GenerateID(path, startLine, endLine),
		Path:      path,
		Content:   text,
		Source:    source,
		CreatedAt: now,
		UpdatedAt: now,
		StartLine: startLine,
		EndLine:   endLine,
		Checksum:  domainService.CalculateChecksum(text),
	}

	// Validate memory
	if err := domainService.ValidateMemory(memory); err != nil {
		return err
	}

	// Generate embedding before acquiring write lock
	embedding, err := s.embeddingRepo.Embed(ctx, text)
	if err != nil {
		return err
	}
	memory.Embedding = embedding

	// Acquire write lock only for the actual store operation
	s.writeMutex.Lock()
	defer s.writeMutex.Unlock()

	// Store the memory (file + database)
	return s.memoryRepo.Store(ctx, memory)
}

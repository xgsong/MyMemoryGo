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

	s.writeMutex.Lock()
	defer s.writeMutex.Unlock()

	// List all markdown files
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

	// Process each file
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

	// Create memory entity
	now := time.Now()
	memory := &entity.Memory{
		Path:      path,
		Content:   text,
		Source:    source,
		CreatedAt: now,
		UpdatedAt: now,
		Checksum:  domainService.CalculateChecksum(text),
	}

	// Validate memory
	if err := domainService.ValidateMemory(memory); err != nil {
		return err
	}

	// Generate embedding
	embedding, err := s.embeddingRepo.Embed(ctx, text)
	if err != nil {
		return err
	}
	memory.Embedding = embedding

	// Store the memory (file + database)
	return s.memoryRepo.Store(ctx, memory)
}

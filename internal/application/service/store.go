package service

import (
	"context"
	"fmt"
	"time"

	"github.com/xgsong/MyMemoryGo/internal/domain/entity"
	domainService "github.com/xgsong/MyMemoryGo/internal/domain/service"
	"github.com/xgsong/MyMemoryGo/internal/pkg/errors"
	"github.com/xgsong/MyMemoryGo/internal/pkg/log"
)

// StoreMemoryRequest contains parameters for storing a memory.
type StoreMemoryRequest struct {
	Content  string
	Path     string // Optional: if empty, auto-select based on source
	Source   entity.SourceType
	Metadata map[string]string
}

// StoreMemoryResponse contains the result of storing a memory.
type StoreMemoryResponse struct {
	Memory *entity.Memory
}

// StoreMemory stores a new memory with automatic embedding generation and indexing.
// It abstracts the complexity of validation, embedding, storage, and indexing.
func (s *MemoryApplicationService) StoreMemory(ctx context.Context, req *StoreMemoryRequest) (*StoreMemoryResponse, error) {
	logger := log.GetLogger(ctx).With("source", req.Source)

	logger.InfoContext(ctx, "storing memory", "content_length", len(req.Content), "path", req.Path)

	if req.Content == "" {
		return nil, errors.New(errors.CodeInvalidInput, "content cannot be empty")
	}

	// Determine path if not specified
	path := req.Path
	if path == "" {
		path = s.determineDefaultPath(req.Source)
	}

	// Normalize path
	path = domainService.NormalizePath(path)

	// Determine source type if not set
	source := req.Source
	if source == "" {
		source = domainService.DetermineSourceType(path)
	}

	// Create memory entity
	now := time.Now()

	// Calculate line numbers from content
	startLine := 1
	endLine := 1
	if req.Content != "" {
		// Count actual lines in content
		for _, ch := range req.Content {
			if ch == '\n' {
				endLine++
			}
		}
		// Adjust: trailing newline should not count as an extra line
		if req.Content[len(req.Content)-1] == '\n' && endLine > 1 {
			endLine--
		}
	}

	// Generate unique ID based on path and line numbers
	memoryID := domainService.GenerateID(path, startLine, endLine)

	memory := &entity.Memory{
		ID:        memoryID,
		Path:      path,
		Content:   req.Content,
		Source:    source,
		CreatedAt: now,
		UpdatedAt: now,
		StartLine: startLine,
		EndLine:   endLine,
		Checksum:  domainService.CalculateChecksum(req.Content),
		Metadata:  req.Metadata,
	}

	if err := domainService.ValidateMemory(memory); err != nil {
		logger.WarnContext(ctx, "memory validation failed", "error", err)
		return nil, errors.WrapOp(errors.CodeValidation, "StoreMemory", "memory validation failed", err)
	}

	embedding, err := s.embeddingRepo.Embed(ctx, memory.Content)
	if err != nil {
		logger.ErrorContext(ctx, "failed to generate embedding", "error", err)
		return nil, errors.WrapOp(errors.CodeNetwork, "StoreMemory", "failed to generate embedding", err)
	}
	memory.Embedding = embedding

	s.writeMutex.Lock()
	defer s.writeMutex.Unlock()

	if err := s.memoryRepo.Store(ctx, memory); err != nil {
		logger.ErrorContext(ctx, "failed to store memory", "error", err, "memory_id", memory.ID)
		return nil, errors.WrapOp(errors.CodeDatabase, "StoreMemory", "failed to store memory", err)
	}

	logger.InfoContext(ctx, "memory stored successfully", "memory_id", memory.ID, "path", memory.Path)

	return &StoreMemoryResponse{Memory: memory}, nil
}

// determineDefaultPath selects an appropriate path based on source type.
func (s *MemoryApplicationService) determineDefaultPath(source entity.SourceType) string {
	switch source {
	case entity.SourceLongTerm:
		return "MEMORY.md"
	case entity.SourceDaily:
		return fmt.Sprintf("memory/%s.md", time.Now().Format("2006-01-02"))
	case entity.SourceSession:
		return fmt.Sprintf("session/%s.md", time.Now().Format("20060102-150405"))
	default:
		return fmt.Sprintf("memory/%s.md", time.Now().Format("2006-01-02"))
	}
}

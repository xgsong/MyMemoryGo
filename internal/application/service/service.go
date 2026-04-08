// Package service provides application-level services that orchestrate
// domain logic and infrastructure components for use cases.
package service

import (
	"context"

	"github.com/xgsong/MyMemoryGo/internal/domain/repository"
)

// MemoryApplicationService orchestrates memory-related use cases.
// It coordinates between domain services, repositories, and infrastructure.
type MemoryApplicationService struct {
	memoryRepo    repository.MemoryRepository
	searchRepo    repository.SearchRepository
	embeddingRepo repository.EmbeddingRepository
	fileRepo      repository.FileRepository
}

// NewMemoryApplicationService creates a new application service.
func NewMemoryApplicationService(
	memoryRepo repository.MemoryRepository,
	searchRepo repository.SearchRepository,
	embeddingRepo repository.EmbeddingRepository,
	fileRepo repository.FileRepository,
) *MemoryApplicationService {
	return &MemoryApplicationService{
		memoryRepo:    memoryRepo,
		searchRepo:    searchRepo,
		embeddingRepo: embeddingRepo,
		fileRepo:      fileRepo,
	}
}

// Embed generates an embedding for the given text.
// This is exposed for health check purposes.
func (s *MemoryApplicationService) Embed(ctx context.Context, text string) ([]float32, error) {
	return s.embeddingRepo.Embed(ctx, text)
}

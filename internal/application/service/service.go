// Package service provides application-level services that orchestrate
// domain logic and infrastructure components for use cases.
package service

import (
	"sync"

	"github.com/xgsong/MyMemoryGo/internal/domain/repository"
)

// MemoryApplicationService orchestrates memory-related use cases.
// It coordinates between domain services, repositories, and infrastructure.
type MemoryApplicationService struct {
	memoryRepo    repository.MemoryRepository
	searchRepo    repository.SearchRepository
	embeddingRepo repository.EmbeddingRepository
	fileRepo      repository.FileRepository

	// writeMutex protects SQLite concurrent writes
	writeMutex sync.RWMutex
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

package service

import (
	"context"

	"github.com/xgsong/MyMemoryGo/internal/domain/entity"
	"github.com/xgsong/MyMemoryGo/internal/domain/repository"
	"github.com/xgsong/MyMemoryGo/internal/pkg/errors"
)

// ListMemoriesRequest contains parameters for listing memories.
type ListMemoriesRequest struct {
	Source         entity.SourceType
	PathPrefix     string
	Limit          int
	Offset         int
	OrderBy        string
	OrderDirection string
}

// ListMemoriesResponse contains list results.
type ListMemoriesResponse struct {
	Memories []*entity.Memory
	Total    int
}

// ListMemories retrieves memories with filtering and pagination.
func (s *MemoryApplicationService) ListMemories(ctx context.Context, req *ListMemoriesRequest) (*ListMemoriesResponse, error) {
	s.writeMutex.RLock()
	defer s.writeMutex.RUnlock()

	opts := &repository.ListOptions{
		Source:         req.Source,
		PathPrefix:     req.PathPrefix,
		Limit:          req.Limit,
		Offset:         req.Offset,
		OrderBy:        req.OrderBy,
		OrderDirection: req.OrderDirection,
	}

	memories, total, err := s.memoryRepo.List(ctx, opts)
	if err != nil {
		return nil, errors.WrapOp(errors.CodeDatabase, "ListMemories", "failed to list memories", err)
	}

	return &ListMemoriesResponse{
		Memories: memories,
		Total:    total,
	}, nil
}

// GetMemory retrieves a single memory by ID.
func (s *MemoryApplicationService) GetMemory(ctx context.Context, id string) (*entity.Memory, error) {
	s.writeMutex.RLock()
	defer s.writeMutex.RUnlock()

	memory, err := s.memoryRepo.Get(ctx, id)
	if err != nil {
		return nil, errors.WrapOp(errors.CodeDatabase, "GetMemory", "failed to get memory", err)
	}

	return memory, nil
}

// DeleteMemory removes a memory by ID from both storage and index.
func (s *MemoryApplicationService) DeleteMemory(ctx context.Context, id string) error {
	s.writeMutex.Lock()
	defer s.writeMutex.Unlock()

	if err := s.memoryRepo.Delete(ctx, id); err != nil {
		return errors.WrapOp(errors.CodeDatabase, "DeleteMemory", "failed to delete memory", err)
	}

	return nil
}

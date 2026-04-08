// Package service defines domain services that orchestrate business logic.
package service

import (
	"fmt"

	"github.com/xgsong/MyMemoryGo/internal/domain/entity"
	"github.com/xgsong/MyMemoryGo/internal/domain/errors"
)

// ValidateMemory validates a memory entity before persistence.
// This enforces business rules and data integrity constraints.
func ValidateMemory(memory *entity.Memory) error {
	if memory == nil {
		return errors.NewValidationError("memory", "cannot be nil")
	}

	if memory.Path == "" {
		return errors.NewValidationError("path", "cannot be empty")
	}

	if memory.Content == "" {
		return errors.NewValidationError("content", "cannot be empty")
	}

	if memory.StartLine < 1 {
		return errors.NewValidationError("start_line", "must be >= 1")
	}

	if memory.EndLine < memory.StartLine {
		return errors.NewValidationError("end_line", "must be >= start_line")
	}

	if memory.Source == "" {
		return errors.NewValidationError("source", "cannot be empty")
	}

	if !IsValidSource(memory.Source) {
		return errors.NewValidationError("source", fmt.Sprintf("invalid source type: %s", memory.Source))
	}

	return nil
}

// IsValidSource checks if the source type is valid.
func IsValidSource(source entity.SourceType) bool {
	switch source {
	case entity.SourceLongTerm, entity.SourceDaily, entity.SourceSession:
		return true
	default:
		return false
	}
}

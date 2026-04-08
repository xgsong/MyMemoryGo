// Package service defines domain services that orchestrate business logic.
package service

import (
	"fmt"
	"strings"
	"time"

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

	// Validate ID if provided
	if memory.ID != "" {
		expectedID := GenerateID(memory.Path, memory.StartLine, memory.EndLine)
		if memory.ID != expectedID {
			return errors.NewValidationError("id", fmt.Sprintf("invalid ID format: expected '%s', got '%s'", expectedID, memory.ID))
		}
	}

	// Validate Checksum if provided
	if memory.Checksum != "" {
		expectedChecksum := CalculateChecksum(memory.Content)
		if memory.Checksum != expectedChecksum {
			return errors.NewValidationError("checksum", "content checksum mismatch")
		}
	}

	// Validate timestamps if provided
	if !memory.CreatedAt.IsZero() && !memory.UpdatedAt.IsZero() {
		if memory.UpdatedAt.Before(memory.CreatedAt) {
			return errors.NewValidationError("updated_at", "cannot be before created_at")
		}
	}

	// Validate Source and Path consistency (if path follows standard patterns)
	if memory.Source == entity.SourceDaily {
		if strings.HasPrefix(memory.Path, "memory/") && strings.HasSuffix(memory.Path, ".md") {
			datePart := strings.TrimSuffix(strings.TrimPrefix(memory.Path, "memory/"), ".md")
			if _, err := time.Parse("2006-01-02", datePart); err != nil {
				return errors.NewValidationError("path", fmt.Sprintf("daily memory path should contain valid date, got '%s'", datePart))
			}
		}
	} else if memory.Source == entity.SourceSession {
		if strings.HasPrefix(memory.Path, "session/") && strings.HasSuffix(memory.Path, ".md") {
			// Session path format is optional, no strict validation
		}
	} else if memory.Source == entity.SourceLongTerm {
		// Long-term memory can be stored in any path, no strict validation
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

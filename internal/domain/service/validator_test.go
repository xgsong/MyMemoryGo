package service

import (
	"testing"

	"github.com/xgsong/MyMemoryGo/internal/domain/entity"
	"github.com/xgsong/MyMemoryGo/internal/domain/errors"
)

func TestValidateMemory(t *testing.T) {
	tests := []struct {
		name    string
		memory  *entity.Memory
		wantErr bool
		errMsg  string
	}{
		{
			name:    "nil memory",
			memory:  nil,
			wantErr: true,
			errMsg:  "cannot be nil",
		},
		{
			name: "valid memory",
			memory: &entity.Memory{
				Path:      "test.md",
				Content:   "test content",
				StartLine: 1,
				EndLine:   5,
				Source:    entity.SourceLongTerm,
			},
			wantErr: false,
		},
		{
			name: "empty path",
			memory: &entity.Memory{
				Path:      "",
				Content:   "test content",
				StartLine: 1,
				EndLine:   5,
				Source:    entity.SourceLongTerm,
			},
			wantErr: true,
			errMsg:  "cannot be empty",
		},
		{
			name: "empty content",
			memory: &entity.Memory{
				Path:      "test.md",
				Content:   "",
				StartLine: 1,
				EndLine:   5,
				Source:    entity.SourceLongTerm,
			},
			wantErr: true,
			errMsg:  "cannot be empty",
		},
		{
			name: "start line less than 1",
			memory: &entity.Memory{
				Path:      "test.md",
				Content:   "test content",
				StartLine: 0,
				EndLine:   5,
				Source:    entity.SourceLongTerm,
			},
			wantErr: true,
			errMsg:  "must be >= 1",
		},
		{
			name: "negative start line",
			memory: &entity.Memory{
				Path:      "test.md",
				Content:   "test content",
				StartLine: -1,
				EndLine:   5,
				Source:    entity.SourceLongTerm,
			},
			wantErr: true,
			errMsg:  "must be >= 1",
		},
		{
			name: "end line less than start line",
			memory: &entity.Memory{
				Path:      "test.md",
				Content:   "test content",
				StartLine: 10,
				EndLine:   5,
				Source:    entity.SourceLongTerm,
			},
			wantErr: true,
			errMsg:  "must be >= start_line",
		},
		{
			name: "empty source",
			memory: &entity.Memory{
				Path:      "test.md",
				Content:   "test content",
				StartLine: 1,
				EndLine:   5,
				Source:    "",
			},
			wantErr: true,
			errMsg:  "cannot be empty",
		},
		{
			name: "invalid source type",
			memory: &entity.Memory{
				Path:      "test.md",
				Content:   "test content",
				StartLine: 1,
				EndLine:   5,
				Source:    "invalid",
			},
			wantErr: true,
			errMsg:  "invalid source type",
		},
		{
			name: "valid daily memory",
			memory: &entity.Memory{
				Path:      "memory/2024-01-01.md",
				Content:   "test content",
				StartLine: 1,
				EndLine:   5,
				Source:    entity.SourceDaily,
			},
			wantErr: false,
		},
		{
			name: "valid session memory",
			memory: &entity.Memory{
				Path:      "session.md",
				Content:   "test content",
				StartLine: 1,
				EndLine:   5,
				Source:    entity.SourceSession,
			},
			wantErr: false,
		},
		{
			name: "start line equals end line",
			memory: &entity.Memory{
				Path:      "test.md",
				Content:   "test content",
				StartLine: 5,
				EndLine:   5,
				Source:    entity.SourceLongTerm,
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateMemory(tt.memory)

			if tt.wantErr {
				if err == nil {
					t.Errorf("ValidateMemory() expected error but got nil")
					return
				}

				if tt.errMsg != "" {
					var validationErr *errors.ValidationError
					if _, ok := err.(*errors.ValidationError); !ok {
						t.Errorf("ValidateMemory() expected ValidationError but got %T", err)
						return
					}
					validationErr = err.(*errors.ValidationError)

					// Check if error message contains expected substring
					if !containsString(validationErr.Message, tt.errMsg) {
						t.Errorf("ValidateMemory() error message = %v, want to contain %v", validationErr.Message, tt.errMsg)
					}
				}
			} else {
				if err != nil {
					t.Errorf("ValidateMemory() unexpected error = %v", err)
				}
			}
		})
	}
}

func TestIsValidSource(t *testing.T) {
	tests := []struct {
		name   string
		source entity.SourceType
		want   bool
	}{
		{
			name:   "valid longterm source",
			source: entity.SourceLongTerm,
			want:   true,
		},
		{
			name:   "valid daily source",
			source: entity.SourceDaily,
			want:   true,
		},
		{
			name:   "valid session source",
			source: entity.SourceSession,
			want:   true,
		},
		{
			name:   "invalid source",
			source: "invalid",
			want:   false,
		},
		{
			name:   "empty source",
			source: "",
			want:   false,
		},
		{
			name:   "numeric source",
			source: "123",
			want:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsValidSource(tt.source)
			if got != tt.want {
				t.Errorf("IsValidSource() = %v, want %v", got, tt.want)
			}
		})
	}
}

// Helper function to check if a string contains a substring
func containsString(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || contains(s, substr))
}

func contains(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

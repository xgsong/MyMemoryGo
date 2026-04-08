package service

import (
	"testing"

	"github.com/xgsong/MyMemoryGo/internal/domain/entity"
)

func TestNormalizePath(t *testing.T) {
	tests := []struct {
		name string
		path string
		want string
	}{
		{
			name: "simple path",
			path: "test.md",
			want: "test.md",
		},
		{
			name: "path with leading slash",
			path: "/test.md",
			want: "test.md",
		},
		{
			name: "path with trailing slash",
			path: "test.md/",
			want: "test.md",
		},
		{
			name: "path with both slashes",
			path: "/test.md/",
			want: "test.md",
		},
		{
			name: "path with backslashes",
			path: "dir\\test.md",
			want: "dir/test.md",
		},
		{
			name: "windows path",
			path: "C:\\Users\\test.md",
			want: "C:/Users/test.md",
		},
		{
			name: "path with spaces",
			path: "  test.md  ",
			want: "test.md",
		},
		{
			name: "complex path with slashes and backslashes",
			path: "/a\\b/c/d\\e/",
			want: "a/b/c/d/e",
		},
		{
			name: "path with directory",
			path: "dir/subdir/file.md",
			want: "dir/subdir/file.md",
		},
		{
			name: "path with leading/trailing spaces and slashes",
			path: "  /test.md/  ",
			want: "test.md",
		},
		{
			name: "nested path with backslashes",
			path: "a\\b\\c\\file.md",
			want: "a/b/c/file.md",
		},
		{
			name: "empty string",
			path: "",
			want: "",
		},
		{
			name: "only slashes",
			path: "///",
			want: "/",
		},
		{
			name: "path with multiple slashes",
			path: "//test.md//",
			want: "/test.md/",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := NormalizePath(tt.path)
			if got != tt.want {
				t.Errorf("NormalizePath() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestDetermineSourceType(t *testing.T) {
	tests := []struct {
		name string
		path string
		want entity.SourceType
	}{
		{
			name: "MEMORY.md long term",
			path: "MEMORY.md",
			want: entity.SourceLongTerm,
		},
		{
			name: "memory.md long term",
			path: "memory.md",
			want: entity.SourceLongTerm,
		},
		{
			name: "daily memory valid date",
			path: "memory/2024-01-01.md",
			want: entity.SourceDaily,
		},
		{
			name: "daily memory another date",
			path: "memory/2023-12-31.md",
			want: entity.SourceDaily,
		},
		{
			name: "daily memory future date",
			path: "memory/2025-06-15.md",
			want: entity.SourceDaily,
		},
		{
			name: "session memory - regular file",
			path: "session.md",
			want: entity.SourceSession,
		},
		{
			name: "session memory - other path",
			path: "notes.md",
			want: entity.SourceSession,
		},
		{
			name: "session memory - subdirectory",
			path: "subdir/file.md",
			want: entity.SourceSession,
		},
		{
			name: "daily memory with slashes",
			path: "/memory/2024-01-01.md",
			want: entity.SourceDaily,
		},
		{
			name: "long term with slashes",
			path: "/MEMORY.md",
			want: entity.SourceLongTerm,
		},
		{
			name: "invalid daily format - wrong date format",
			path: "memory/2024/01/01.md",
			want: entity.SourceSession,
		},
		{
			name: "invalid daily format - not date",
			path: "memory/january.md",
			want: entity.SourceSession,
		},
		{
			name: "invalid daily format - missing dashes",
			path: "memory/20240101.md",
			want: entity.SourceSession,
		},
		{
			name: "invalid daily format - wrong dash positions",
			path: "memory/2024-01.01.md",
			want: entity.SourceSession,
		},
		{
			name: "invalid daily format - short length",
			path: "memory/2024.md",
			want: entity.SourceSession,
		},
		{
			name: "invalid daily format - missing extension",
			path: "memory/2024-01-01",
			want: entity.SourceSession,
		},
		{
			name: "daily memory with trailing slash",
			path: "memory/2024-01-01.md/",
			want: entity.SourceDaily,
		},
		{
			name: "MEMORY.md with trailing slash",
			path: "MEMORY.md/",
			want: entity.SourceLongTerm,
		},
		{
			name: "empty string",
			path: "",
			want: entity.SourceSession,
		},
		{
			name: "only slashes",
			path: "///",
			want: entity.SourceSession,
		},
		{
			name: "path with backslashes",
			path: "memory\\2024-01-01.md",
			want: entity.SourceDaily,
		},
		{
			name: "windows path with backslashes",
			path: "C:\\Users\\test.md",
			want: entity.SourceSession,
		},
		{
			name: "daily memory edge case - 0000-00-00",
			path: "memory/0000-00-00.md",
			want: entity.SourceDaily,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := DetermineSourceType(tt.path)
			if got != tt.want {
				t.Errorf("DetermineSourceType() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestDetermineSourceType_DateFormats(t *testing.T) {
	tests := []struct {
		name      string
		path      string
		wantValid bool
	}{
		{
			name:      "valid date 2024-01-01",
			path:      "memory/2024-01-01.md",
			wantValid: true,
		},
		{
			name:      "valid date 2023-12-31",
			path:      "memory/2023-12-31.md",
			wantValid: true,
		},
		{
			name:      "valid date 2025-06-15",
			path:      "memory/2025-06-15.md",
			wantValid: true,
		},
		{
			name:      "valid date 2000-01-01",
			path:      "memory/2000-01-01.md",
			wantValid: true,
		},
		{
			name:      "valid date 1999-12-31",
			path:      "memory/1999-12-31.md",
			wantValid: true,
		},
		{
			name:      "valid date 2099-12-31",
			path:      "memory/2099-12-31.md",
			wantValid: true,
		},
		{
			name:      "invalid month 13",
			path:      "memory/2024-13-01.md",
			wantValid: true,
		},
		{
			name:      "invalid month 00",
			path:      "memory/2024-00-01.md",
			wantValid: true,
		},
		{
			name:      "invalid day 32",
			path:      "memory/2024-01-32.md",
			wantValid: true,
		},
		{
			name:      "invalid day 00",
			path:      "memory/2024-01-00.md",
			wantValid: true,
		},
		{
			name:      "invalid year",
			path:      "memory/24-01-01.md",
			wantValid: false,
		},
		{
			name:      "invalid separator",
			path:      "memory/2024/01/01.md",
			wantValid: false,
		},
		{
			name:      "invalid format",
			path:      "memory/01-01-2024.md",
			wantValid: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := DetermineSourceType(tt.path)
			isDaily := (got == entity.SourceDaily)

			if isDaily != tt.wantValid {
				t.Errorf("DetermineSourceType() daily detection = %v, want %v", isDaily, tt.wantValid)
			}
		})
	}
}

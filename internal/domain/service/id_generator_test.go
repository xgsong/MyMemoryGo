package service

import (
	"testing"
)

func TestGenerateID(t *testing.T) {
	tests := []struct {
		name      string
		path      string
		startLine int
		endLine   int
		want      string
	}{
		{
			name:      "simple path",
			path:      "test.md",
			startLine: 1,
			endLine:   5,
			want:      "test.md:1-5",
		},
		{
			name:      "path with directory",
			path:      "dir/test.md",
			startLine: 10,
			endLine:   20,
			want:      "dir/test.md:10-20",
		},
		{
			name:      "path with leading slash",
			path:      "/test.md",
			startLine: 1,
			endLine:   1,
			want:      "test.md:1-1",
		},
		{
			name:      "path with trailing slash",
			path:      "test.md/",
			startLine: 1,
			endLine:   10,
			want:      "test.md:1-10",
		},
		{
			name:      "path with backslashes",
			path:      "dir\\test.md",
			startLine: 5,
			endLine:   15,
			want:      "dir/test.md:5-15",
		},
		{
			name:      "path with spaces",
			path:      "  test.md  ",
			startLine: 1,
			endLine:   5,
			want:      "test.md:1-5",
		},
		{
			name:      "complex path",
			path:      "/path/to/file.md/",
			startLine: 100,
			endLine:   200,
			want:      "path/to/file.md:100-200",
		},
		{
			name:      "single line",
			path:      "single.md",
			startLine: 42,
			endLine:   42,
			want:      "single.md:42-42",
		},
		{
			name:      "large line numbers",
			path:      "large.md",
			startLine: 1000,
			endLine:   2000,
			want:      "large.md:1000-2000",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := GenerateID(tt.path, tt.startLine, tt.endLine)
			if got != tt.want {
				t.Errorf("GenerateID() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestParseID(t *testing.T) {
	tests := []struct {
		name      string
		id        string
		wantPath  string
		wantStart int
		wantEnd   int
		wantErr   bool
		errMsg    string
	}{
		{
			name:      "valid ID simple",
			id:        "test.md:1-5",
			wantPath:  "test.md",
			wantStart: 1,
			wantEnd:   5,
			wantErr:   false,
		},
		{
			name:      "valid ID with directory",
			id:        "dir/test.md:10-20",
			wantPath:  "dir/test.md",
			wantStart: 10,
			wantEnd:   20,
			wantErr:   false,
		},
		{
			name:      "valid ID single line",
			id:        "single.md:42-42",
			wantPath:  "single.md",
			wantStart: 42,
			wantEnd:   42,
			wantErr:   false,
		},
		{
			name:      "valid ID large numbers",
			id:        "large.md:1000-2000",
			wantPath:  "large.md",
			wantStart: 1000,
			wantEnd:   2000,
			wantErr:   false,
		},
		{
			name:      "valid ID nested path",
			id:        "a/b/c/d/e.md:1-10",
			wantPath:  "a/b/c/d/e.md",
			wantStart: 1,
			wantEnd:   10,
			wantErr:   false,
		},
		{
			name:    "missing colon",
			id:      "test.md",
			wantErr: true,
			errMsg:  "invalid ID format",
		},
		{
			name:    "multiple colons",
			id:      "test.md:1:5",
			wantErr: true,
			errMsg:  "invalid ID format",
		},
		{
			name:    "missing dash in line range",
			id:      "test.md:15",
			wantErr: true,
			errMsg:  "invalid line range",
		},
		{
			name:    "multiple dashes in line range",
			id:      "test.md:1-2-3",
			wantErr: true,
			errMsg:  "invalid line range",
		},
		{
			name:    "invalid start line (non-numeric)",
			id:      "test.md:abc-5",
			wantErr: true,
			errMsg:  "invalid start line",
		},
		{
			name:    "invalid end line (non-numeric)",
			id:      "test.md:1-def",
			wantErr: true,
			errMsg:  "invalid end line",
		},
		{
			name:    "empty ID",
			id:      "",
			wantErr: true,
			errMsg:  "invalid ID format",
		},
		{
			name:      "empty path part",
			id:        ":1-5",
			wantPath:  "",
			wantStart: 1,
			wantEnd:   5,
			wantErr:   false,
		},
		{
			name:    "negative start line",
			id:      "test.md:-1-5",
			wantErr: true,
			errMsg:  "invalid line range",
		},
		{
			name:      "zero start line",
			id:        "test.md:0-5",
			wantPath:  "test.md",
			wantStart: 0,
			wantEnd:   5,
			wantErr:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path, startLine, endLine, err := ParseID(tt.id)

			if tt.wantErr {
				if err == nil {
					t.Errorf("ParseID() expected error but got nil")
					return
				}

				if tt.errMsg != "" && !containsString(err.Error(), tt.errMsg) {
					t.Errorf("ParseID() error message = %v, want to contain %v", err.Error(), tt.errMsg)
				}
			} else {
				if err != nil {
					t.Errorf("ParseID() unexpected error = %v", err)
					return
				}

				if path != tt.wantPath {
					t.Errorf("ParseID() path = %v, want %v", path, tt.wantPath)
				}

				if startLine != tt.wantStart {
					t.Errorf("ParseID() startLine = %v, want %v", startLine, tt.wantStart)
				}

				if endLine != tt.wantEnd {
					t.Errorf("ParseID() endLine = %v, want %v", endLine, tt.wantEnd)
				}
			}
		})
	}
}

func TestGenerateID_ParseID_RoundTrip(t *testing.T) {
	tests := []struct {
		name      string
		path      string
		startLine int
		endLine   int
	}{
		{
			name:      "simple case",
			path:      "test.md",
			startLine: 1,
			endLine:   5,
		},
		{
			name:      "complex path",
			path:      "/a/b/c/test.md/",
			startLine: 10,
			endLine:   20,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			id := GenerateID(tt.path, tt.startLine, tt.endLine)
			path, startLine, endLine, err := ParseID(id)

			if err != nil {
				t.Errorf("ParseID() error during round trip: %v", err)
				return
			}

			expectedPath := NormalizePath(tt.path)
			if path != expectedPath {
				t.Errorf("Round trip path = %v, want %v", path, expectedPath)
			}

			if startLine != tt.startLine {
				t.Errorf("Round trip startLine = %v, want %v", startLine, tt.startLine)
			}

			if endLine != tt.endLine {
				t.Errorf("Round trip endLine = %v, want %v", endLine, tt.endLine)
			}
		})
	}
}

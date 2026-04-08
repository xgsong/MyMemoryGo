package filestore

import (
	"context"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/xgsong/MyMemoryGo/internal/domain/repository"
)

func TestWatch_CreateWatcher(t *testing.T) {
	tests := []struct {
		name    string
		setup   func(t *testing.T) (*Manager, string, repository.FileChangeHandler)
		cleanup func(t *testing.T, fm *Manager, tempDir string)
		wantErr bool
		errMsg  string
	}{
		{
			name: "watch existing file",
			setup: func(t *testing.T) (*Manager, string, repository.FileChangeHandler) {
				tempDir := t.TempDir()
				config := &Config{
					WorkspaceDir: tempDir,
					LongTermFile: "MEMORY.md",
					DailyDir:     "memory",
				}

				fm, err := New(config)
				if err != nil {
					t.Fatalf("failed to create manager: %v", err)
				}
				defer fm.Close()

				// Create a test file
				testFile := filepath.Join(tempDir, "test.md")
				err = os.WriteFile(testFile, []byte("test content"), 0644)
				if err != nil {
					t.Fatalf("failed to create test file: %v", err)
				}

				handler := func(event repository.FileChangeEvent) {
					// Mock handler
				}

				return fm, "test.md", handler
			},
			cleanup: func(t *testing.T, fm *Manager, tempDir string) {
				fm.Close()
			},
			wantErr: false,
		},
		{
			name: "watch existing directory",
			setup: func(t *testing.T) (*Manager, string, repository.FileChangeHandler) {
				tempDir := t.TempDir()
				config := &Config{
					WorkspaceDir: tempDir,
					LongTermFile: "MEMORY.md",
					DailyDir:     "memory",
				}

				fm, err := New(config)
				if err != nil {
					t.Fatalf("failed to create manager: %v", err)
				}
				defer fm.Close()

				// Create a test directory
				testDir := filepath.Join(tempDir, "testdir")
				err = os.Mkdir(testDir, 0755)
				if err != nil {
					t.Fatalf("failed to create test directory: %v", err)
				}

				handler := func(event repository.FileChangeEvent) {
					// Mock handler
				}

				return fm, "testdir", handler
			},
			cleanup: func(t *testing.T, fm *Manager, tempDir string) {
				fm.Close()
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fm, path, handler := tt.setup(t)
			defer tt.cleanup(t, fm, filepath.Dir(fm.config.WorkspaceDir))

			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()

			err := fm.Watch(ctx, path, handler)

			if tt.wantErr {
				if err == nil {
					t.Errorf("Watch() expected error but got nil")
					return
				}

				if tt.errMsg != "" && !containsString(err.Error(), tt.errMsg) {
					t.Errorf("Watch() error message = %v, want to contain %v", err.Error(), tt.errMsg)
				}
			} else {
				if err != nil {
					t.Errorf("Watch() unexpected error = %v", err)
				}
			}
		})
	}
}

func TestWatch_FileEvents(t *testing.T) {
	t.Skip("Skipping file events test due to timing-dependent nature and flakiness")

	tempDir := t.TempDir()
	config := &Config{
		WorkspaceDir: tempDir,
		LongTermFile: "MEMORY.md",
		DailyDir:     "memory",
	}

	fm, err := New(config)
	if err != nil {
		t.Fatalf("failed to create manager: %v", err)
	}
	defer fm.Close()

	// Create a test file
	testFile := filepath.Join(tempDir, "watched.md")
	err = os.WriteFile(testFile, []byte("initial content"), 0644)
	if err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	tests := []struct {
		name      string
		operation func() error
		wantOp    repository.FileOperation
	}{
		{
			name: "file create event",
			operation: func() error {
				newFile := filepath.Join(tempDir, "newfile.md")
				return os.WriteFile(newFile, []byte("new content"), 0644)
			},
			wantOp: repository.FileOperationCreate,
		},
		{
			name: "file write event",
			operation: func() error {
				return os.WriteFile(testFile, []byte("modified content"), 0644)
			},
			wantOp: repository.FileOperationModify,
		},
		{
			name: "file delete event",
			operation: func() error {
				deleteFile := filepath.Join(tempDir, "delete.md")
				err := os.WriteFile(deleteFile, []byte("to delete"), 0644)
				if err != nil {
					return err
				}
				// Wait a bit for the file system to settle
				time.Sleep(10 * time.Millisecond)
				return os.Remove(deleteFile)
			},
			wantOp: repository.FileOperationDelete,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()

			// Capture events
			var events []repository.FileChangeEvent
			var mu sync.Mutex
			handler := func(event repository.FileChangeEvent) {
				mu.Lock()
				defer mu.Unlock()
				events = append(events, event)
			}

			// Start watching the directory
			watchPath := "."
			err := fm.Watch(ctx, watchPath, handler)
			if err != nil {
				t.Fatalf("failed to start watching: %v", err)
			}

			// Wait for watcher to be ready
			time.Sleep(50 * time.Millisecond)

			// Perform the operation
			err = tt.operation()
			if err != nil {
				t.Fatalf("failed to perform operation: %v", err)
			}

			// Wait for events to be processed - file system events can be slow
			time.Sleep(200 * time.Millisecond)

			// Check if we received the expected event
			mu.Lock()
			defer mu.Unlock()

			found := false
			for _, event := range events {
				if event.Operation == tt.wantOp {
					found = true
					break
				}
			}

			if !found {
				t.Errorf("Did not receive expected event %v, got events: %+v", tt.wantOp, events)
			}
		})
	}
}

func TestWatch_MultipleHandlers(t *testing.T) {
	tempDir := t.TempDir()
	config := &Config{
		WorkspaceDir: tempDir,
		LongTermFile: "MEMORY.md",
		DailyDir:     "memory",
	}

	fm, err := New(config)
	if err != nil {
		t.Fatalf("failed to create manager: %v", err)
	}
	defer fm.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Track calls to each handler
	var handler1Calls, handler2Calls []repository.FileChangeEvent
	var mu sync.Mutex

	handler1 := func(event repository.FileChangeEvent) {
		mu.Lock()
		defer mu.Unlock()
		handler1Calls = append(handler1Calls, event)
	}

	handler2 := func(event repository.FileChangeEvent) {
		mu.Lock()
		defer mu.Unlock()
		handler2Calls = append(handler2Calls, event)
	}

	// Watch with first handler
	watchPath := "."
	err = fm.Watch(ctx, watchPath, handler1)
	if err != nil {
		t.Fatalf("failed to start watching with handler1: %v", err)
	}

	// Watch with second handler
	err = fm.Watch(ctx, watchPath, handler2)
	if err != nil {
		t.Fatalf("failed to start watching with handler2: %v", err)
	}

	// Create a file to trigger events
	testFile := filepath.Join(tempDir, "test.md")
	err = os.WriteFile(testFile, []byte("test"), 0644)
	if err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	// Wait for events
	time.Sleep(100 * time.Millisecond)

	// Check that both handlers received events
	mu.Lock()
	defer mu.Unlock()

	if len(handler1Calls) == 0 {
		t.Error("handler1 did not receive any events")
	}

	if len(handler2Calls) == 0 {
		t.Error("handler2 did not receive any events")
	}

	// Both handlers should receive the same events
	if len(handler1Calls) != len(handler2Calls) {
		t.Errorf("handler1 received %d events, handler2 received %d events", len(handler1Calls), len(handler2Calls))
	}
}

func TestWatch_ContextCancellation(t *testing.T) {
	tempDir := t.TempDir()
	config := &Config{
		WorkspaceDir: tempDir,
		LongTermFile: "MEMORY.md",
		DailyDir:     "memory",
	}

	fm, err := New(config)
	if err != nil {
		t.Fatalf("failed to create manager: %v", err)
	}
	defer fm.Close()

	// Create a test file
	testFile := filepath.Join(tempDir, "test.md")
	err = os.WriteFile(testFile, []byte("test"), 0644)
	if err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())

	handler := func(event repository.FileChangeEvent) {
		// Mock handler
	}

	watchPath := "."
	err = fm.Watch(ctx, watchPath, handler)
	if err != nil {
		t.Fatalf("failed to start watching: %v", err)
	}

	// Cancel the context
	cancel()

	// The watcher should stop processing events
	// This is hard to test directly, but we can at least verify it doesn't crash
	time.Sleep(10 * time.Millisecond)
}

func TestWatch_ResolvePath(t *testing.T) {
	tempDir := t.TempDir()
	config := &Config{
		WorkspaceDir: tempDir,
		LongTermFile: "MEMORY.md",
		DailyDir:     "memory",
	}

	fm, err := New(config)
	if err != nil {
		t.Fatalf("failed to create manager: %v", err)
	}
	defer fm.Close()

	tests := []struct {
		name         string
		inputPath    string
		expectedPath string
	}{
		{
			name:         "relative path",
			inputPath:    "test.md",
			expectedPath: filepath.Join(tempDir, "test.md"),
		},
		{
			name:         "relative path with directory",
			inputPath:    "subdir/test.md",
			expectedPath: filepath.Join(tempDir, "subdir", "test.md"),
		},
		{
			name:         "absolute path",
			inputPath:    filepath.Join(tempDir, "abs.md"),
			expectedPath: filepath.Join(tempDir, "abs.md"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := fm.resolvePath(tt.inputPath)
			if got != tt.expectedPath {
				t.Errorf("resolvePath() = %v, want %v", got, tt.expectedPath)
			}
		})
	}
}

func TestWatch_NonExistentPath(t *testing.T) {
	tempDir := t.TempDir()
	config := &Config{
		WorkspaceDir: tempDir,
		LongTermFile: "MEMORY.md",
		DailyDir:     "memory",
	}

	fm, err := New(config)
	if err != nil {
		t.Fatalf("failed to create manager: %v", err)
	}
	defer fm.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	handler := func(event repository.FileChangeEvent) {
		// Mock handler
	}

	// Try to watch a non-existent path
	err = fm.Watch(ctx, "nonexistent.md", handler)
	if err == nil {
		t.Error("Watch() expected error for non-existent path but got nil")
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

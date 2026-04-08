package filestore

import (
	"context"
	"fmt"
	"path/filepath"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/xgsong/MyMemoryGo/internal/domain/repository"
)

// Watch starts watching a file or directory for changes.
func (fm *Manager) Watch(ctx context.Context, path string, handler repository.FileChangeHandler) error {
	fm.mu.Lock()
	defer fm.mu.Unlock()

	// Add handler
	fm.handlers = append(fm.handlers, handler)

	// Create watcher if not exists
	if fm.watcher == nil {
		watcher, err := fsnotify.NewWatcher()
		if err != nil {
			return fmt.Errorf("create watcher: %w", err)
		}
		fm.watcher = watcher

		// Create done channel for lifecycle management
		fm.done = make(chan struct{})

		// Start event processor
		go fm.processEvents(ctx)
	}

	// Resolve path
	fullPath, err := fm.resolvePath(path)
	if err != nil {
		return err
	}

	// Add path to watcher
	if err := fm.watcher.Add(fullPath); err != nil {
		return fmt.Errorf("add watch path: %w", err)
	}

	return nil
}

// RemoveHandler removes a previously registered file change handler.
func (fm *Manager) RemoveHandler(handler repository.FileChangeHandler) {
	fm.mu.Lock()
	defer fm.mu.Unlock()

	for i, h := range fm.handlers {
		if &h == &handler {
			fm.handlers = append(fm.handlers[:i], fm.handlers[i+1:]...)
			break
		}
	}
}

// processEvents processes file system events.
// It signals its exit via the done channel for lifecycle management.
func (fm *Manager) processEvents(ctx context.Context) {
	defer close(fm.done)

	for {
		select {
		case <-ctx.Done():
			return

		case event, ok := <-fm.watcher.Events:
			if !ok {
				return
			}
			fm.handleFileEvent(event)

		case err, ok := <-fm.watcher.Errors:
			if !ok {
				return
			}
			// Log error but continue watching
			fmt.Printf("watcher error: %v\n", err)
		}
	}
}

// handleFileEvent handles a file system event.
func (fm *Manager) handleFileEvent(event fsnotify.Event) {
	// Determine operation
	var operation repository.FileOperation
	switch {
	case event.Op&fsnotify.Create == fsnotify.Create:
		operation = repository.FileOperationCreate
	case event.Op&fsnotify.Write == fsnotify.Write:
		operation = repository.FileOperationModify
	case event.Op&fsnotify.Remove == fsnotify.Remove:
		operation = repository.FileOperationDelete
	default:
		return
	}

	// Get relative path
	relPath, err := filepath.Rel(fm.config.WorkspaceDir, event.Name)
	if err != nil {
		return
	}

	// Create change event
	changeEvent := repository.FileChangeEvent{
		Path:      relPath,
		Operation: operation,
		Timestamp: time.Now(),
	}

	// Notify all handlers
	fm.mu.RLock()
	handlers := make([]repository.FileChangeHandler, len(fm.handlers))
	copy(handlers, fm.handlers)
	fm.mu.RUnlock()

	for _, handler := range handlers {
		handler(changeEvent)
	}
}

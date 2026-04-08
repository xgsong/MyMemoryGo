package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/xgsong/MyMemoryGo/internal/domain/entity"
	"github.com/xgsong/MyMemoryGo/internal/infrastructure/persistence/filestore"
	"github.com/xgsong/MyMemoryGo/internal/infrastructure/persistence/sqlite"
)

// Integration test demonstrating the complete workflow
func main() {
	fmt.Println("Running Integration Tests...")
	fmt.Println("==========================")

	// Setup
	tmpDir, err := os.MkdirTemp("", "memory-test-*")
	if err != nil {
		fmt.Printf("FAIL: %v\n", err)
		os.Exit(1)
	}
	defer os.RemoveAll(tmpDir)

	// Test 1: SQLite Storage
	fmt.Println("\nTest 1: SQLite Storage")
	store, err := sqlite.New(&sqlite.Config{
		DBPath:           fmt.Sprintf("%s/test.db", tmpDir),
		WALMode:          true,
		VectorDimensions: 768,
	})
	if err != nil {
		fmt.Printf("FAIL: %v\n", err)
		os.Exit(1)
	}
	defer store.Close()

	// Test 2: Store Memory
	fmt.Println("\nTest 2: Store Memory")
	memory := &entity.Memory{
		ID:        "test:1-5",
		Path:      "test.md",
		StartLine: 1,
		EndLine:   5,
		Content:   "Test content for integration testing",
		Source:    entity.SourceLongTerm,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
		Checksum:  "test123",
	}
	err = store.Store(context.Background(), memory)
	if err != nil {
		fmt.Printf("FAIL: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("PASS: Memory stored successfully")

	// Test 3: Retrieve Memory
	fmt.Println("\nTest 3: Retrieve Memory")
	retrieved, err := store.Get(context.Background(), "test:1-5")
	if err != nil {
		fmt.Printf("FAIL: %v\n", err)
		os.Exit(1)
	}
	if retrieved.Content != memory.Content {
		fmt.Printf("FAIL: Content mismatch\n")
		os.Exit(1)
	}
	fmt.Println("PASS: Memory retrieved successfully")

	// Test 4: Batch Store
	fmt.Println("\nTest 4: Batch Store")
	memories := []*entity.Memory{
		{
			ID:        "batch1:1-5",
			Path:      "batch.md",
			StartLine: 1,
			EndLine:   5,
			Content:   "Batch memory 1",
			Source:    entity.SourceDaily,
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
			Checksum:  "batch1",
		},
		{
			ID:        "batch2:6-10",
			Path:      "batch.md",
			StartLine: 6,
			EndLine:   10,
			Content:   "Batch memory 2",
			Source:    entity.SourceDaily,
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
			Checksum:  "batch2",
		},
	}
	err = store.StoreBatch(context.Background(), memories)
	if err != nil {
		fmt.Printf("FAIL: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("PASS: Batch store successful")

	// Test 5: List Memories
	fmt.Println("\nTest 5: List Memories")
	list, total, err := store.List(context.Background(), nil)
	if err != nil {
		fmt.Printf("FAIL: %v\n", err)
		os.Exit(1)
	}
	if total != 3 {
		fmt.Printf("FAIL: Expected 3 memories, got %d\n", total)
		os.Exit(1)
	}
	fmt.Printf("PASS: Listed %d memories\n", len(list))

	// Test 6: Delete Memory
	fmt.Println("\nTest 6: Delete Memory")
	err = store.Delete(context.Background(), "test:1-5")
	if err != nil {
		fmt.Printf("FAIL: %v\n", err)
		os.Exit(1)
	}

	// Verify deletion
	_, err = store.Get(context.Background(), "test:1-5")
	if err == nil {
		fmt.Printf("FAIL: Memory should have been deleted\n")
		os.Exit(1)
	}
	fmt.Println("PASS: Memory deleted successfully")

	// Test 7: File Manager
	fmt.Println("\nTest 7: File Manager")
	fileMgr, err := filestore.New(&filestore.Config{
		WorkspaceDir:  tmpDir,
		LongTermFile:  "MEMORY.md",
		DailyDir:      "memory",
		FileExtension: ".md",
	})
	if err != nil {
		fmt.Printf("FAIL: %v\n", err)
		os.Exit(1)
	}
	defer fileMgr.Close()

	// Write file
	testContent := []byte("# Test Memory\n\nThis is a test.")
	err = fileMgr.Write(context.Background(), "MEMORY.md", testContent)
	if err != nil {
		fmt.Printf("FAIL: %v\n", err)
		os.Exit(1)
	}

	// Read file
	readContent, err := fileMgr.Read(context.Background(), "MEMORY.md")
	if err != nil {
		fmt.Printf("FAIL: %v\n", err)
		os.Exit(1)
	}
	if string(readContent) != string(testContent) {
		fmt.Printf("FAIL: Content mismatch\n")
		os.Exit(1)
	}
	fmt.Println("PASS: File manager works correctly")

	// Test 8: Source Type Detection
	fmt.Println("\nTest 8: Source Type Detection")
	tests := []struct {
		path     string
		expected entity.SourceType
	}{
		{"MEMORY.md", entity.SourceLongTerm},
		{"memory/2026-03-08.md", entity.SourceDaily},
		{"other/file.md", entity.SourceSession},
	}

	for _, test := range tests {
		result := fileMgr.DetermineSourceType(test.path)
		if result != test.expected {
			fmt.Printf("FAIL: %s should be %s, got %s\n", test.path, test.expected, result)
			os.Exit(1)
		}
	}
	fmt.Println("PASS: Source type detection works correctly")

	// Summary
	fmt.Println("\n==========================")
	fmt.Println("✅ All integration tests passed!")
	fmt.Printf("   Tests run: 8\n")
	fmt.Printf("   Failures: 0\n")
}

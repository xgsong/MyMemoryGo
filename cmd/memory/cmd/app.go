// Package cmd contains the CLI commands for the memory application.
package cmd

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/spf13/viper"
	"github.com/xgsong/MyMemoryGo/internal/application/service"
	"github.com/xgsong/MyMemoryGo/internal/infrastructure/embedding"
	"github.com/xgsong/MyMemoryGo/internal/infrastructure/persistence/filestore"
	"github.com/xgsong/MyMemoryGo/internal/infrastructure/persistence/sqlite"
)

// AppContext holds the initialized application components.
type AppContext struct {
	MemoryApp *service.MemoryApplicationService
	Store     *sqlite.Store
	FileMgr   *filestore.Manager
}

// InitializeApp initializes all application components.
func InitializeApp(ctx context.Context) (*AppContext, error) {
	// Get configuration
	workspaceDir := viper.GetString("workspace.dir")
	dbPath := viper.GetString("storage.db_path")
	walMode := viper.GetBool("storage.wal_mode")

	// Expand home directory
	if len(workspaceDir) >= 2 && workspaceDir[:2] == "~/" {
		home, err := os.UserHomeDir()
		if err != nil {
			return nil, fmt.Errorf("failed to get home directory: %w", err)
		}
		workspaceDir = filepath.Join(home, workspaceDir[2:])
	} else if len(workspaceDir) >= 1 && workspaceDir[0] == '~' {
		home, err := os.UserHomeDir()
		if err != nil {
			return nil, fmt.Errorf("failed to get home directory: %w", err)
		}
		workspaceDir = home
	}
	if len(dbPath) >= 2 && dbPath[:2] == "~/" {
		home, err := os.UserHomeDir()
		if err != nil {
			return nil, fmt.Errorf("failed to get home directory: %w", err)
		}
		dbPath = filepath.Join(home, dbPath[2:])
	} else if len(dbPath) >= 1 && dbPath[0] == '~' {
		home, err := os.UserHomeDir()
		if err != nil {
			return nil, fmt.Errorf("failed to get home directory: %w", err)
		}
		dbPath = home
	}

	// Ensure workspace directory exists
	if err := os.MkdirAll(workspaceDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create workspace directory: %w", err)
	}

	// Ensure database directory exists
	dbDir := filepath.Dir(dbPath)
	if err := os.MkdirAll(dbDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create database directory: %w", err)
	}

	// Initialize embedding provider
	embedConfig := &embedding.Config{
		BaseURL:    viper.GetString("embedding.base_url"),
		Model:      viper.GetString("embedding.model"),
		Dimensions: viper.GetInt("embedding.dimensions"),
		APIKey:     viper.GetString("embedding.api_key"),
		BatchSize:  viper.GetInt("embedding.batch_size"),
		Timeout:    viper.GetDuration("embedding.timeout"),
		MaxRetries: viper.GetInt("embedding.max_retries"),
		RetryDelay: viper.GetDuration("embedding.retry_delay"),
	}

	if embedConfig.Timeout == 0 {
		embedConfig.Timeout = 60 * time.Second
	}
	if embedConfig.MaxRetries == 0 {
		embedConfig.MaxRetries = 3
	}
	if embedConfig.RetryDelay == 0 {
		embedConfig.RetryDelay = 1 * time.Second
	}

	embedProvider, err := embedding.NewProvider(embedConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize embedding provider: %w", err)
	}

	cachedProvider := embedding.NewCachedProvider(embedProvider, 10000)
	slog.Info("Initialized embedding cache",
		"capacity", 10000,
		"provider", embedConfig.Model,
	)

	// Initialize SQLite store
	storeConfig := &sqlite.Config{
		DBPath:           dbPath,
		WALMode:          walMode,
		VectorDimensions: embedConfig.Dimensions,
	}

	store, err := sqlite.New(storeConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize SQLite store: %w", err)
	}

	// Initialize file manager
	fileConfig := &filestore.Config{
		WorkspaceDir:  workspaceDir,
		LongTermFile:  "MEMORY.md",
		DailyDir:      "memory",
		FileExtension: ".md",
	}

	fileMgr, err := filestore.New(fileConfig)
	if err != nil {
		store.Close()
		return nil, fmt.Errorf("failed to initialize file manager: %w", err)
	}

	// Initialize application service
	memoryApp := service.NewMemoryApplicationService(
		store,
		store,
		cachedProvider,
		fileMgr,
	)

	return &AppContext{
		MemoryApp: memoryApp,
		Store:     store,
		FileMgr:   fileMgr,
	}, nil
}

// Cleanup cleans up application resources.
func (app *AppContext) Cleanup() {
	if app.Store != nil {
		app.Store.Close()
	}
	if app.FileMgr != nil {
		app.FileMgr.Close()
	}
}

// WaitForInterrupt blocks until an interrupt signal is received.
func WaitForInterrupt(ctx context.Context) context.Context {
	ctx, cancel := context.WithCancel(ctx)

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	go func() {
		<-sigChan
		cancel()
	}()

	return ctx
}

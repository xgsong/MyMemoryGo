// Package embedding provides OpenAI-compatible embedding client implementations.
// This package supports both local models (Ollama, vLLM, LocalAI) and cloud models (OpenAI, Gemini).
package embedding

import (
	"context"
	"fmt"
)

// Provider defines the interface for embedding generation.
// Implementations must be thread-safe for concurrent use.
type Provider interface {
	// Embed generates an embedding for a single text.
	Embed(ctx context.Context, text string) ([]float32, error)

	// EmbedBatch generates embeddings for multiple texts in a single request.
	// This is more efficient than calling Embed multiple times.
	EmbedBatch(ctx context.Context, texts []string) ([][]float32, error)

	// Model returns the name of the embedding model being used.
	Model() string

	// Dimensions returns the dimensionality of the embeddings.
	Dimensions() int
}

// OpenAICompatibleProvider implements Provider for OpenAI-compatible APIs.
type OpenAICompatibleProvider struct {
	config     *Config
	client     *embedClient
	dimensions int
}

// NewProvider creates a new OpenAI-compatible embedding provider.
func NewProvider(cfg *Config) (*OpenAICompatibleProvider, error) {
	if cfg == nil {
		cfg = DefaultConfig()
	}

	// Validate configuration
	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	// Create HTTP client
	client := newEmbedClient(cfg)

	// Determine dimensions
	dimensions := cfg.GetDimensions()

	return &OpenAICompatibleProvider{
		config:     cfg,
		client:     client,
		dimensions: dimensions,
	}, nil
}

// Embed generates an embedding for a single text.
func (p *OpenAICompatibleProvider) Embed(ctx context.Context, text string) ([]float32, error) {
	embeddings, err := p.EmbedBatch(ctx, []string{text})
	if err != nil {
		return nil, err
	}
	if len(embeddings) == 0 {
		return nil, fmt.Errorf("no embedding returned")
	}
	return embeddings[0], nil
}

// EmbedBatch generates embeddings for multiple texts.
func (p *OpenAICompatibleProvider) EmbedBatch(ctx context.Context, texts []string) ([][]float32, error) {
	if len(texts) == 0 {
		return [][]float32{}, nil
	}

	// Process in batches
	var allEmbeddings [][]float32
	for i := 0; i < len(texts); i += p.config.BatchSize {
		end := i + p.config.BatchSize
		if end > len(texts) {
			end = len(texts)
		}

		batch := texts[i:end]
		embeddings, err := p.client.embedBatchWithRetry(ctx, batch)
		if err != nil {
			return nil, fmt.Errorf("batch %d: %w", i/p.config.BatchSize, err)
		}

		allEmbeddings = append(allEmbeddings, embeddings...)
	}

	return allEmbeddings, nil
}

// Model returns the model name.
func (p *OpenAICompatibleProvider) Model() string {
	return p.config.Model
}

// Dimensions returns the vector dimensions.
func (p *OpenAICompatibleProvider) Dimensions() int {
	return p.dimensions
}

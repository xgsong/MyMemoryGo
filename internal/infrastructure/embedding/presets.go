// Package embedding provides OpenAI-compatible embedding client implementations.
package embedding

import (
	"fmt"
	"time"
)

// inferDimensions infers the embedding dimensions based on model name.
// This function contains known model dimensions for common embedding models.
func inferDimensions(model string) int {
	// OpenAI models
	openAIDims := map[string]int{
		"text-embedding-3-small": 1536,
		"text-embedding-3-large": 3072,
		"text-embedding-ada-002": 1536,
	}

	// Ollama models
	ollamaDims := map[string]int{
		"nomic-embed-text":       768,
		"all-minilm":             384,
		"mxbai-embed-large":      1024,
		"snowflake-arctic-embed": 1024,
	}

	// vLLM/HuggingFace models
	vllmDims := map[string]int{
		"BAAI/bge-large-en-v1.5":                 1024,
		"BAAI/bge-base-en-v1.5":                  768,
		"sentence-transformers/all-MiniLM-L6-v2": 384,
	}

	// Merge all known models
	knownDims := make(map[string]int)
	for k, v := range openAIDims {
		knownDims[k] = v
	}
	for k, v := range ollamaDims {
		knownDims[k] = v
	}
	for k, v := range vllmDims {
		knownDims[k] = v
	}

	// Check if model is known
	if dim, ok := knownDims[model]; ok {
		return dim
	}

	// Default dimension (OpenAI text-embedding-3-small)
	return 1536
}

// PresetConfig returns a Config for common embedding providers.
func PresetConfig(preset string) (*Config, error) {
	presets := map[string]*Config{
		// OpenAI presets
		"openai-small": {
			BaseURL:    "https://api.openai.com/v1",
			Model:      "text-embedding-3-small",
			Dimensions: 1536,
			BatchSize:  100,
			Timeout:    30 * time.Second,
		},
		"openai-large": {
			BaseURL:    "https://api.openai.com/v1",
			Model:      "text-embedding-3-large",
			Dimensions: 3072,
			BatchSize:  100,
			Timeout:    30 * time.Second,
		},

		// Ollama presets
		"ollama-nomic": {
			BaseURL:    "http://localhost:11434/v1",
			Model:      "nomic-embed-text",
			Dimensions: 768,
			BatchSize:  50,
			Timeout:    60 * time.Second,
		},
		"ollama-minilm": {
			BaseURL:    "http://localhost:11434/v1",
			Model:      "all-minilm",
			Dimensions: 384,
			BatchSize:  100,
			Timeout:    60 * time.Second,
		},

		// vLLM presets
		"vllm-bge-large": {
			BaseURL:    "http://localhost:8000/v1",
			Model:      "BAAI/bge-large-en-v1.5",
			Dimensions: 1024,
			BatchSize:  50,
			Timeout:    60 * time.Second,
		},

		// LocalAI presets
		"localai-all-minilm": {
			BaseURL:    "http://localhost:8080/v1",
			Model:      "all-MiniLM-L6-v2",
			Dimensions: 384,
			BatchSize:  100,
			Timeout:    60 * time.Second,
		},
	}

	cfg, ok := presets[preset]
	if !ok {
		return nil, fmt.Errorf("unknown preset: %s", preset)
	}

	return cfg, nil
}

// NewProviderFromPreset creates a provider using a preset configuration.
func NewProviderFromPreset(preset string, apiKey string) (*OpenAICompatibleProvider, error) {
	cfg, err := PresetConfig(preset)
	if err != nil {
		return nil, err
	}

	// Override API key if provided
	if apiKey != "" {
		cfg.APIKey = apiKey
	}

	return NewProvider(cfg)
}

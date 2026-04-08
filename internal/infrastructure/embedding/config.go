// Package embedding provides OpenAI-compatible embedding client implementations.
package embedding

import (
	"fmt"
	"net/http"
	"time"
)

// Config holds the configuration for OpenAI-compatible embedding providers.
type Config struct {
	// BaseURL is the API base URL.
	// Examples:
	//   - OpenAI: https://api.openai.com/v1
	//   - Ollama: http://localhost:11434/v1
	//   - vLLM: http://localhost:8000/v1
	//   - LocalAI: http://localhost:8080/v1
	BaseURL string `json:"base_url" yaml:"base_url"`

	// APIKey is the API key for authentication.
	// Optional for local models.
	APIKey string `json:"api_key" yaml:"api_key"`

	// Model is the embedding model name.
	// Examples:
	//   - OpenAI: text-embedding-3-small, text-embedding-3-large
	//   - Ollama: nomic-embed-text, all-minilm
	//   - vLLM: BAAI/bge-large-en-v1.5
	Model string `json:"model" yaml:"model"`

	// Dimensions is the vector dimensionality.
	// Optional: some models support custom dimensions.
	Dimensions int `json:"dimensions" yaml:"dimensions"`

	// BatchSize is the maximum number of texts per batch request.
	BatchSize int `json:"batch_size" yaml:"batch_size"`

	// Timeout is the HTTP request timeout.
	Timeout time.Duration `json:"timeout" yaml:"timeout"`

	// MaxRetries is the maximum number of retry attempts.
	MaxRetries int `json:"max_retries" yaml:"max_retries"`

	// RetryDelay is the delay between retry attempts.
	RetryDelay time.Duration `json:"retry_delay" yaml:"retry_delay"`

	// HTTPClient is a custom HTTP client (optional).
	HTTPClient *http.Client `json:"-"`
}

// DefaultConfig returns a Config with sensible defaults.
func DefaultConfig() *Config {
	return &Config{
		Timeout:    30 * time.Second,
		BatchSize:  100,
		MaxRetries: 3,
		RetryDelay: 1 * time.Second,
	}
}

// Validate validates the configuration and applies defaults.
func (c *Config) Validate() error {
	// Apply defaults
	if c.Timeout == 0 {
		c.Timeout = 30 * time.Second
	}
	if c.BatchSize == 0 {
		c.BatchSize = 100
	}
	if c.MaxRetries == 0 {
		c.MaxRetries = 3
	}
	if c.RetryDelay == 0 {
		c.RetryDelay = 1 * time.Second
	}

	// Validate required fields
	if c.BaseURL == "" {
		return fmt.Errorf("base_url is required")
	}
	if c.Model == "" {
		return fmt.Errorf("model is required")
	}

	return nil
}

// GetHTTPClient returns the configured HTTP client or creates a default one.
func (c *Config) GetHTTPClient() *http.Client {
	if c.HTTPClient != nil {
		return c.HTTPClient
	}
	return &http.Client{
		Timeout: c.Timeout,
	}
}

// GetDimensions returns the configured dimensions or infers them from the model name.
func (c *Config) GetDimensions() int {
	if c.Dimensions > 0 {
		return c.Dimensions
	}
	return inferDimensions(c.Model)
}

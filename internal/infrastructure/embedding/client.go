// Package embedding provides OpenAI-compatible embedding client implementations.
package embedding

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// embeddingRequest represents an OpenAI embedding API request.
type embeddingRequest struct {
	Model      string   `json:"model"`
	Input      []string `json:"input"`
	Dimensions int      `json:"dimensions,omitempty"`
}

// embeddingResponse represents an OpenAI embedding API response.
type embeddingResponse struct {
	Object string `json:"object"`
	Data   []struct {
		Object    string    `json:"object"`
		Index     int       `json:"index"`
		Embedding []float32 `json:"embedding"`
	} `json:"data"`
	Model string `json:"model"`
	Usage struct {
		PromptTokens int `json:"prompt_tokens"`
		TotalTokens  int `json:"total_tokens"`
	} `json:"usage"`
}

// embedClient handles HTTP communication with the embedding API.
type embedClient struct {
	config     *Config
	httpClient *http.Client
}

// newEmbedClient creates a new embedding client.
func newEmbedClient(cfg *Config) *embedClient {
	return &embedClient{
		config:     cfg,
		httpClient: cfg.GetHTTPClient(),
	}
}

// embedBatchWithRetry performs embedding with retry logic.
func (c *embedClient) embedBatchWithRetry(ctx context.Context, texts []string) ([][]float32, error) {
	var lastErr error

	for attempt := 0; attempt <= c.config.MaxRetries; attempt++ {
		if attempt > 0 {
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(c.config.RetryDelay):
			}
		}

		embeddings, err := c.callEmbeddingAPI(ctx, texts)
		if err == nil {
			return embeddings, nil
		}

		lastErr = err

		// Don't retry on certain errors
		if isNonRetryableError(err) {
			break
		}
	}

	return nil, fmt.Errorf("after %d retries: %w", c.config.MaxRetries, lastErr)
}

// callEmbeddingAPI makes the HTTP request to the embedding API.
func (c *embedClient) callEmbeddingAPI(ctx context.Context, texts []string) ([][]float32, error) {
	// Build request
	req := embeddingRequest{
		Model: c.config.Model,
		Input: texts,
	}
	if c.config.Dimensions > 0 {
		req.Dimensions = c.config.Dimensions
	}

	// Serialize request
	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	// Build URL
	url := c.config.BaseURL
	if url[len(url)-1] != '/' {
		url += "/"
	}
	url += "embeddings"

	// Create HTTP request
	httpReq, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	// Set headers
	httpReq.Header.Set("Content-Type", "application/json")
	if c.config.APIKey != "" {
		httpReq.Header.Set("Authorization", "Bearer "+c.config.APIKey)
	}

	// Send request
	httpResp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("http request: %w", err)
	}
	defer httpResp.Body.Close()

	// Read response
	respBody, err := io.ReadAll(httpResp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	// Check status code
	if httpResp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("api error (status %d): %s", httpResp.StatusCode, respBody)
	}

	// Parse response
	var resp embeddingResponse
	if err := json.Unmarshal(respBody, &resp); err != nil {
		return nil, fmt.Errorf("unmarshal response: %w", err)
	}

	// Extract embeddings
	result := make([][]float32, len(resp.Data))
	for i, d := range resp.Data {
		result[i] = d.Embedding
	}

	return result, nil
}

// isNonRetryableError checks if an error should not be retried.
func isNonRetryableError(err error) bool {
	// Add logic to identify non-retryable errors
	// For now, return false to retry all errors
	return false
}

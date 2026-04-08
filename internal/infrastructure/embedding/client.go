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

	"github.com/xgsong/MyMemoryGo/internal/pkg/errors"
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
				return nil, errors.WrapOp(errors.CodeCancelled, "embedBatchWithRetry", "context cancelled", ctx.Err())
			case <-time.After(c.config.RetryDelay):
			}
		}

		embeddings, err := c.callEmbeddingAPI(ctx, texts)
		if err == nil {
			return embeddings, nil
		}

		lastErr = err

		if isNonRetryableError(err) {
			break
		}
	}

	return nil, errors.WrapOp(errors.CodeNetwork, "embedBatchWithRetry", "max retries exceeded", lastErr)
}

// callEmbeddingAPI makes the HTTP request to the embedding API.
func (c *embedClient) callEmbeddingAPI(ctx context.Context, texts []string) ([][]float32, error) {
	req := embeddingRequest{
		Model: c.config.Model,
		Input: texts,
	}
	if c.config.Dimensions > 0 {
		req.Dimensions = c.config.Dimensions
	}

	body, err := json.Marshal(req)
	if err != nil {
		return nil, errors.WrapOp(errors.CodeNetwork, "callEmbeddingAPI", "marshal request failed", err)
	}

	url := c.config.BaseURL
	if url[len(url)-1] != '/' {
		url += "/"
	}
	url += "embeddings"

	httpReq, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(body))
	if err != nil {
		return nil, errors.WrapOp(errors.CodeNetwork, "callEmbeddingAPI", "create request failed", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	if c.config.APIKey != "" {
		httpReq.Header.Set("Authorization", "Bearer "+c.config.APIKey)
	}

	httpResp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, errors.WrapOp(errors.CodeNetwork, "callEmbeddingAPI", "http request failed", err)
	}
	defer httpResp.Body.Close()

	respBody, err := io.ReadAll(httpResp.Body)
	if err != nil {
		return nil, errors.WrapOp(errors.CodeNetwork, "callEmbeddingAPI", "read response failed", err)
	}

	if httpResp.StatusCode != http.StatusOK {
		return nil, errors.WrapOp(errors.CodeNetwork, "callEmbeddingAPI", "api error", fmt.Errorf("status %d: %s", httpResp.StatusCode, respBody))
	}

	var resp embeddingResponse
	if err := json.Unmarshal(respBody, &resp); err != nil {
		return nil, errors.WrapOp(errors.CodeNetwork, "callEmbeddingAPI", "unmarshal response failed", err)
	}

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

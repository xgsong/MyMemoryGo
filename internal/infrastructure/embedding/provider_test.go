package embedding_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/xgsong/MyMemoryGo/internal/infrastructure/embedding"
)

func TestDefaultConfig(t *testing.T) {
	cfg := embedding.DefaultConfig()

	assert.Equal(t, 30*time.Second, cfg.Timeout)
	assert.Equal(t, 100, cfg.BatchSize)
	assert.Equal(t, 3, cfg.MaxRetries)
	assert.Equal(t, 1*time.Second, cfg.RetryDelay)
}

func TestNewProvider(t *testing.T) {
	tests := []struct {
		name      string
		config    *embedding.Config
		wantErr   bool
		errString string
	}{
		{
			name: "valid config",
			config: &embedding.Config{
				BaseURL: "http://localhost:11434/v1",
				Model:   "nomic-embed-text",
			},
			wantErr: false,
		},
		{
			name: "valid config with all fields",
			config: &embedding.Config{
				BaseURL:    "http://localhost:11434/v1",
				Model:      "nomic-embed-text",
				Dimensions: 768,
				BatchSize:  50,
				Timeout:    60 * time.Second,
				MaxRetries: 5,
				RetryDelay: 2 * time.Second,
			},
			wantErr: false,
		},
		{
			name:      "missing base_url",
			config:    &embedding.Config{Model: "test"},
			wantErr:   true,
			errString: "base_url is required",
		},
		{
			name:      "missing model",
			config:    &embedding.Config{BaseURL: "http://localhost"},
			wantErr:   true,
			errString: "model is required",
		},
		{
			name:    "nil config uses defaults but fails",
			config:  nil,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			provider, err := embedding.NewProvider(tt.config)

			if tt.wantErr {
				require.Error(t, err)
				if tt.errString != "" {
					assert.Contains(t, err.Error(), tt.errString)
				}
				assert.Nil(t, provider)
			} else {
				require.NoError(t, err)
				assert.NotNil(t, provider)
			}
		})
	}
}

func TestProvider_Embed(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "POST", r.Method)
		assert.Equal(t, "/embeddings", r.URL.Path)
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))

		var req mockEmbeddingRequest
		err := json.NewDecoder(r.Body).Decode(&req)
		require.NoError(t, err)
		assert.Equal(t, "test-model", req.Model)
		assert.Len(t, req.Input, 1)

		resp := mockEmbeddingResponse{
			Object: "list",
			Data: []mockEmbeddingData{
				{
					Object:    "embedding",
					Index:     0,
					Embedding: []float32{0.1, 0.2, 0.3, 0.4, 0.5},
				},
			},
			Model: "test-model",
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	provider, err := embedding.NewProvider(&embedding.Config{
		BaseURL: server.URL,
		Model:   "test-model",
	})
	require.NoError(t, err)

	ctx := context.Background()
	vec, err := provider.Embed(ctx, "test text")
	require.NoError(t, err)
	assert.Equal(t, []float32{0.1, 0.2, 0.3, 0.4, 0.5}, vec)
}

func TestProvider_EmbedBatch(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req mockEmbeddingRequest
		err := json.NewDecoder(r.Body).Decode(&req)
		require.NoError(t, err)

		resp := mockEmbeddingResponse{
			Object: "list",
			Data:   make([]mockEmbeddingData, len(req.Input)),
			Model:  req.Model,
		}

		for i := range resp.Data {
			resp.Data[i].Object = "embedding"
			resp.Data[i].Index = i
			resp.Data[i].Embedding = []float32{float32(i), float32(i + 1)}
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	provider, err := embedding.NewProvider(&embedding.Config{
		BaseURL:   server.URL,
		Model:     "test-model",
		BatchSize: 2,
	})
	require.NoError(t, err)

	ctx := context.Background()

	t.Run("empty batch", func(t *testing.T) {
		embeddings, err := provider.EmbedBatch(ctx, []string{})
		require.NoError(t, err)
		assert.Len(t, embeddings, 0)
	})

	t.Run("single text", func(t *testing.T) {
		embeddings, err := provider.EmbedBatch(ctx, []string{"text1"})
		require.NoError(t, err)
		assert.Len(t, embeddings, 1)
	})

	t.Run("multiple texts", func(t *testing.T) {
		texts := []string{"text1", "text2", "text3"}
		embeddings, err := provider.EmbedBatch(ctx, texts)
		require.NoError(t, err)
		assert.Len(t, embeddings, 3)
	})

	t.Run("large batch", func(t *testing.T) {
		texts := make([]string, 10)
		for i := range texts {
			texts[i] = "text"
		}
		embeddings, err := provider.EmbedBatch(ctx, texts)
		require.NoError(t, err)
		assert.Len(t, embeddings, 10)
	})
}

func TestProvider_Embed_ErrorHandling(t *testing.T) {
	t.Run("server error", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte("internal error"))
		}))
		defer server.Close()

		provider, err := embedding.NewProvider(&embedding.Config{
			BaseURL:    server.URL,
			Model:      "test-model",
			MaxRetries: 1,
			RetryDelay: 1 * time.Millisecond,
		})
		require.NoError(t, err)

		_, err = provider.Embed(context.Background(), "test")
		require.Error(t, err)
	})

	t.Run("malformed response", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte("invalid json"))
		}))
		defer server.Close()

		provider, err := embedding.NewProvider(&embedding.Config{
			BaseURL:    server.URL,
			Model:      "test-model",
			MaxRetries: 1,
			RetryDelay: 1 * time.Millisecond,
		})
		require.NoError(t, err)

		_, err = provider.Embed(context.Background(), "test")
		require.Error(t, err)
	})

	t.Run("empty response", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			resp := mockEmbeddingResponse{
				Object: "list",
				Data:   []mockEmbeddingData{},
				Model:  "test-model",
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(resp)
		}))
		defer server.Close()

		provider, err := embedding.NewProvider(&embedding.Config{
			BaseURL: server.URL,
			Model:   "test-model",
		})
		require.NoError(t, err)

		_, err = provider.Embed(context.Background(), "test")
		require.Error(t, err)
	})
}

func TestProvider_Model(t *testing.T) {
	provider, err := embedding.NewProvider(&embedding.Config{
		BaseURL: "http://localhost",
		Model:   "test-model",
	})
	require.NoError(t, err)

	assert.Equal(t, "test-model", provider.Model())
}

func TestProvider_Dimensions(t *testing.T) {
	tests := []struct {
		name           string
		config         *embedding.Config
		wantDimensions int
	}{
		{
			name: "explicit dimensions",
			config: &embedding.Config{
				BaseURL:    "http://localhost",
				Model:      "test-model",
				Dimensions: 768,
			},
			wantDimensions: 768,
		},
		{
			name: "inferred from text-embedding-3-small",
			config: &embedding.Config{
				BaseURL: "http://localhost",
				Model:   "text-embedding-3-small",
			},
			wantDimensions: 1536,
		},
		{
			name: "inferred from text-embedding-3-large",
			config: &embedding.Config{
				BaseURL: "http://localhost",
				Model:   "text-embedding-3-large",
			},
			wantDimensions: 3072,
		},
		{
			name: "inferred from nomic-embed-text",
			config: &embedding.Config{
				BaseURL: "http://localhost",
				Model:   "nomic-embed-text",
			},
			wantDimensions: 768,
		},
		{
			name: "inferred from all-minilm",
			config: &embedding.Config{
				BaseURL: "http://localhost",
				Model:   "all-minilm",
			},
			wantDimensions: 384,
		},
		{
			name: "default for unknown model",
			config: &embedding.Config{
				BaseURL: "http://localhost",
				Model:   "unknown-model",
			},
			wantDimensions: 1536,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			provider, err := embedding.NewProvider(tt.config)
			require.NoError(t, err)
			assert.Equal(t, tt.wantDimensions, provider.Dimensions())
		})
	}
}

func TestProvider_WithAPIKey(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth := r.Header.Get("Authorization")
		assert.Equal(t, "Bearer test-api-key", auth)

		resp := mockEmbeddingResponse{
			Object: "list",
			Data: []mockEmbeddingData{{
				Object:    "embedding",
				Index:     0,
				Embedding: []float32{0.1, 0.2},
			}},
			Model: "test-model",
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	provider, err := embedding.NewProvider(&embedding.Config{
		BaseURL: server.URL,
		Model:   "test-model",
		APIKey:  "test-api-key",
	})
	require.NoError(t, err)

	_, err = provider.Embed(context.Background(), "test")
	require.NoError(t, err)
}

func TestPresetConfig(t *testing.T) {
	tests := []struct {
		name      string
		preset    string
		wantErr   bool
		wantModel string
	}{
		{
			name:      "openai-small",
			preset:    "openai-small",
			wantErr:   false,
			wantModel: "text-embedding-3-small",
		},
		{
			name:      "openai-large",
			preset:    "openai-large",
			wantErr:   false,
			wantModel: "text-embedding-3-large",
		},
		{
			name:      "ollama-nomic",
			preset:    "ollama-nomic",
			wantErr:   false,
			wantModel: "nomic-embed-text",
		},
		{
			name:      "ollama-minilm",
			preset:    "ollama-minilm",
			wantErr:   false,
			wantModel: "all-minilm",
		},
		{
			name:      "vllm-bge-large",
			preset:    "vllm-bge-large",
			wantErr:   false,
			wantModel: "BAAI/bge-large-en-v1.5",
		},
		{
			name:      "localai-all-minilm",
			preset:    "localai-all-minilm",
			wantErr:   false,
			wantModel: "all-MiniLM-L6-v2",
		},
		{
			name:    "invalid preset",
			preset:  "invalid-preset",
			wantErr: true,
		},
		{
			name:    "empty preset",
			preset:  "",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg, err := embedding.PresetConfig(tt.preset)

			if tt.wantErr {
				require.Error(t, err)
				assert.Nil(t, cfg)
			} else {
				require.NoError(t, err)
				assert.NotNil(t, cfg)
				assert.Equal(t, tt.wantModel, cfg.Model)
				assert.NotEmpty(t, cfg.BaseURL)
				assert.Greater(t, cfg.Dimensions, 0)
				assert.Greater(t, cfg.BatchSize, 0)
				assert.Greater(t, cfg.Timeout, time.Duration(0))
			}
		})
	}
}

func TestNewProviderFromPreset(t *testing.T) {
	t.Run("ollama-nomic without API key", func(t *testing.T) {
		provider, err := embedding.NewProviderFromPreset("ollama-nomic", "")
		require.NoError(t, err)
		require.NotNil(t, provider)
		assert.Equal(t, "nomic-embed-text", provider.Model())
		assert.Equal(t, 768, provider.Dimensions())
	})

	t.Run("openai-small without API key", func(t *testing.T) {
		provider, err := embedding.NewProviderFromPreset("openai-small", "")
		require.NoError(t, err)
		require.NotNil(t, provider)
		assert.Equal(t, "text-embedding-3-small", provider.Model())
		assert.Equal(t, 1536, provider.Dimensions())
	})

	t.Run("with API key", func(t *testing.T) {
		provider, err := embedding.NewProviderFromPreset("openai-small", "sk-test")
		require.NoError(t, err)
		require.NotNil(t, provider)
	})

	t.Run("invalid preset", func(t *testing.T) {
		provider, err := embedding.NewProviderFromPreset("invalid", "")
		require.Error(t, err)
		assert.Nil(t, provider)
	})
}

func TestProvider_ContextCancellation(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(100 * time.Millisecond) // Slow response
		resp := mockEmbeddingResponse{
			Object: "list",
			Data: []mockEmbeddingData{{
				Object:    "embedding",
				Index:     0,
				Embedding: []float32{0.1},
			}},
			Model: "test-model",
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	provider, err := embedding.NewProvider(&embedding.Config{
		BaseURL: server.URL,
		Model:   "test-model",
	})
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	_, err = provider.Embed(ctx, "test")
	require.Error(t, err)
}

func TestProvider_CustomHTTPClient(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := mockEmbeddingResponse{
			Object: "list",
			Data: []mockEmbeddingData{{
				Object:    "embedding",
				Index:     0,
				Embedding: []float32{0.1},
			}},
			Model: "test-model",
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	customClient := &http.Client{
		Timeout: 5 * time.Second,
	}

	provider, err := embedding.NewProvider(&embedding.Config{
		BaseURL:    server.URL,
		Model:      "test-model",
		HTTPClient: customClient,
	})
	require.NoError(t, err)
	assert.NotNil(t, provider)
}

func TestProvider_DimensionsParameter(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req mockEmbeddingRequest
		json.NewDecoder(r.Body).Decode(&req)

		// Check if dimensions was sent
		assert.Greater(t, req.Dimensions, 0)

		resp := mockEmbeddingResponse{
			Object: "list",
			Data: []mockEmbeddingData{{
				Object:    "embedding",
				Index:     0,
				Embedding: []float32{0.1, 0.2, 0.3},
			}},
			Model: "test-model",
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	provider, err := embedding.NewProvider(&embedding.Config{
		BaseURL:    server.URL,
		Model:      "test-model",
		Dimensions: 256,
	})
	require.NoError(t, err)

	_, err = provider.Embed(context.Background(), "test")
	require.NoError(t, err)
}

// Mock types for testing (internal to test package)
type mockEmbeddingRequest struct {
	Model      string   `json:"model"`
	Input      []string `json:"input"`
	Dimensions int      `json:"dimensions,omitempty"`
}

type mockEmbeddingData struct {
	Object    string    `json:"object"`
	Index     int       `json:"index"`
	Embedding []float32 `json:"embedding"`
}

type mockEmbeddingResponse struct {
	Object string              `json:"object"`
	Data   []mockEmbeddingData `json:"data"`
	Model  string              `json:"model"`
}

// Benchmark tests
func BenchmarkProvider_Embed(b *testing.B) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := mockEmbeddingResponse{
			Object: "list",
			Data: []mockEmbeddingData{{
				Object:    "embedding",
				Index:     0,
				Embedding: []float32{0.1, 0.2, 0.3, 0.4, 0.5},
			}},
			Model: "test-model",
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	provider, _ := embedding.NewProvider(&embedding.Config{
		BaseURL: server.URL,
		Model:   "test-model",
	})
	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		provider.Embed(ctx, "test text")
	}
}

func BenchmarkProvider_EmbedBatch(b *testing.B) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req mockEmbeddingRequest
		json.NewDecoder(r.Body).Decode(&req)

		resp := mockEmbeddingResponse{
			Object: "list",
			Data:   make([]mockEmbeddingData, len(req.Input)),
			Model:  "test-model",
		}
		for i := range resp.Data {
			resp.Data[i] = mockEmbeddingData{
				Object:    "embedding",
				Index:     i,
				Embedding: []float32{0.1, 0.2, 0.3},
			}
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	provider, _ := embedding.NewProvider(&embedding.Config{
		BaseURL: server.URL,
		Model:   "test-model",
	})
	ctx := context.Background()
	texts := []string{"text1", "text2", "text3", "text4", "text5"}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		provider.EmbedBatch(ctx, texts)
	}
}

func BenchmarkPresetConfig(b *testing.B) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		embedding.PresetConfig("openai-small")
	}
}

func BenchmarkNewProvider(b *testing.B) {
	cfg := &embedding.Config{
		BaseURL: "http://localhost",
		Model:   "test-model",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		embedding.NewProvider(cfg)
	}
}

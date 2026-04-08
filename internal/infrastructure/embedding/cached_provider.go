package embedding

import (
	"context"
	"crypto/sha256"
	"encoding/hex"

	"github.com/samber/hot"
)

// CachedProvider wraps an embedding provider with caching support.
type CachedProvider struct {
	provider Provider
	cache    *hot.HotCache[string, []float32]
}

// NewCachedProvider creates a new cached embedding provider with LRU eviction.
func NewCachedProvider(provider Provider, capacity int) *CachedProvider {
	if capacity <= 0 {
		capacity = 10000
	}

	cache := hot.NewHotCache[string, []float32](hot.LRU, capacity).Build()

	return &CachedProvider{
		provider: provider,
		cache:    cache,
	}
}

// hashText creates a SHA256 hash of the text for cache key.
func hashText(text string) string {
	hash := sha256.Sum256([]byte(text))
	return hex.EncodeToString(hash[:])
}

// Embed generates an embedding for a single text with caching.
func (p *CachedProvider) Embed(ctx context.Context, text string) ([]float32, error) {
	key := hashText(text)

	if emb, found, _ := p.cache.Get(key); found {
		return emb, nil
	}

	emb, err := p.provider.Embed(ctx, text)
	if err != nil {
		return nil, err
	}

	p.cache.Set(key, emb)
	return emb, nil
}

// EmbedBatch generates embeddings for multiple texts with caching.
func (p *CachedProvider) EmbedBatch(ctx context.Context, texts []string) ([][]float32, error) {
	if len(texts) == 0 {
		return [][]float32{}, nil
	}

	embeddings := make([][]float32, len(texts))
	missIndexes := make([]int, 0, len(texts))

	for i, text := range texts {
		key := hashText(text)
		if emb, found, _ := p.cache.Get(key); found {
			embeddings[i] = emb
		} else {
			missIndexes = append(missIndexes, i)
		}
	}

	if len(missIndexes) == 0 {
		return embeddings, nil
	}

	missTexts := make([]string, len(missIndexes))
	for i, idx := range missIndexes {
		missTexts[i] = texts[idx]
	}

	missEmbeddings, err := p.provider.EmbedBatch(ctx, missTexts)
	if err != nil {
		return nil, err
	}

	for i, idx := range missIndexes {
		embeddings[idx] = missEmbeddings[i]
		key := hashText(texts[idx])
		p.cache.Set(key, missEmbeddings[i])
	}

	return embeddings, nil
}

// Model returns the model name from the underlying provider.
func (p *CachedProvider) Model() string {
	return p.provider.Model()
}

// Dimensions returns the dimensions from the underlying provider.
func (p *CachedProvider) Dimensions() int {
	return p.provider.Dimensions()
}

// Clear clears all items from the cache.
func (p *CachedProvider) Clear() {
	p.cache.Purge()
}

// Len returns the current number of items in cache.
func (p *CachedProvider) Len() int {
	return p.cache.Len()
}

// Package repository_test tests the repository interfaces and types.
package repository_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/xgsong/MyMemoryGo/internal/domain/entity"
	"github.com/xgsong/MyMemoryGo/internal/domain/repository"
)

func TestDefaultSearchOptions(t *testing.T) {
	opts := repository.DefaultSearchOptions()

	assert.Equal(t, 10, opts.Limit)
	assert.Equal(t, 0.5, opts.MinScore)
	assert.Equal(t, 0.7, opts.VectorWeight)
	assert.Equal(t, 0.3, opts.FulltextWeight)
	assert.False(t, opts.UseMMR)
	assert.False(t, opts.UseDecay)
	assert.Empty(t, opts.SourceFilter)
}

func TestSearchOptionsBuilder(t *testing.T) {
	t.Run("builds default options when no methods called", func(t *testing.T) {
		builder := repository.NewSearchOptionsBuilder()
		opts := builder.Build()

		defaultOpts := repository.DefaultSearchOptions()
		assert.Equal(t, defaultOpts, opts)
	})

	t.Run("WithLimit sets limit", func(t *testing.T) {
		opts := repository.NewSearchOptionsBuilder().
			WithLimit(50).
			Build()

		assert.Equal(t, 50, opts.Limit)
	})

	t.Run("WithMinScore sets minimum score", func(t *testing.T) {
		opts := repository.NewSearchOptionsBuilder().
			WithMinScore(0.75).
			Build()

		assert.Equal(t, 0.75, opts.MinScore)
	})

	t.Run("WithSourceFilter sets source filter", func(t *testing.T) {
		sources := []entity.SourceType{entity.SourceLongTerm, entity.SourceDaily}
		opts := repository.NewSearchOptionsBuilder().
			WithSourceFilter(sources).
			Build()

		assert.Equal(t, sources, opts.SourceFilter)
	})

	t.Run("WithMMR enables MMR and sets lambda", func(t *testing.T) {
		opts := repository.NewSearchOptionsBuilder().
			WithMMR(0.6).
			Build()

		assert.True(t, opts.UseMMR)
		assert.Equal(t, 0.6, opts.MMRLambda)
	})

	t.Run("WithDecay enables decay and sets half-life", func(t *testing.T) {
		halfLife := 30 * 24 * time.Hour
		opts := repository.NewSearchOptionsBuilder().
			WithDecay(halfLife).
			Build()

		assert.True(t, opts.UseDecay)
		assert.Equal(t, halfLife, opts.DecayHalfLife)
	})

	t.Run("WithWeights sets vector and fulltext weights", func(t *testing.T) {
		opts := repository.NewSearchOptionsBuilder().
			WithWeights(0.8, 0.2).
			Build()

		assert.Equal(t, 0.8, opts.VectorWeight)
		assert.Equal(t, 0.2, opts.FulltextWeight)
	})

	t.Run("chains multiple methods correctly", func(t *testing.T) {
		sources := []entity.SourceType{entity.SourceLongTerm}
		halfLife := 7 * 24 * time.Hour

		opts := repository.NewSearchOptionsBuilder().
			WithLimit(20).
			WithMinScore(0.6).
			WithSourceFilter(sources).
			WithMMR(0.7).
			WithDecay(halfLife).
			WithWeights(0.6, 0.4).
			Build()

		assert.Equal(t, 20, opts.Limit)
		assert.Equal(t, 0.6, opts.MinScore)
		assert.Equal(t, sources, opts.SourceFilter)
		assert.True(t, opts.UseMMR)
		assert.Equal(t, 0.7, opts.MMRLambda)
		assert.True(t, opts.UseDecay)
		assert.Equal(t, halfLife, opts.DecayHalfLife)
		assert.Equal(t, 0.6, opts.VectorWeight)
		assert.Equal(t, 0.4, opts.FulltextWeight)
	})
}

func TestFileOperation_Constants(t *testing.T) {
	assert.Equal(t, "create", string(repository.FileOperationCreate))
	assert.Equal(t, "modify", string(repository.FileOperationModify))
	assert.Equal(t, "delete", string(repository.FileOperationDelete))
}

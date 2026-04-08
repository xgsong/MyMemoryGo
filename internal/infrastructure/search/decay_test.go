package search_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/xgsong/MyMemoryGo/internal/domain/entity"
	"github.com/xgsong/MyMemoryGo/internal/infrastructure/search"
)

func TestNewTemporalDecayCalculator(t *testing.T) {
	halfLife := 30 * 24 * time.Hour
	calc := search.NewTemporalDecayCalculator(halfLife)

	require.NotNil(t, calc)
	assert.Equal(t, halfLife, calc.DefaultHalfLife)
	assert.Len(t, calc.EvergreenPatterns, 2)
}

func TestTemporalDecayCalculator_Apply(t *testing.T) {
	halfLife := 30 * 24 * time.Hour

	tests := []struct {
		name         string
		hits         []*entity.SearchHit
		halfLife     time.Duration
		checkResults func(t *testing.T, hits []*entity.SearchHit)
	}{
		{
			name:     "empty hits",
			hits:     []*entity.SearchHit{},
			halfLife: halfLife,
			checkResults: func(t *testing.T, hits []*entity.SearchHit) {
				assert.Empty(t, hits)
			},
		},
		{
			name:     "nil hits",
			hits:     nil,
			halfLife: halfLife,
			checkResults: func(t *testing.T, hits []*entity.SearchHit) {
				assert.Nil(t, hits)
			},
		},
		{
			name: "recent memory",
			hits: []*entity.SearchHit{
				{
					Entry: &entity.Entry{
						ID:        "recent",
						Score:     1.0,
						Timestamp: time.Now(),
						Path:      "memory/2026-03-08.md",
					},
				},
			},
			halfLife: halfLife,
			checkResults: func(t *testing.T, hits []*entity.SearchHit) {
				// Recent memory should have high score
				assert.Greater(t, hits[0].Score, 0.9)
			},
		},
		{
			name: "old memory (90 days)",
			hits: []*entity.SearchHit{
				{
					Entry: &entity.Entry{
						ID:        "old",
						Score:     1.0,
						Timestamp: time.Now().Add(-90 * 24 * time.Hour),
						Path:      "memory/2026-03-08.md",
					},
				},
			},
			halfLife: halfLife,
			checkResults: func(t *testing.T, hits []*entity.SearchHit) {
				// 90 days old with 30-day half-life: multiplier ≈ 0.125
				assert.Less(t, hits[0].Score, 0.2)
				assert.Greater(t, hits[0].Score, 0.05)
			},
		},
		{
			name: "evergreen memory (MEMORY.md)",
			hits: []*entity.SearchHit{
				{
					Entry: &entity.Entry{
						ID:        "evergreen",
						Score:     1.0,
						Timestamp: time.Now().Add(-365 * 24 * time.Hour),
						Path:      "MEMORY.md",
					},
				},
			},
			halfLife: halfLife,
			checkResults: func(t *testing.T, hits []*entity.SearchHit) {
				// Evergreen memory should maintain high score
				assert.Equal(t, 1.0, hits[0].Score)
			},
		},
		{
			name: "evergreen memory lowercase",
			hits: []*entity.SearchHit{
				{
					Entry: &entity.Entry{
						ID:        "evergreen-lower",
						Score:     1.0,
						Timestamp: time.Now().Add(-365 * 24 * time.Hour),
						Path:      "memory.md",
					},
				},
			},
			halfLife: halfLife,
			checkResults: func(t *testing.T, hits []*entity.SearchHit) {
				// Lowercase evergreen should also maintain score
				assert.Equal(t, 1.0, hits[0].Score)
			},
		},
		{
			name: "use default half-life when zero provided",
			hits: []*entity.SearchHit{
				{
					Entry: &entity.Entry{
						ID:        "test",
						Score:     1.0,
						Timestamp: time.Now().Add(-30 * 24 * time.Hour),
						Path:      "memory/2026-03-08.md",
					},
				},
			},
			halfLife: 0, // Use default
			checkResults: func(t *testing.T, hits []*entity.SearchHit) {
				// Should use default half-life from calculator
				assert.Greater(t, hits[0].Score, 0.0)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			calc := search.NewTemporalDecayCalculator(halfLife)
			calc.Apply(tt.hits, tt.halfLife)
			tt.checkResults(t, tt.hits)
		})
	}
}

func TestTemporalDecayCalculator_CalculateDecayMultiplier(t *testing.T) {
	halfLife := 30 * 24 * time.Hour
	calc := search.NewTemporalDecayCalculator(halfLife)

	// Correct formula: multiplier = Exp(-Ln2 * ageDays / halfLifeDays)
	// For halfLife = 30 days:
	// - age = 0: multiplier = 1.0
	// - age = 30 days: multiplier ≈ 0.5 (half-life)
	// - age = 60 days: multiplier ≈ 0.25

	tests := []struct {
		name          string
		age           time.Duration
		halfLife      time.Duration
		minMultiplier float64
		maxMultiplier float64
	}{
		{
			name:          "zero age",
			age:           0,
			halfLife:      halfLife,
			minMultiplier: 0.99,
			maxMultiplier: 1.01,
		},
		{
			name:          "one day old",
			age:           24 * time.Hour,
			halfLife:      halfLife,
			minMultiplier: 0.97,
			maxMultiplier: 1.0,
		},
		{
			name:          "one week old",
			age:           7 * 24 * time.Hour,
			halfLife:      halfLife,
			minMultiplier: 0.80,
			maxMultiplier: 0.95,
		},
		{
			name:          "half life (30 days)",
			age:           halfLife,
			halfLife:      halfLife,
			minMultiplier: 0.45,
			maxMultiplier: 0.55,
		},
		{
			name:          "double half life (60 days)",
			age:           halfLife * 2,
			halfLife:      halfLife,
			minMultiplier: 0.20,
			maxMultiplier: 0.35,
		},
		{
			name:          "very old (365 days)",
			age:           365 * 24 * time.Hour,
			halfLife:      halfLife,
			minMultiplier: 0.0,
			maxMultiplier: 0.001,
		},
		{
			name:          "zero half life",
			age:           24 * time.Hour,
			halfLife:      0,
			minMultiplier: 1.0,
			maxMultiplier: 1.0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			multiplier := calc.CalculateDecayMultiplier(tt.age, tt.halfLife)
			assert.GreaterOrEqual(t, multiplier, tt.minMultiplier)
			assert.LessOrEqual(t, multiplier, tt.maxMultiplier)
		})
	}
}

func TestTemporalDecayCalculator_IsEvergreen(t *testing.T) {
	calc := search.NewTemporalDecayCalculator(30 * 24 * time.Hour)

	tests := []struct {
		name     string
		path     string
		expected bool
	}{
		{"MEMORY.md uppercase", "MEMORY.md", true},
		{"memory.md lowercase", "memory.md", true},
		{"daily log", "memory/2026-03-08.md", false},
		{"daily log different date", "memory/2025-12-31.md", false},
		{"other file", "notes/test.md", true},
		{"session file", "session/abc123.md", true},
		{"root file", "README.md", true},
		{"nested non-dated", "docs/guide.md", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := calc.IsEvergreen(tt.path)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestCalculateHalfLifeFromDays(t *testing.T) {
	tests := []struct {
		days     float64
		expected time.Duration
	}{
		{1.0, 24 * time.Hour},
		{7.0, 7 * 24 * time.Hour},
		{30.0, 30 * 24 * time.Hour},
		{0.5, 12 * time.Hour},
		{365.0, 365 * 24 * time.Hour},
	}

	for _, tt := range tests {
		t.Run("", func(t *testing.T) {
			result := search.CalculateHalfLifeFromDays(tt.days)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestLinearDecayCalculator(t *testing.T) {
	halfLife := 30 * 24 * time.Hour
	calc := search.NewLinearDecayCalculator(halfLife)

	t.Run("Apply", func(t *testing.T) {
		hits := []*entity.SearchHit{
			{
				Entry: &entity.Entry{
					ID:        "new",
					Score:     1.0,
					Timestamp: time.Now(),
					Path:      "memory/2026-03-08.md",
				},
			},
			{
				Entry: &entity.Entry{
					ID:        "old",
					Score:     1.0,
					Timestamp: time.Now().Add(-halfLife),
					Path:      "memory/2026-03-08.md",
				},
			},
		}

		calc.Apply(hits, halfLife)

		// New hit should have higher score than old hit
		assert.Greater(t, hits[0].Score, hits[1].Score)
	})

	t.Run("CalculateDecayMultiplier", func(t *testing.T) {
		tests := []struct {
			name          string
			age           time.Duration
			halfLife      time.Duration
			minMultiplier float64
			maxMultiplier float64
		}{
			{"zero age", 0, halfLife, 0.99, 1.01},
			{"half life", halfLife, halfLife, 0.49, 0.51},
			{"double half life (max)", halfLife * 2, halfLife, 0.0, 0.01},
			{"triple half life (max exceeded)", halfLife * 3, halfLife, 0.0, 0.0},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				multiplier := calc.CalculateDecayMultiplier(tt.age, tt.halfLife)
				assert.GreaterOrEqual(t, multiplier, tt.minMultiplier)
				assert.LessOrEqual(t, multiplier, tt.maxMultiplier)
			})
		}
	})

	t.Run("use default half-life", func(t *testing.T) {
		calc := search.NewLinearDecayCalculator(halfLife)
		hits := []*entity.SearchHit{
			{
				Entry: &entity.Entry{
					ID:        "test",
					Score:     1.0,
					Timestamp: time.Now().Add(-15 * 24 * time.Hour),
					Path:      "memory/2026-03-08.md",
				},
			},
		}

		// Pass 0 to use default
		calc.Apply(hits, 0)
		assert.Greater(t, hits[0].Score, 0.0)
	})
}

func TestStepDecayCalculator(t *testing.T) {
	steps := search.DefaultStepDecaySteps()
	calc := search.NewStepDecayCalculator(steps)

	require.NotNil(t, calc)
	assert.Len(t, calc.Steps, 5)

	t.Run("Apply", func(t *testing.T) {
		now := time.Now()
		hits := []*entity.SearchHit{
			{
				Entry: &entity.Entry{
					ID:        "today",
					Score:     1.0,
					Timestamp: now,
					Path:      "memory/2026-03-08.md",
				},
			},
			{
				Entry: &entity.Entry{
					ID:        "week-old",
					Score:     1.0,
					Timestamp: now.Add(-8 * 24 * time.Hour),
					Path:      "memory/2026-03-08.md",
				},
			},
			{
				Entry: &entity.Entry{
					ID:        "month-old",
					Score:     1.0,
					Timestamp: now.Add(-31 * 24 * time.Hour),
					Path:      "memory/2026-03-08.md",
				},
			},
		}

		calc.Apply(hits, 0) // halfLife not used in step decay

		// Today should have highest score
		assert.Equal(t, 1.0, hits[0].Score)
		// Week old should have 0.8 multiplier
		assert.Equal(t, 0.8, hits[1].Score)
		// Month old should have 0.6 multiplier
		assert.Equal(t, 0.6, hits[2].Score)
	})

	t.Run("CalculateDecayMultiplier", func(t *testing.T) {
		tests := []struct {
			name     string
			age      time.Duration
			expected float64
		}{
			{"today", 0, 1.0},
			{"3 days", 3 * 24 * time.Hour, 1.0},
			{"10 days", 10 * 24 * time.Hour, 0.8},
			{"45 days", 45 * 24 * time.Hour, 0.6},
			{"100 days", 100 * 24 * time.Hour, 0.4},
			{"200 days", 200 * 24 * time.Hour, 0.2},
			{"very old", 1000 * 24 * time.Hour, 0.2},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				multiplier := calc.CalculateDecayMultiplier(tt.age)
				assert.Equal(t, tt.expected, multiplier)
			})
		}
	})
}

func TestDefaultStepDecaySteps(t *testing.T) {
	steps := search.DefaultStepDecaySteps()

	assert.Len(t, steps, 5)
	assert.Equal(t, time.Duration(0), steps[0].AgeThreshold)
	assert.Equal(t, 1.0, steps[0].Multiplier)
	assert.Equal(t, 7*24*time.Hour, steps[1].AgeThreshold)
	assert.Equal(t, 0.8, steps[1].Multiplier)
}

// Benchmark tests
func BenchmarkTemporalDecayCalculator_Apply(b *testing.B) {
	calc := search.NewTemporalDecayCalculator(30 * 24 * time.Hour)
	halfLife := 30 * 24 * time.Hour

	// Create 100 hits
	hits := make([]*entity.SearchHit, 100)
	now := time.Now()
	for i := 0; i < 100; i++ {
		hits[i] = &entity.SearchHit{
			Entry: &entity.Entry{
				ID:        string(rune(i)),
				Score:     1.0,
				Timestamp: now.Add(-time.Duration(i) * 24 * time.Hour),
				Path:      "memory/2026-03-08.md",
			},
		}
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		calc.Apply(hits, halfLife)
	}
}

func BenchmarkTemporalDecayCalculator_CalculateDecayMultiplier(b *testing.B) {
	calc := search.NewTemporalDecayCalculator(30 * 24 * time.Hour)
	age := 15 * 24 * time.Hour
	halfLife := 30 * 24 * time.Hour

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		calc.CalculateDecayMultiplier(age, halfLife)
	}
}

func BenchmarkLinearDecayCalculator_Apply(b *testing.B) {
	calc := search.NewLinearDecayCalculator(30 * 24 * time.Hour)
	halfLife := 30 * 24 * time.Hour

	hits := make([]*entity.SearchHit, 100)
	now := time.Now()
	for i := 0; i < 100; i++ {
		hits[i] = &entity.SearchHit{
			Entry: &entity.Entry{
				ID:        string(rune(i)),
				Score:     1.0,
				Timestamp: now.Add(-time.Duration(i) * 24 * time.Hour),
				Path:      "memory/2026-03-08.md",
			},
		}
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		calc.Apply(hits, halfLife)
	}
}

func BenchmarkStepDecayCalculator_Apply(b *testing.B) {
	calc := search.NewStepDecayCalculator(search.DefaultStepDecaySteps())

	hits := make([]*entity.SearchHit, 100)
	now := time.Now()
	for i := 0; i < 100; i++ {
		hits[i] = &entity.SearchHit{
			Entry: &entity.Entry{
				ID:        string(rune(i)),
				Score:     1.0,
				Timestamp: now.Add(-time.Duration(i) * 24 * time.Hour),
				Path:      "memory/2026-03-08.md",
			},
		}
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		calc.Apply(hits, 0)
	}
}

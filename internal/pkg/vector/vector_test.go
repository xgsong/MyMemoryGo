package vector

import (
	"math"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCosineSimilarity(t *testing.T) {
	tests := []struct {
		name     string
		a        []float32
		b        []float32
		expected float64
	}{
		{
			name:     "identical vectors",
			a:        []float32{1.0, 0.0, 0.0},
			b:        []float32{1.0, 0.0, 0.0},
			expected: 1.0,
		},
		{
			name:     "orthogonal vectors",
			a:        []float32{1.0, 0.0, 0.0},
			b:        []float32{0.0, 1.0, 0.0},
			expected: 0.0,
		},
		{
			name:     "opposite vectors",
			a:        []float32{1.0, 0.0, 0.0},
			b:        []float32{-1.0, 0.0, 0.0},
			expected: -1.0,
		},
		{
			name:     "different lengths",
			a:        []float32{1.0, 0.0},
			b:        []float32{1.0, 0.0, 0.0},
			expected: 0.0,
		},
		{
			name:     "zero vectors",
			a:        []float32{0.0, 0.0, 0.0},
			b:        []float32{0.0, 0.0, 0.0},
			expected: 0.0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := CosineSimilarity(tt.a, tt.b)
			assert.InDelta(t, tt.expected, result, 1e-6)
		})
	}
}

func TestEuclideanDistance(t *testing.T) {
	tests := []struct {
		name     string
		a        []float32
		b        []float32
		expected float64
	}{
		{
			name:     "identical vectors",
			a:        []float32{1.0, 2.0, 3.0},
			b:        []float32{1.0, 2.0, 3.0},
			expected: 0.0,
		},
		{
			name:     "unit distance",
			a:        []float32{0.0, 0.0, 0.0},
			b:        []float32{1.0, 0.0, 0.0},
			expected: 1.0,
		},
		{
			name:     "different lengths",
			a:        []float32{1.0, 0.0},
			b:        []float32{1.0, 0.0, 0.0},
			expected: math.Inf(1),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := EuclideanDistance(tt.a, tt.b)
			if math.IsInf(tt.expected, 1) {
				assert.True(t, math.IsInf(result, 1))
			} else {
				assert.InDelta(t, tt.expected, result, 1e-6)
			}
		})
	}
}

func TestNormalize(t *testing.T) {
	tests := []struct {
		name     string
		vec      []float32
		expected []float32
	}{
		{
			name:     "unit vector",
			vec:      []float32{1.0, 0.0, 0.0},
			expected: []float32{1.0, 0.0, 0.0},
		},
		{
			name:     "scale to unit",
			vec:      []float32{3.0, 4.0},
			expected: []float32{0.6, 0.8},
		},
		{
			name:     "zero vector",
			vec:      []float32{0.0, 0.0, 0.0},
			expected: []float32{0.0, 0.0, 0.0},
		},
		{
			name:     "empty vector",
			vec:      []float32{},
			expected: []float32{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := Normalize(tt.vec)
			if len(tt.expected) == 0 {
				assert.Empty(t, result)
			} else {
				for i := range tt.expected {
					assert.InDelta(t, tt.expected[i], result[i], 1e-6)
				}
			}
		})
	}
}

func TestDotProduct(t *testing.T) {
	tests := []struct {
		name     string
		a        []float32
		b        []float32
		expected float64
	}{
		{
			name:     "orthogonal vectors",
			a:        []float32{1.0, 0.0},
			b:        []float32{0.0, 1.0},
			expected: 0.0,
		},
		{
			name:     "parallel vectors",
			a:        []float32{1.0, 2.0},
			b:        []float32{2.0, 4.0},
			expected: 10.0,
		},
		{
			name:     "different lengths",
			a:        []float32{1.0, 0.0},
			b:        []float32{1.0, 0.0, 0.0},
			expected: 0.0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := DotProduct(tt.a, tt.b)
			assert.InDelta(t, tt.expected, result, 1e-6)
		})
	}
}

func TestNorm(t *testing.T) {
	tests := []struct {
		name     string
		vec      []float32
		expected float64
	}{
		{
			name:     "unit vector",
			vec:      []float32{1.0, 0.0, 0.0},
			expected: 1.0,
		},
		{
			name:     "3-4-5 triangle",
			vec:      []float32{3.0, 4.0},
			expected: 5.0,
		},
		{
			name:     "zero vector",
			vec:      []float32{0.0, 0.0, 0.0},
			expected: 0.0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := Norm(tt.vec)
			assert.InDelta(t, tt.expected, result, 1e-6)
		})
	}
}

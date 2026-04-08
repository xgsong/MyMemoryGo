package vector

import (
	"math"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewTopKHeap(t *testing.T) {
	h := NewTopKHeap(5)
	assert.Equal(t, 0, h.Len())
	assert.Equal(t, 5, h.k)
	assert.False(t, h.Full())
	assert.Equal(t, 0.0, h.MinScore())
}

func TestNewTopKHeap_ZeroK(t *testing.T) {
	h := NewTopKHeap(0)
	assert.Equal(t, 1, h.k)
}

func TestTopKHeap_Push_BelowCapacity(t *testing.T) {
	h := NewTopKHeap(3)
	h.Add("a", 0.5)
	h.Add("b", 0.8)
	assert.Equal(t, 2, h.Len())
	assert.False(t, h.Full())
	assert.Equal(t, 0.5, h.MinScore())
}

func TestTopKHeap_Push_AtCapacity(t *testing.T) {
	h := NewTopKHeap(3)
	h.Add("a", 0.5)
	h.Add("b", 0.8)
	h.Add("c", 0.6)
	assert.Equal(t, 3, h.Len())
	assert.True(t, h.Full())
}

func TestTopKHeap_Push_ReplaceMinimum(t *testing.T) {
	h := NewTopKHeap(3)
	h.Add("a", 0.5)
	h.Add("b", 0.8)
	h.Add("c", 0.6)

	// Score 0.7 > min(0.5), should replace "a"
	h.Add("d", 0.7)
	assert.Equal(t, 3, h.Len())

	items := h.Items()
	assert.Equal(t, 3, len(items))
	// Should contain b(0.8), d(0.7), c(0.6)
	ids := make(map[string]bool)
	for _, item := range items {
		ids[item.ID] = true
	}
	assert.True(t, ids["b"])
	assert.True(t, ids["c"])
	assert.True(t, ids["d"])
	assert.False(t, ids["a"])
}

func TestTopKHeap_Push_ScoreBelowMinimum(t *testing.T) {
	h := NewTopKHeap(3)
	h.Add("a", 0.5)
	h.Add("b", 0.8)
	h.Add("c", 0.6)

	// Score 0.3 < min(0.5), should be ignored
	h.Add("d", 0.3)
	assert.Equal(t, 3, h.Len())

	items := h.Items()
	ids := make(map[string]bool)
	for _, item := range items {
		ids[item.ID] = true
	}
	assert.True(t, ids["a"])
	assert.True(t, ids["b"])
	assert.True(t, ids["c"])
	assert.False(t, ids["d"])
}

func TestTopKHeap_Items_SortedDescending(t *testing.T) {
	h := NewTopKHeap(5)
	h.Add("a", 0.3)
	h.Add("b", 0.9)
	h.Add("c", 0.5)
	h.Add("d", 0.7)
	h.Add("e", 0.1)

	items := h.Items()
	assert.Equal(t, 5, len(items))

	// Verify descending order
	for i := 1; i < len(items); i++ {
		assert.GreaterOrEqual(t, items[i-1].Score, items[i].Score)
	}

	assert.Equal(t, "b", items[0].ID)
	assert.Equal(t, 0.9, items[0].Score)
	assert.Equal(t, "e", items[4].ID)
	assert.Equal(t, 0.1, items[4].Score)
}

func TestTopKHeap_Items_OverCapacity(t *testing.T) {
	h := NewTopKHeap(3)
	h.Add("a", 0.3)
	h.Add("b", 0.9)
	h.Add("c", 0.5)
	h.Add("d", 0.7) // Replaces a (0.3)
	h.Add("e", 0.1) // Ignored (0.1 < min 0.5)
	h.Add("f", 0.8) // Replaces c (0.5)

	items := h.Items()
	assert.Equal(t, 3, len(items))

	// Top 3: b(0.9), f(0.8), d(0.7)
	assert.Equal(t, "b", items[0].ID)
	assert.Equal(t, "f", items[1].ID)
	assert.Equal(t, "d", items[2].ID)
}

func TestTopKHeap_MinScore(t *testing.T) {
	h := NewTopKHeap(3)
	assert.Equal(t, 0.0, h.MinScore())

	h.Add("a", 0.5)
	h.Add("b", 0.8)
	assert.Equal(t, 0.5, h.MinScore())

	h.Add("c", 0.6)
	assert.Equal(t, 0.5, h.MinScore())

	h.Add("d", 0.9)
	assert.Equal(t, 0.6, h.MinScore())
}

func TestTopKHeap_Empty(t *testing.T) {
	h := NewTopKHeap(5)
	assert.Equal(t, 0, h.Len())
	assert.False(t, h.Full())
	assert.Equal(t, 0.0, h.MinScore())
	assert.Empty(t, h.Items())
}

func TestTopKHeap_SingleItem(t *testing.T) {
	h := NewTopKHeap(5)
	h.Add("only", 0.42)
	assert.Equal(t, 1, h.Len())
	assert.False(t, h.Full())
	assert.Equal(t, 0.42, h.MinScore())

	items := h.Items()
	assert.Equal(t, 1, len(items))
	assert.Equal(t, "only", items[0].ID)
}

func TestTopKHeap_SameScores(t *testing.T) {
	h := NewTopKHeap(3)
	h.Add("a", 0.5)
	h.Add("b", 0.5)
	h.Add("c", 0.5)

	// Equal score should still be accepted (not > min, so ignored when full)
	h.Add("d", 0.5)
	assert.Equal(t, 3, h.Len())

	// Higher score replaces
	h.Add("e", 0.6)
	assert.Equal(t, 3, h.Len())
}

func TestTopKHeap_NegativeScores(t *testing.T) {
	h := NewTopKHeap(3)
	h.Add("a", -0.5)
	h.Add("b", -0.2)
	h.Add("c", -0.8)
	h.Add("d", -0.1) // Replaces c (-0.8)

	items := h.Items()
	assert.Equal(t, 3, len(items))
	assert.Equal(t, -0.1, items[0].Score)
	assert.Equal(t, -0.2, items[1].Score)
	assert.Equal(t, -0.5, items[2].Score)
}

func TestTopKHeap_LargeK(t *testing.T) {
	h := NewTopKHeap(1000)
	for i := 0; i < 5000; i++ {
		h.Add(string(rune('a'+i%26)), float64(i)/5000.0)
	}
	assert.Equal(t, 1000, h.Len())
	assert.True(t, h.Full())

	items := h.Items()
	// Top items should have highest scores
	assert.GreaterOrEqual(t, items[0].Score, items[len(items)-1].Score)
	assert.InDelta(t, 1.0, items[0].Score, 0.01)
}

func BenchmarkTopKHeap_Push(b *testing.B) {
	h := NewTopKHeap(10)
	scores := make([]float64, b.N)
	for i := range scores {
		scores[i] = float64(i) / float64(b.N)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		h.Add("id", scores[i])
	}
}

func BenchmarkTopKHeap_Items(b *testing.B) {
	h := NewTopKHeap(100)
	for i := 0; i < 100; i++ {
		h.Add(string(rune('A'+i%26))+string(rune('0'+i/26)), float64(i)/100.0)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		h.Items()
	}
}

func BenchmarkTopKHeap_LargeDataset(b *testing.B) {
	data := make([]float64, 10000)
	for i := range data {
		data[i] = math.Sin(float64(i) * 0.01)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		h := NewTopKHeap(20)
		for j, score := range data {
			h.Add(string(rune(j)), score)
		}
		_ = h.Items()
	}
}

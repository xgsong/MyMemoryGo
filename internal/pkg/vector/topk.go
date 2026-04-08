package vector

import "container/heap"

// TopKItem represents an item in a Top-K heap with an ID and score.
type TopKItem struct {
	ID    string
	Score float64
}

// TopKHeap maintains the top-K items by score using a min-heap.
// The root is the minimum score among the top-K, allowing efficient
// comparison with new candidates.
type TopKHeap struct {
	items []*TopKItem
	k     int
}

// NewTopKHeap creates a new Top-K heap with the given capacity.
func NewTopKHeap(k int) *TopKHeap {
	if k < 1 {
		k = 1
	}
	return &TopKHeap{
		items: make([]*TopKItem, 0, k),
		k:     k,
	}
}

// Add adds an item to the heap if it qualifies for the top-K.
// If the heap is not full, the item is always added.
// If the heap is full and the item's score exceeds the current minimum,
// the minimum item is replaced.
func (h *TopKHeap) Add(id string, score float64) {
	if h.k <= 0 {
		return
	}

	if len(h.items) < h.k {
		heap.Push(h, &TopKItem{ID: id, Score: score})
		return
	}

	if score > h.items[0].Score {
		h.items[0].ID = id
		h.items[0].Score = score
		heap.Fix(h, 0)
	}
}

// Items returns all items in the heap sorted by score in descending order.
func (h *TopKHeap) Items() []*TopKItem {
	sorted := make([]*TopKItem, len(h.items))
	copy(sorted, h.items)

	// Heap sort: max-heap then extract
	n := len(sorted)
	// Build max-heap
	for i := n/2 - 1; i >= 0; i-- {
		maxSiftDown(sorted, i, n)
	}
	// Extract elements
	result := make([]*TopKItem, 0, n)
	for i := n - 1; i >= 0; i-- {
		sorted[0], sorted[i] = sorted[i], sorted[0]
		maxSiftDown(sorted, 0, i)
		result = append(result, sorted[i])
	}

	return result
}

// MinScore returns the current minimum score in the top-K.
// Returns 0 if the heap is empty.
func (h *TopKHeap) MinScore() float64 {
	if len(h.items) == 0 {
		return 0
	}
	return h.items[0].Score
}

// Full returns whether the heap has reached its capacity.
func (h *TopKHeap) Full() bool {
	return len(h.items) >= h.k
}

// Len implements heap.Interface.
func (h *TopKHeap) Len() int { return len(h.items) }

// Less implements heap.Interface. Min-heap: smaller scores are "less".
func (h *TopKHeap) Less(i, j int) bool { return h.items[i].Score < h.items[j].Score }

// Swap implements heap.Interface.
func (h *TopKHeap) Swap(i, j int) { h.items[i], h.items[j] = h.items[j], h.items[i] }

// Push implements heap.Interface.
func (h *TopKHeap) Push(x interface{}) { h.items = append(h.items, x.(*TopKItem)) }

// Pop implements heap.Interface.
func (h *TopKHeap) Pop() interface{} {
	old := h.items
	n := len(old)
	item := old[n-1]
	h.items = old[:n-1]
	return item
}

// maxSiftDown implements max-heap sift-down for sorting.
func maxSiftDown(data []*TopKItem, lo, hi int) {
	root := lo
	for {
		child := 2*root + 1
		if child >= hi {
			break
		}
		if child+1 < hi && data[child].Score < data[child+1].Score {
			child++
		}
		if data[root].Score >= data[child].Score {
			return
		}
		data[root], data[child] = data[child], data[root]
		root = child
	}
}

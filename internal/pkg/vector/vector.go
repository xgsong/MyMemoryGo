package vector

import (
	"math"
)

func CosineSimilarity(a, b []float32) float64 {
	if len(a) != len(b) {
		return 0.0
	}

	var dot, normA, normB float64
	for i := range a {
		dot += float64(a[i]) * float64(b[i])
		normA += float64(a[i]) * float64(a[i])
		normB += float64(b[i]) * float64(b[i])
	}

	if normA == 0 || normB == 0 {
		return 0.0
	}

	return dot / (math.Sqrt(normA) * math.Sqrt(normB))
}

func EuclideanDistance(a, b []float32) float64 {
	if len(a) != len(b) {
		return math.Inf(1)
	}

	var sum float64
	for i := range a {
		diff := float64(a[i]) - float64(b[i])
		sum += diff * diff
	}

	return math.Sqrt(sum)
}

func Normalize(vec []float32) []float32 {
	if len(vec) == 0 {
		return vec
	}

	var norm float64
	for _, v := range vec {
		norm += float64(v) * float64(v)
	}
	norm = math.Sqrt(norm)

	if norm == 0 {
		return vec
	}

	result := make([]float32, len(vec))
	for i, v := range vec {
		result[i] = float32(float64(v) / norm)
	}

	return result
}

func DotProduct(a, b []float32) float64 {
	if len(a) != len(b) {
		return 0.0
	}

	var sum float64
	for i := range a {
		sum += float64(a[i]) * float64(b[i])
	}

	return sum
}

func Norm(vec []float32) float64 {
	var sum float64
	for _, v := range vec {
		sum += float64(v) * float64(v)
	}
	return math.Sqrt(sum)
}

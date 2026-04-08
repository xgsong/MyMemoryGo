// Package search provides temporal decay calculation for search results.
// This file implements exponential time decay for memory relevance scoring.
package search

import (
	"fmt"
	"math"
	"regexp"
	"time"

	"github.com/xgsong/MyMemoryGo/internal/domain/entity"
)

// TemporalDecayCalculator implements exponential temporal decay.
// Recent memories get higher scores, older memories naturally fade.
type TemporalDecayCalculator struct {
	// DefaultHalfLife is the default half-life for decay calculation.
	DefaultHalfLife time.Duration

	// EvergreenPaths are path patterns that should not decay.
	EvergreenPatterns []*regexp.Regexp
}

// NewTemporalDecayCalculator creates a new temporal decay calculator.
func NewTemporalDecayCalculator(halfLife time.Duration) *TemporalDecayCalculator {
	return &TemporalDecayCalculator{
		DefaultHalfLife: halfLife,
		EvergreenPatterns: []*regexp.Regexp{
			regexp.MustCompile(`^MEMORY\.md$`),
			regexp.MustCompile(`^memory\.md$`),
		},
	}
}

// Apply applies temporal decay to search hits.
// Score multiplier = e^(-λ * age_in_days)
// where λ = ln(2) / half_life_in_days
func (c *TemporalDecayCalculator) Apply(hits []*entity.SearchHit, halfLife time.Duration) {
	if halfLife == 0 {
		halfLife = c.DefaultHalfLife
	}

	for _, hit := range hits {
		// Skip evergreen memories
		if c.IsEvergreen(hit.Path) {
			continue
		}

		// Calculate age
		age := time.Since(hit.Timestamp)

		// Calculate decay multiplier
		multiplier := c.CalculateDecayMultiplier(age, halfLife)

		// Apply decay
		hit.Score *= multiplier
	}
}

// CalculateDecayMultiplier calculates the decay multiplier.
// Formula: e^(-λ * age_in_days)
// where λ = ln(2) / half_life_in_days
func (c *TemporalDecayCalculator) CalculateDecayMultiplier(age, halfLife time.Duration) float64 {
	if halfLife == 0 {
		return 1.0
	}

	// Calculate lambda (decay constant) per day
	lambda := math.Ln2 / (halfLife.Hours() / 24.0)

	// Calculate age in days
	ageDays := age.Hours() / 24.0

	// Calculate exponential decay
	multiplier := math.Exp(-lambda * ageDays)

	// Ensure non-negative
	if multiplier < 0 {
		multiplier = 0
	}

	return multiplier
}

// IsEvergreen checks if a memory path should not decay.
// Long-term memories (MEMORY.md) are considered evergreen.
func (c *TemporalDecayCalculator) IsEvergreen(path string) bool {
	// Check against evergreen patterns
	for _, pattern := range c.EvergreenPatterns {
		if pattern.MatchString(path) {
			return true
		}
	}

	// Check if path is not a dated daily log
	// Daily logs: memory/YYYY-MM-DD.md
	// Evergreen: all other files
	return !isDatedPath(path)
}

// isDatedPath checks if path matches the dated daily log pattern.
func isDatedPath(path string) bool {
	// Pattern: memory/YYYY-MM-DD.md
	pattern := `^memory/\d{4}-\d{2}-\d{2}\.md$`
	matched, _ := regexp.MatchString(pattern, path)
	return matched
}

// CalculateHalfLifeFromDays creates a half-life duration from days.
func CalculateHalfLifeFromDays(days float64) time.Duration {
	return time.Duration(days * 24 * float64(time.Hour))
}

// CalculateDecayExample demonstrates decay calculation.
func CalculateDecayExample() {
	// Create calculator with 30-day half-life
	calc := NewTemporalDecayCalculator(30 * 24 * time.Hour)

	// Example ages and their decay multipliers
	examples := []time.Duration{
		0,                    // Today
		24 * time.Hour,       // 1 day
		7 * 24 * time.Hour,   // 7 days
		30 * 24 * time.Hour,  // 30 days
		60 * 24 * time.Hour,  // 60 days
		90 * 24 * time.Hour,  // 90 days
		180 * 24 * time.Hour, // 180 days
	}

	for _, age := range examples {
		multiplier := calc.CalculateDecayMultiplier(age, 30*24*time.Hour)
		days := age.Hours() / 24.0
		percentage := multiplier * 100
		fmt.Printf("Age: %6.1f days -> Score: %5.1f%% (%.3f)\n", days, percentage, multiplier)
	}
}

// DecayStrategy defines different decay strategies.
type DecayStrategy string

const (
	// DecayStrategyExponential uses exponential decay (default).
	DecayStrategyExponential DecayStrategy = "exponential"

	// DecayStrategyLinear uses linear decay.
	DecayStrategyLinear DecayStrategy = "linear"

	// DecayStrategyStep uses step decay (sudden drop after threshold).
	DecayStrategyStep DecayStrategy = "step"
)

// LinearDecayCalculator implements linear temporal decay.
type LinearDecayCalculator struct {
	DefaultHalfLife time.Duration
}

// NewLinearDecayCalculator creates a new linear decay calculator.
func NewLinearDecayCalculator(halfLife time.Duration) *LinearDecayCalculator {
	return &LinearDecayCalculator{
		DefaultHalfLife: halfLife,
	}
}

// Apply applies linear decay to search hits.
func (c *LinearDecayCalculator) Apply(hits []*entity.SearchHit, halfLife time.Duration) {
	if halfLife == 0 {
		halfLife = c.DefaultHalfLife
	}

	for _, hit := range hits {
		age := time.Since(hit.Timestamp)
		multiplier := c.CalculateDecayMultiplier(age, halfLife)
		hit.Score *= multiplier
	}
}

// CalculateDecayMultiplier calculates linear decay.
// Formula: max(0, 1 - age / max_age)
func (c *LinearDecayCalculator) CalculateDecayMultiplier(age, halfLife time.Duration) float64 {
	// For linear decay, half-life means score is 0.5
	// So max_age = half_life * 2
	maxAge := halfLife * 2

	if age >= maxAge {
		return 0.0
	}

	decay := float64(age) / float64(maxAge)
	return 1.0 - decay
}

// StepDecayCalculator implements step-based decay.
type StepDecayCalculator struct {
	Steps []DecayStep
}

// DecayStep defines a step in step decay.
type DecayStep struct {
	// AgeThreshold is the age after which this multiplier applies.
	AgeThreshold time.Duration

	// Multiplier is the score multiplier for this step.
	Multiplier float64
}

// NewStepDecayCalculator creates a new step decay calculator.
func NewStepDecayCalculator(steps []DecayStep) *StepDecayCalculator {
	return &StepDecayCalculator{
		Steps: steps,
	}
}

// DefaultStepDecaySteps returns default step decay configuration.
func DefaultStepDecaySteps() []DecayStep {
	return []DecayStep{
		{AgeThreshold: 0, Multiplier: 1.0},                    // Today
		{AgeThreshold: 7 * 24 * time.Hour, Multiplier: 0.8},   // After 7 days
		{AgeThreshold: 30 * 24 * time.Hour, Multiplier: 0.6},  // After 30 days
		{AgeThreshold: 90 * 24 * time.Hour, Multiplier: 0.4},  // After 90 days
		{AgeThreshold: 180 * 24 * time.Hour, Multiplier: 0.2}, // After 180 days
	}
}

// Apply applies step decay to search hits.
func (c *StepDecayCalculator) Apply(hits []*entity.SearchHit, halfLife time.Duration) {
	for _, hit := range hits {
		age := time.Since(hit.Timestamp)
		multiplier := c.CalculateDecayMultiplier(age)
		hit.Score *= multiplier
	}
}

// CalculateDecayMultiplier calculates step decay.
func (c *StepDecayCalculator) CalculateDecayMultiplier(age time.Duration) float64 {
	// Find applicable step (latest threshold before age)
	multiplier := 1.0
	for _, step := range c.Steps {
		if age >= step.AgeThreshold {
			multiplier = step.Multiplier
		} else {
			break
		}
	}
	return multiplier
}

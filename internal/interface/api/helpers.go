// Package api provides REST API server implementation.
package api

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/xgsong/MyMemoryGo/internal/domain/entity"
)

// sourceFromString converts a string to SourceType.
func sourceFromString(s string) entity.SourceType {
	switch s {
	case "longterm":
		return entity.SourceLongTerm
	case "daily":
		return entity.SourceDaily
	case "session":
		return entity.SourceSession
	default:
		return entity.SourceDaily
	}
}

// sourcesToTypes converts a slice of strings to SourceType slice.
func sourcesToTypes(sources []string) []entity.SourceType {
	if len(sources) == 0 {
		return nil
	}
	result := make([]entity.SourceType, 0, len(sources))
	for _, s := range sources {
		result = append(result, sourceFromString(s))
	}
	return result
}

// parseIntParam parses an integer parameter with a default value.
func parseIntParam(s string, defaultValue int) int {
	if s == "" {
		return defaultValue
	}
	val, err := strconv.Atoi(s)
	if err != nil {
		return defaultValue
	}
	return val
}

// parseFloatParam parses a float parameter with a default value.
func parseFloatParam(s string, defaultValue float64) float64 {
	if s == "" {
		return defaultValue
	}
	val, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return defaultValue
	}
	return val
}

// respondJSON sends a JSON response.
func respondJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

// respondError sends an error response.
func respondError(w http.ResponseWriter, status int, message string) {
	respondJSON(w, status, map[string]string{
		"error": message,
	})
}

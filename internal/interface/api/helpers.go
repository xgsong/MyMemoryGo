// Package api provides REST API server implementation.
package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"

	"github.com/xgsong/MyMemoryGo/internal/domain/entity"
	"github.com/xgsong/MyMemoryGo/internal/pkg/errors"
)

// sourceFromString converts a string to SourceType.
// Returns zero value and no error for empty string (meaning "no filter").
func sourceFromString(s string) (entity.SourceType, error) {
	if s == "" {
		return "", nil
	}
	switch s {
	case "longterm":
		return entity.SourceLongTerm, nil
	case "daily":
		return entity.SourceDaily, nil
	case "session":
		return entity.SourceSession, nil
	default:
		return "", errors.New(errors.CodeInvalidInput, fmt.Sprintf("invalid source type: %s", s))
	}
}

// sourcesToTypes converts a slice of strings to SourceType slice.
func sourcesToTypes(sources []string) ([]entity.SourceType, error) {
	if len(sources) == 0 {
		return nil, nil
	}
	result := make([]entity.SourceType, 0, len(sources))
	for _, s := range sources {
		sourceType, err := sourceFromString(s)
		if err != nil {
			return nil, err
		}
		result = append(result, sourceType)
	}
	return result, nil
}

// parseIntParam parses an integer parameter with a default value.
// Returns error if the parameter is present but not a valid integer.
func parseIntParam(s string, defaultValue int) (int, error) {
	if s == "" {
		return defaultValue, nil
	}
	val, err := strconv.Atoi(s)
	if err != nil {
		return 0, errors.New(errors.CodeInvalidInput, fmt.Sprintf("invalid integer parameter: %s", s))
	}
	return val, nil
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

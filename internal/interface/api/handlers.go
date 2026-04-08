// Package api provides REST API server implementation.
package api

import (
	"encoding/json"
	"net/http"
	"net/url"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/xgsong/MyMemoryGo/internal/application/service"
)

// healthCheck handles GET /health.
func (s *Server) healthCheck(w http.ResponseWriter, r *http.Request) {
	respondJSON(w, http.StatusOK, map[string]string{
		"status": "ok",
		"time":   time.Now().UTC().Format(time.RFC3339),
	})
}

// readinessCheck handles GET /ready.
func (s *Server) readinessCheck(w http.ResponseWriter, r *http.Request) {
	respondJSON(w, http.StatusOK, map[string]interface{}{
		"status": "ready",
		"time":   time.Now().UTC().Format(time.RFC3339),
		"checks": map[string]string{
			"database": "ok",
			"embedder": "ok",
		},
	})
}

// storeMemoryRequest represents the request body for storing a memory.
type storeMemoryRequest struct {
	Content  string            `json:"content"`
	Path     string            `json:"path,omitempty"`
	Source   string            `json:"source,omitempty"`
	Metadata map[string]string `json:"metadata,omitempty"`
}

// storeMemory handles POST /api/v1/memories.
func (s *Server) storeMemory(w http.ResponseWriter, r *http.Request) {
	var req storeMemoryRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
		return
	}

	if req.Content == "" {
		respondError(w, http.StatusBadRequest, "content is required")
		return
	}

	source := sourceFromString(req.Source)

	appReq := &service.StoreMemoryRequest{
		Content:  req.Content,
		Path:     req.Path,
		Source:   source,
		Metadata: req.Metadata,
	}

	resp, err := s.memoryApp.StoreMemory(r.Context(), appReq)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to store memory: "+err.Error())
		return
	}

	respondJSON(w, http.StatusCreated, resp.Memory)
}

// getMemory handles GET /api/v1/memories/{id}.
func (s *Server) getMemory(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		respondError(w, http.StatusBadRequest, "memory ID is required")
		return
	}

	// Decode URL-encoded ID (chi doesn't auto-decode %2F)
	decodedID, err := url.QueryUnescape(id)
	if err != nil {
		decodedID = id
	}

	memory, err := s.memoryApp.GetMemory(r.Context(), decodedID)
	if err != nil {
		respondError(w, http.StatusNotFound, "memory not found: "+err.Error())
		return
	}

	respondJSON(w, http.StatusOK, memory)
}

// listMemoriesRequest represents query parameters for listing memories.
type listMemoriesRequest struct {
	Source string `json:"source,omitempty"`
	Limit  int    `json:"limit,omitempty"`
	Offset int    `json:"offset,omitempty"`
}

// listMemories handles GET /api/v1/memories.
func (s *Server) listMemories(w http.ResponseWriter, r *http.Request) {
	source := r.URL.Query().Get("source")
	limit := parseIntParam(r.URL.Query().Get("limit"), 100)
	offset := parseIntParam(r.URL.Query().Get("offset"), 0)

	req := &service.ListMemoriesRequest{
		Source: sourceFromString(source),
		Limit:  limit,
		Offset: offset,
	}

	resp, err := s.memoryApp.ListMemories(r.Context(), req)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to list memories: "+err.Error())
		return
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"memories": resp.Memories,
		"total":    resp.Total,
	})
}

// deleteMemory handles DELETE /api/v1/memories/{id}.
func (s *Server) deleteMemory(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		respondError(w, http.StatusBadRequest, "memory ID is required")
		return
	}

	// Decode URL-encoded ID (chi doesn't auto-decode %2F)
	decodedID, err := url.QueryUnescape(id)
	if err != nil {
		decodedID = id
	}

	if err := s.memoryApp.DeleteMemory(r.Context(), decodedID); err != nil {
		respondError(w, http.StatusInternalServerError, "failed to delete memory: "+err.Error())
		return
	}

	respondJSON(w, http.StatusOK, map[string]string{
		"message": "memory deleted",
		"id":      decodedID,
	})
}

// searchMemoriesRequest represents the request body for searching memories.
type searchMemoriesRequest struct {
	Query    string   `json:"query"`
	Limit    int      `json:"limit,omitempty"`
	Sources  []string `json:"sources,omitempty"`
	MinScore float64  `json:"min_score,omitempty"`
}

// searchMemories handles POST/GET /api/v1/search.
func (s *Server) searchMemories(w http.ResponseWriter, r *http.Request) {
	var req searchMemoriesRequest

	if r.Method == "POST" {
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			respondError(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
			return
		}
	} else {
		req.Query = r.URL.Query().Get("q")
		req.Limit = parseIntParam(r.URL.Query().Get("limit"), 10)
		req.MinScore = parseFloatParam(r.URL.Query().Get("min_score"), 0.0)
	}

	if req.Query == "" {
		respondError(w, http.StatusBadRequest, "query is required")
		return
	}

	appReq := &service.SearchMemoriesRequest{
		Query:    req.Query,
		Limit:    req.Limit,
		MinScore: req.MinScore,
	}

	resp, err := s.memoryApp.SearchMemories(r.Context(), appReq)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to search memories: "+err.Error())
		return
	}

	respondJSON(w, http.StatusOK, resp.Results)
}

// syncIndex handles POST /api/v1/sync.
func (s *Server) syncIndex(w http.ResponseWriter, r *http.Request) {
	if err := s.memoryApp.SyncIndex(r.Context()); err != nil {
		respondError(w, http.StatusInternalServerError, "failed to sync index: "+err.Error())
		return
	}

	respondJSON(w, http.StatusOK, map[string]string{
		"message": "index synchronized",
		"time":    time.Now().UTC().Format(time.RFC3339),
	})
}

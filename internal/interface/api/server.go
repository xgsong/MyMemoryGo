// Package api provides REST API server implementation.
package api

import (
	"context"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	slogchi "github.com/samber/slog-chi"
	"github.com/xgsong/MyMemoryGo/internal/application/service"
	"github.com/xgsong/MyMemoryGo/internal/pkg/log"
)

// Server represents the REST API server.
type Server struct {
	router     chi.Router
	memoryApp  *service.MemoryApplicationService
	httpServer *http.Server
}

// NewServer creates a new REST API server.
func NewServer(memoryApp *service.MemoryApplicationService) *Server {
	s := &Server{
		memoryApp: memoryApp,
		router:    chi.NewRouter(),
	}

	s.setupMiddleware()
	s.setupRoutes()

	return s
}

// setupMiddleware configures the middleware stack.
func (s *Server) setupMiddleware() {
	s.router.Use(middleware.RequestID)
	s.router.Use(middleware.RealIP)
	s.router.Use(slogchi.NewWithFilters(
		log.DefaultLogger(),
		slogchi.IgnorePath("/health", "/ready"),
	))
	s.router.Use(middleware.Recoverer)
	s.router.Use(middleware.Timeout(60 * time.Second))
	s.router.Use(middleware.AllowContentType("application/json"))
}

// setupRoutes configures the API routes.
func (s *Server) setupRoutes() {
	// Health check endpoints
	s.router.Get("/health", s.healthCheck)
	s.router.Get("/ready", s.readinessCheck)

	// API v1 routes
	s.router.Route("/api/v1", func(r chi.Router) {
		// Memory operations
		r.Post("/memories", s.storeMemory)
		r.Get("/memories/{id}", s.getMemory)
		r.Get("/memories", s.listMemories)
		r.Delete("/memories/{id}", s.deleteMemory)

		// Search
		r.Post("/search", s.searchMemories)
		r.Get("/search", s.searchMemories)

		// Sync
		r.Post("/sync", s.syncIndex)
	})
}

// Start starts the HTTP server.
func (s *Server) Start(addr string) error {
	s.httpServer = &http.Server{
		Addr:         addr,
		Handler:      s.router,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	return s.httpServer.ListenAndServe()
}

// Shutdown gracefully shuts down the server.
func (s *Server) Shutdown(ctx context.Context) error {
	if s.httpServer != nil {
		return s.httpServer.Shutdown(ctx)
	}
	return nil
}

// Handler returns the HTTP handler.
func (s *Server) Handler() http.Handler {
	return s.router
}

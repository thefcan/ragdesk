// Package server wires the HTTP router, middleware and dependencies.
package server

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
)

// Server holds the router and its backing dependencies.
type Server struct {
	router *chi.Mux
	db     *pgxpool.Pool
	rdb    *redis.Client
	log    *slog.Logger
}

// New constructs a Server with production middleware and routes registered.
func New(db *pgxpool.Pool, rdb *redis.Client, log *slog.Logger) *Server {
	s := &Server{
		router: chi.NewRouter(),
		db:     db,
		rdb:    rdb,
		log:    log,
	}
	s.routes()
	return s
}

func (s *Server) routes() {
	r := s.router
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Recoverer)
	r.Use(middleware.Timeout(15 * time.Second))
	r.Use(s.requestLogger)

	r.Get("/healthz", s.handleHealth)
	r.Get("/readyz", s.handleReady)
}

// Handler exposes the configured http.Handler.
func (s *Server) Handler() http.Handler {
	return s.router
}

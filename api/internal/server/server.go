// Package server wires the HTTP router, middleware and dependencies.
package server

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/redis/go-redis/v9"

	"github.com/thefcan/ragdesk/api/internal/auth"
	"github.com/thefcan/ragdesk/api/internal/store"
)

// Server holds the router and its backing dependencies.
type Server struct {
	router *chi.Mux
	store  *store.Store
	rdb    *redis.Client
	issuer *auth.Issuer
	log    *slog.Logger
}

// New constructs a Server with production middleware and routes registered.
func New(st *store.Store, rdb *redis.Client, iss *auth.Issuer, log *slog.Logger) *Server {
	s := &Server{
		router: chi.NewRouter(),
		store:  st,
		rdb:    rdb,
		issuer: iss,
		log:    log,
	}
	s.routes()
	return s
}

func (s *Server) routes() {
	r := s.router
	r.Use(middleware.RequestID)
	// middleware.RealIP is intentionally omitted: it trusts client-supplied
	// X-Forwarded-For / X-Real-IP headers and is spoofable (chi GHSA advisories).
	r.Use(middleware.Recoverer)
	r.Use(middleware.Timeout(15 * time.Second))
	r.Use(s.requestLogger)

	r.Get("/healthz", s.handleHealth)
	r.Get("/readyz", s.handleReady)
	r.Get("/version", s.handleVersion)

	r.Route("/auth", func(r chi.Router) {
		r.Post("/register", s.handleRegister)
		r.Post("/login", s.handleLogin)
	})

	r.Group(func(r chi.Router) {
		r.Use(s.requireAuth)
		r.Get("/workspaces", s.handleListWorkspaces)
		r.Post("/workspaces", s.handleCreateWorkspace)
		r.Get("/workspaces/{id}", s.handleGetWorkspace)
		r.Get("/workspaces/{id}/members", s.handleListMembers)
		r.Post("/workspaces/{id}/members", s.handleAddMember)
	})
}

// Handler exposes the configured http.Handler.
func (s *Server) Handler() http.Handler {
	return s.router
}

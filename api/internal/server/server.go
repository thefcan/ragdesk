// Package server wires the HTTP router, middleware and dependencies.
package server

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	"github.com/redis/go-redis/v9"

	"github.com/thefcan/ragdesk/api/internal/ai"
	"github.com/thefcan/ragdesk/api/internal/auth"
	"github.com/thefcan/ragdesk/api/internal/billing"
	"github.com/thefcan/ragdesk/api/internal/ingest"
	"github.com/thefcan/ragdesk/api/internal/store"
)

// Server holds the router and its backing dependencies.
type Server struct {
	router     *chi.Mux
	store      *store.Store
	rdb        *redis.Client
	issuer     *auth.Issuer
	ai         *ai.Client
	queue      *ingest.Queue
	billing    billing.Provider
	origins    []string
	webBaseURL string
	log        *slog.Logger
}

// New constructs a Server with production middleware and routes registered.
func New(st *store.Store, rdb *redis.Client, iss *auth.Issuer, aiClient *ai.Client, billingProvider billing.Provider, corsOrigins []string, webBaseURL string, log *slog.Logger) *Server {
	s := &Server{
		router:     chi.NewRouter(),
		store:      st,
		rdb:        rdb,
		issuer:     iss,
		ai:         aiClient,
		queue:      ingest.NewQueue(rdb),
		billing:    billingProvider,
		origins:    corsOrigins,
		webBaseURL: webBaseURL,
		log:        log,
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
	r.Use(middleware.Timeout(60 * time.Second))
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   s.origins,
		AllowedMethods:   []string{http.MethodGet, http.MethodPost, http.MethodPut, http.MethodDelete, http.MethodOptions},
		AllowedHeaders:   []string{"Authorization", "Content-Type"},
		AllowCredentials: false,
		MaxAge:           300,
	}))
	r.Use(s.requestLogger)

	r.Get("/healthz", s.handleHealth)
	r.Get("/readyz", s.handleReady)
	r.Get("/version", s.handleVersion)

	// Stripe webhook: public (Stripe signs the payload) and verified inside.
	r.Post("/billing/webhook", s.handleStripeWebhook)

	r.Route("/auth", func(r chi.Router) {
		r.Use(s.rateLimit(10, time.Minute))
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
		r.Get("/workspaces/{id}/documents", s.handleListDocuments)
		r.Post("/workspaces/{id}/documents", s.handleCreateDocument)
		r.Post("/workspaces/{id}/documents/{docId}/reingest", s.handleReingestDocument)
		r.With(s.userRateLimit(20, time.Minute)).Post("/workspaces/{id}/chat", s.handleChat)
		r.Get("/workspaces/{id}/billing", s.handleGetBilling)
		r.Post("/workspaces/{id}/billing/checkout", s.handleCheckout)
		r.Post("/workspaces/{id}/billing/dev-confirm", s.handleDevConfirm)
	})
}

// Handler exposes the configured http.Handler.
func (s *Server) Handler() http.Handler {
	return s.router
}

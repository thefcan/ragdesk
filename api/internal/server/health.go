package server

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5/middleware"
)

type healthResponse struct {
	Status  string            `json:"status"`
	Service string            `json:"service"`
	Checks  map[string]string `json:"checks,omitempty"`
}

// handleHealth is a liveness probe: it reports that the process is running.
func (s *Server) handleHealth(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, healthResponse{Status: "ok", Service: "ragdesk-api"})
}

// handleReady is a readiness probe: it verifies downstream dependencies.
func (s *Server) handleReady(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
	defer cancel()

	checks := map[string]string{}
	status := http.StatusOK

	if err := s.store.Ping(ctx); err != nil {
		checks["postgres"] = "down: " + err.Error()
		status = http.StatusServiceUnavailable
	} else {
		checks["postgres"] = "ok"
	}
	if err := s.rdb.Ping(ctx).Err(); err != nil {
		checks["redis"] = "down: " + err.Error()
		status = http.StatusServiceUnavailable
	} else {
		checks["redis"] = "ok"
	}

	state := "ok"
	if status != http.StatusOK {
		state = "degraded"
	}
	writeJSON(w, status, healthResponse{Status: state, Service: "ragdesk-api", Checks: checks})
}

// requestLogger emits one structured log line per request.
func (s *Server) requestLogger(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		ww := &statusWriter{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(ww, r)
		s.log.Info("request",
			slog.String("request_id", middleware.GetReqID(r.Context())),
			slog.String("method", r.Method),
			slog.String("path", r.URL.Path),
			slog.Int("status", ww.status),
			slog.Duration("duration", time.Since(start)),
		)
	})
}

type statusWriter struct {
	http.ResponseWriter
	status int
}

func (w *statusWriter) WriteHeader(code int) {
	w.status = code
	w.ResponseWriter.WriteHeader(code)
}

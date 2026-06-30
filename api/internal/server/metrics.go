package server

import (
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	httpRequestsTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "ragdesk_http_requests_total",
		Help: "Total HTTP requests, labelled by method, route pattern and status code.",
	}, []string{"method", "route", "status"})

	httpRequestDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "ragdesk_http_request_duration_seconds",
		Help:    "HTTP request latency in seconds, labelled by method and route pattern.",
		Buckets: prometheus.DefBuckets,
	}, []string{"method", "route"})
)

// metrics records a count and latency sample for every request, labelled by the
// chi route *pattern* (e.g. /workspaces/{id}/chat, not the concrete id) so the
// label cardinality stays bounded. It wraps the response writer with chi's
// WrapResponseWriter, which stays transparent to Flush — streaming endpoints
// (the chat SSE/NDJSON response) keep working.
func (s *Server) metrics(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		ww := middleware.NewWrapResponseWriter(w, r.ProtoMajor)
		next.ServeHTTP(ww, r)

		route := chi.RouteContext(r.Context()).RoutePattern()
		if route == "" {
			route = "unmatched"
		}
		status := ww.Status()
		if status == 0 {
			status = http.StatusOK
		}
		httpRequestsTotal.WithLabelValues(r.Method, route, strconv.Itoa(status)).Inc()
		httpRequestDuration.WithLabelValues(r.Method, route).Observe(time.Since(start).Seconds())
	})
}

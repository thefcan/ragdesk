package server

import (
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"strconv"
	"time"
)

// rateLimit returns middleware allowing up to `limit` requests per client IP
// within `window`, backed by Redis (INCR + EXPIRE). It fails open if Redis is
// unavailable, so a cache outage degrades to no limiting rather than an outage.
func (s *Server) rateLimit(limit int, window time.Duration) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			key := fmt.Sprintf("rl:%s:%s", r.URL.Path, clientIP(r))
			count, err := s.rdb.Incr(r.Context(), key).Result()
			if err != nil {
				s.log.Warn("rate limit unavailable", slog.Any("err", err))
				next.ServeHTTP(w, r)
				return
			}
			if count == 1 {
				s.rdb.Expire(r.Context(), key, window)
			}
			if count > int64(limit) {
				w.Header().Set("Retry-After", strconv.Itoa(int(window.Seconds())))
				writeError(w, http.StatusTooManyRequests, "too many requests, please slow down")
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

func clientIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

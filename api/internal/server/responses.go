package server

import (
	"encoding/json"
	"log/slog"
	"net/http"
)

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

// serverError logs the underlying error and returns a generic 500 to the client.
func (s *Server) serverError(w http.ResponseWriter, err error) {
	s.log.Error("internal error", slog.Any("err", err))
	writeError(w, http.StatusInternalServerError, "internal server error")
}

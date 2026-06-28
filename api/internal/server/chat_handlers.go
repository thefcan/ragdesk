package server

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"

	"github.com/thefcan/ragdesk/api/internal/store"
)

func (s *Server) handleChat(w http.ResponseWriter, r *http.Request) {
	userID := userIDFrom(r.Context())
	workspaceID := chi.URLParam(r, "id")

	// Tenant isolation: only members may query a workspace.
	if _, err := s.store.GetWorkspaceForUser(r.Context(), userID, workspaceID); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeError(w, http.StatusNotFound, "workspace not found")
			return
		}
		s.serverError(w, err)
		return
	}

	var req struct {
		Question string `json:"question"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	req.Question = strings.TrimSpace(req.Question)
	if req.Question == "" {
		writeError(w, http.StatusBadRequest, "question is required")
		return
	}

	w.Header().Set("Content-Type", "application/x-ndjson")
	if err := s.ai.Chat(r.Context(), workspaceID, req.Question, w); err != nil {
		s.log.Error("chat stream", slog.String("workspace_id", workspaceID), slog.Any("err", err))
	}
}

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

func (s *Server) handleCreateDocument(w http.ResponseWriter, r *http.Request) {
	userID := userIDFrom(r.Context())
	workspaceID := chi.URLParam(r, "id")

	var req struct {
		Title   string `json:"title"`
		Content string `json:"content"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	req.Title = strings.TrimSpace(req.Title)
	if req.Title == "" || strings.TrimSpace(req.Content) == "" {
		writeError(w, http.StatusBadRequest, "title and content are required")
		return
	}

	doc, err := s.store.CreateDocument(r.Context(), userID, workspaceID, req.Title, req.Content)
	if errors.Is(err, store.ErrNotFound) {
		writeError(w, http.StatusNotFound, "workspace not found")
		return
	}
	if err != nil {
		s.serverError(w, err)
		return
	}

	if err := s.queue.Enqueue(r.Context(), doc.ID); err != nil {
		// The document is persisted as pending; surface the failure in logs.
		s.log.Warn("enqueue ingestion", slog.String("document_id", doc.ID), slog.Any("err", err))
	}
	writeJSON(w, http.StatusCreated, doc)
}

func (s *Server) handleListDocuments(w http.ResponseWriter, r *http.Request) {
	docs, err := s.store.ListDocuments(r.Context(), userIDFrom(r.Context()), chi.URLParam(r, "id"))
	if errors.Is(err, store.ErrNotFound) {
		writeError(w, http.StatusNotFound, "workspace not found")
		return
	}
	if err != nil {
		s.serverError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"documents": docs})
}

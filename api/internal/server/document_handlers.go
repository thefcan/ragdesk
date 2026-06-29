package server

import (
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"

	"github.com/thefcan/ragdesk/api/internal/billing"
	"github.com/thefcan/ragdesk/api/internal/store"
)

// maxDocumentBytes caps a single document's text to bound ingestion cost.
const maxDocumentBytes = 1 << 20 // 1 MiB

func (s *Server) handleCreateDocument(w http.ResponseWriter, r *http.Request) {
	userID := userIDFrom(r.Context())
	workspaceID := chi.URLParam(r, "id")

	r.Body = http.MaxBytesReader(w, r.Body, maxDocumentBytes+64*1024)
	var req struct {
		Title   string `json:"title"`
		Content string `json:"content"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		var maxErr *http.MaxBytesError
		if errors.As(err, &maxErr) {
			writeError(w, http.StatusRequestEntityTooLarge, "document too large (max 1 MB)")
			return
		}
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	req.Title = strings.TrimSpace(req.Title)
	if req.Title == "" || strings.TrimSpace(req.Content) == "" {
		writeError(w, http.StatusBadRequest, "title and content are required")
		return
	}
	if len(req.Content) > maxDocumentBytes {
		writeError(w, http.StatusRequestEntityTooLarge, "document too large (max 1 MB)")
		return
	}

	// Plan enforcement: cap stored documents per the workspace's plan.
	ws, err := s.store.GetWorkspaceForUser(r.Context(), userID, workspaceID)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeError(w, http.StatusNotFound, "workspace not found")
			return
		}
		s.serverError(w, err)
		return
	}
	plan := billing.PlanByID(ws.Plan)
	if plan.MaxDocuments >= 0 {
		count, err := s.store.CountDocuments(r.Context(), workspaceID)
		if err != nil {
			s.serverError(w, err)
			return
		}
		if count >= plan.MaxDocuments {
			writeError(w, http.StatusPaymentRequired,
				fmt.Sprintf("document limit reached (%d) on the %s plan; upgrade to add more", plan.MaxDocuments, plan.Name))
			return
		}
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

func (s *Server) handleReingestDocument(w http.ResponseWriter, r *http.Request) {
	userID := userIDFrom(r.Context())
	workspaceID := chi.URLParam(r, "id")
	documentID := chi.URLParam(r, "docId")

	err := s.store.ReingestDocument(r.Context(), userID, workspaceID, documentID)
	if errors.Is(err, store.ErrNotFound) {
		writeError(w, http.StatusNotFound, "document not found")
		return
	}
	if err != nil {
		s.serverError(w, err)
		return
	}
	if err := s.queue.Enqueue(r.Context(), documentID); err != nil {
		s.log.Warn("enqueue reingestion", slog.String("document_id", documentID), slog.Any("err", err))
	}
	writeJSON(w, http.StatusAccepted, map[string]string{"status": "pending"})
}

package server

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/thefcan/ragdesk/api/internal/store"
)

func (s *Server) handleListWorkspaces(w http.ResponseWriter, r *http.Request) {
	ws, err := s.store.ListWorkspacesForUser(r.Context(), userIDFrom(r.Context()))
	if err != nil {
		s.serverError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"workspaces": ws})
}

func (s *Server) handleCreateWorkspace(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name string `json:"name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	req.Name = strings.TrimSpace(req.Name)
	if req.Name == "" {
		writeError(w, http.StatusBadRequest, "name is required")
		return
	}
	ws, err := s.store.CreateWorkspace(r.Context(), userIDFrom(r.Context()), req.Name, slugify(req.Name))
	if err != nil {
		s.serverError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, ws)
}

func (s *Server) handleGetWorkspace(w http.ResponseWriter, r *http.Request) {
	ws, err := s.store.GetWorkspaceForUser(r.Context(), userIDFrom(r.Context()), chi.URLParam(r, "id"))
	if errors.Is(err, store.ErrNotFound) {
		writeError(w, http.StatusNotFound, "workspace not found")
		return
	}
	if err != nil {
		s.serverError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, ws)
}

func (s *Server) handleListMembers(w http.ResponseWriter, r *http.Request) {
	members, err := s.store.ListMembers(r.Context(), userIDFrom(r.Context()), chi.URLParam(r, "id"))
	if errors.Is(err, store.ErrNotFound) {
		writeError(w, http.StatusNotFound, "workspace not found")
		return
	}
	if err != nil {
		s.serverError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"members": members})
}

func (s *Server) handleAddMember(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Email string `json:"email"`
		Role  string `json:"role"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	req.Email = strings.TrimSpace(strings.ToLower(req.Email))
	m, err := s.store.AddMemberByEmail(r.Context(), userIDFrom(r.Context()), chi.URLParam(r, "id"), req.Email, req.Role)
	switch {
	case errors.Is(err, store.ErrForbidden):
		writeError(w, http.StatusForbidden, "only owners and admins can add members")
	case errors.Is(err, store.ErrNotFound):
		writeError(w, http.StatusNotFound, "workspace or user not found")
	case err != nil:
		s.serverError(w, err)
	default:
		writeJSON(w, http.StatusCreated, m)
	}
}

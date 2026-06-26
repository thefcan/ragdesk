package server

import (
	"encoding/json"
	"errors"
	"net/http"
	"regexp"
	"strings"

	"github.com/thefcan/ragdesk/api/internal/auth"
	"github.com/thefcan/ragdesk/api/internal/store"
)

var emailRe = regexp.MustCompile(`^[^@\s]+@[^@\s]+\.[^@\s]+$`)
var nonSlug = regexp.MustCompile(`[^a-z0-9]+`)

type authResponse struct {
	Token     string           `json:"token"`
	User      store.User       `json:"user"`
	Workspace *store.Workspace `json:"workspace,omitempty"`
}

// handleRegister creates a user, bootstraps their first workspace and returns a JWT.
func (s *Server) handleRegister(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Email     string `json:"email"`
		Password  string `json:"password"`
		Workspace string `json:"workspace"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	req.Email = strings.TrimSpace(strings.ToLower(req.Email))
	if !emailRe.MatchString(req.Email) {
		writeError(w, http.StatusBadRequest, "invalid email")
		return
	}
	if len(req.Password) < 8 {
		writeError(w, http.StatusBadRequest, "password must be at least 8 characters")
		return
	}

	hash, err := auth.HashPassword(req.Password)
	if err != nil {
		s.serverError(w, err)
		return
	}
	user, err := s.store.CreateUser(r.Context(), req.Email, hash)
	if errors.Is(err, store.ErrConflict) {
		writeError(w, http.StatusConflict, "email already registered")
		return
	}
	if err != nil {
		s.serverError(w, err)
		return
	}

	name := strings.TrimSpace(req.Workspace)
	if name == "" {
		name = "My Workspace"
	}
	ws, err := s.store.CreateWorkspace(r.Context(), user.ID, name, slugify(name))
	if err != nil {
		s.serverError(w, err)
		return
	}

	token, err := s.issuer.Issue(user.ID)
	if err != nil {
		s.serverError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, authResponse{Token: token, User: user, Workspace: &ws})
}

// handleLogin verifies credentials and returns a JWT.
func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	req.Email = strings.TrimSpace(strings.ToLower(req.Email))

	user, err := s.store.GetUserByEmail(r.Context(), req.Email)
	if err != nil || !auth.CheckPassword(user.PasswordHash, req.Password) {
		// Identical response whether or not the account exists.
		writeError(w, http.StatusUnauthorized, "invalid credentials")
		return
	}
	token, err := s.issuer.Issue(user.ID)
	if err != nil {
		s.serverError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, authResponse{Token: token, User: user})
}

func slugify(name string) string {
	s := nonSlug.ReplaceAllString(strings.ToLower(strings.TrimSpace(name)), "-")
	s = strings.Trim(s, "-")
	if s == "" {
		s = "workspace"
	}
	return s
}

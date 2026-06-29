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

// maxQuestionBytes caps a chat question to bound prompt size and LLM cost.
const maxQuestionBytes = 4 << 10 // 4 KiB

func (s *Server) handleChat(w http.ResponseWriter, r *http.Request) {
	userID := userIDFrom(r.Context())
	workspaceID := chi.URLParam(r, "id")

	// Tenant isolation: only members may query a workspace.
	ws, err := s.store.GetWorkspaceForUser(r.Context(), userID, workspaceID)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeError(w, http.StatusNotFound, "workspace not found")
			return
		}
		s.serverError(w, err)
		return
	}

	// Plan enforcement: cap chat messages per billing period (metered usage).
	plan := billing.PlanByID(ws.Plan)
	period := usagePeriod()
	if plan.MaxChatPerMonth >= 0 {
		used, err := s.store.CurrentUsage(r.Context(), workspaceID, "chat_messages", period)
		if err != nil {
			s.serverError(w, err)
			return
		}
		if used >= plan.MaxChatPerMonth {
			writeError(w, http.StatusPaymentRequired,
				fmt.Sprintf("monthly chat limit reached (%d) on the %s plan; upgrade for more", plan.MaxChatPerMonth, plan.Name))
			return
		}
	}

	r.Body = http.MaxBytesReader(w, r.Body, maxQuestionBytes+1024)
	var req struct {
		Question string `json:"question"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		var maxErr *http.MaxBytesError
		if errors.As(err, &maxErr) {
			writeError(w, http.StatusRequestEntityTooLarge, "question too long (max 4 KB)")
			return
		}
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	req.Question = strings.TrimSpace(req.Question)
	if req.Question == "" {
		writeError(w, http.StatusBadRequest, "question is required")
		return
	}
	if len(req.Question) > maxQuestionBytes {
		writeError(w, http.StatusRequestEntityTooLarge, "question too long (max 4 KB)")
		return
	}

	// Meter the accepted message before answering. Counting here (rather than on
	// success) makes the limit robust to clients that disconnect mid-stream.
	if _, err := s.store.IncrementUsage(r.Context(), workspaceID, "chat_messages", period); err != nil {
		s.log.Warn("increment chat usage", slog.String("workspace_id", workspaceID), slog.Any("err", err))
	}

	w.Header().Set("Content-Type", "application/x-ndjson")
	if err := s.ai.Chat(r.Context(), workspaceID, req.Question, w); err != nil {
		s.log.Error("chat stream", slog.String("workspace_id", workspaceID), slog.Any("err", err))
	}
}

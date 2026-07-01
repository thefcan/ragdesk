package server

import (
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"

	"github.com/thefcan/ragdesk/api/internal/ai"
	"github.com/thefcan/ragdesk/api/internal/billing"
	"github.com/thefcan/ragdesk/api/internal/store"
)

const (
	// maxQuestionBytes caps a chat question to bound prompt size and LLM cost.
	maxQuestionBytes = 4 << 10 // 4 KiB
	// maxHistoryTurns / maxHistoryBytes bound how much prior conversation a
	// client can replay for follow-up context, so the prompt stays cheap.
	maxHistoryTurns = 10
	maxHistoryBytes = 12 << 10 // 12 KiB across all turns
)

// sanitizeHistory keeps only well-formed, recent conversation turns within the
// size budget. It drops unknown roles and empty content, keeps the most recent
// maxHistoryTurns, then trims the oldest turns until under maxHistoryBytes.
func sanitizeHistory(in []ai.ChatTurn) []ai.ChatTurn {
	cleaned := make([]ai.ChatTurn, 0, len(in))
	for _, t := range in {
		if t.Role != "user" && t.Role != "assistant" {
			continue
		}
		content := strings.TrimSpace(t.Content)
		if content == "" {
			continue
		}
		cleaned = append(cleaned, ai.ChatTurn{Role: t.Role, Content: content})
	}
	if len(cleaned) > maxHistoryTurns {
		cleaned = cleaned[len(cleaned)-maxHistoryTurns:]
	}
	total := 0
	for _, t := range cleaned {
		total += len(t.Content)
	}
	for total > maxHistoryBytes && len(cleaned) > 0 {
		total -= len(cleaned[0].Content)
		cleaned = cleaned[1:]
	}
	return cleaned
}

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

	r.Body = http.MaxBytesReader(w, r.Body, maxQuestionBytes+maxHistoryBytes+2048)
	var req struct {
		Question string        `json:"question"`
		History  []ai.ChatTurn `json:"history"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		var maxErr *http.MaxBytesError
		if errors.As(err, &maxErr) {
			writeError(w, http.StatusRequestEntityTooLarge, "message too long")
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
	if err := s.ai.Chat(r.Context(), workspaceID, req.Question, sanitizeHistory(req.History), w); err != nil {
		s.log.Error("chat stream", slog.String("workspace_id", workspaceID), slog.Any("err", err))
	}
}

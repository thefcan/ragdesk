package server

import (
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/thefcan/ragdesk/api/internal/billing"
	"github.com/thefcan/ragdesk/api/internal/store"
)

// usagePeriod is the current UTC month key used to bucket metered usage.
func usagePeriod() string { return time.Now().UTC().Format("2006-01") }

// handleGetBilling returns a workspace's plan, limits and current-period usage.
func (s *Server) handleGetBilling(w http.ResponseWriter, r *http.Request) {
	userID := userIDFrom(r.Context())
	workspaceID := chi.URLParam(r, "id")
	period := usagePeriod()

	b, err := s.store.WorkspaceBilling(r.Context(), userID, workspaceID, period)
	if errors.Is(err, store.ErrNotFound) {
		writeError(w, http.StatusNotFound, "workspace not found")
		return
	}
	if err != nil {
		s.serverError(w, err)
		return
	}
	plan := billing.PlanByID(b.Plan)
	writeJSON(w, http.StatusOK, map[string]any{
		"plan":            b.Plan,
		"status":          b.Status,
		"role":            b.Role,
		"period":          period,
		"usage":           map[string]int{"documents": b.Documents, "chat_messages": b.ChatUsed},
		"limits":          map[string]int{"documents": plan.MaxDocuments, "chat_messages": plan.MaxChatPerMonth},
		"plans":           billing.Catalog(),
		"billing_enabled": s.billing.Configured(),
	})
}

// handleCheckout starts an upgrade. With Stripe configured it returns a hosted
// checkout URL; otherwise it returns a dev-confirm URL (the $0 local path).
// Owner-only.
func (s *Server) handleCheckout(w http.ResponseWriter, r *http.Request) {
	ws, ok := s.requireOwner(w, r)
	if !ok {
		return
	}
	plan := s.decodePlan(w, r)
	if plan == "" {
		return
	}

	if !s.billing.Configured() {
		// $0 path: the web app simulates the payment screen, then calls dev-confirm.
		url := s.webBaseURL + "/workspaces/" + ws.ID + "/billing?checkout=dev&plan=" + plan
		writeJSON(w, http.StatusOK, map[string]string{"url": url, "mode": "dev"})
		return
	}

	email, _ := s.store.UserEmail(r.Context(), ws.OwnerID)
	customerID, _ := s.store.WorkspaceStripeCustomer(r.Context(), ws.ID)
	url, err := s.billing.Checkout(r.Context(), billing.CheckoutParams{
		WorkspaceID: ws.ID,
		PlanID:      plan,
		Email:       email,
		CustomerID:  customerID,
		SuccessURL:  s.webBaseURL + "/workspaces/" + ws.ID + "/billing?checkout=success",
		CancelURL:   s.webBaseURL + "/workspaces/" + ws.ID + "/billing?checkout=cancel",
	})
	if err != nil {
		s.serverError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"url": url, "mode": "stripe"})
}

// handleDevConfirm upgrades a workspace without a real payment. It exists only
// in dev mode (Stripe not configured) and is owner-only — it stands in for the
// webhook that would otherwise fulfil the subscription.
func (s *Server) handleDevConfirm(w http.ResponseWriter, r *http.Request) {
	if s.billing.Configured() {
		writeError(w, http.StatusNotFound, "not found")
		return
	}
	ws, ok := s.requireOwner(w, r)
	if !ok {
		return
	}
	plan := s.decodePlan(w, r)
	if plan == "" {
		return
	}
	if err := s.store.SetWorkspacePlanByID(r.Context(), ws.ID, plan, "active", "", ""); err != nil {
		s.serverError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"plan": plan, "status": "active"})
}

// handlePortal opens a Stripe billing-portal session so an owner can manage or
// cancel the subscription. Stripe-only and owner-only.
func (s *Server) handlePortal(w http.ResponseWriter, r *http.Request) {
	if !s.billing.Configured() {
		writeError(w, http.StatusNotFound, "not found")
		return
	}
	ws, ok := s.requireOwner(w, r)
	if !ok {
		return
	}
	customerID, err := s.store.WorkspaceStripeCustomer(r.Context(), ws.ID)
	if err != nil {
		s.serverError(w, err)
		return
	}
	if customerID == "" {
		writeError(w, http.StatusBadRequest, "no active subscription to manage")
		return
	}
	url, err := s.billing.Portal(r.Context(), customerID, s.webBaseURL+"/workspaces/"+ws.ID+"/billing")
	if err != nil {
		s.serverError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"url": url})
}

// handleDevCancel downgrades a workspace to Free without Stripe. It exists only
// in dev mode (the $0 path) and is owner-only — it stands in for a portal cancel.
func (s *Server) handleDevCancel(w http.ResponseWriter, r *http.Request) {
	if s.billing.Configured() {
		writeError(w, http.StatusNotFound, "not found")
		return
	}
	ws, ok := s.requireOwner(w, r)
	if !ok {
		return
	}
	if err := s.store.SetWorkspacePlanByID(r.Context(), ws.ID, billing.PlanFree, "canceled", "", ""); err != nil {
		s.serverError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"plan": billing.PlanFree, "status": "canceled"})
}

// handleStripeWebhook applies subscription changes from verified Stripe events.
// It is public (Stripe signs the payload) and only active when Stripe is wired up.
func (s *Server) handleStripeWebhook(w http.ResponseWriter, r *http.Request) {
	if !s.billing.Configured() {
		writeError(w, http.StatusNotFound, "not found")
		return
	}
	payload, err := io.ReadAll(http.MaxBytesReader(w, r.Body, 1<<20))
	if err != nil {
		writeError(w, http.StatusRequestEntityTooLarge, "payload too large")
		return
	}
	event, err := s.billing.ParseWebhook(payload, r.Header.Get("Stripe-Signature"))
	if err != nil {
		s.log.Warn("stripe webhook verification failed", slog.Any("err", err))
		writeError(w, http.StatusBadRequest, "invalid signature")
		return
	}

	// Idempotency: Stripe delivers at least once. Skip events already applied.
	if event.ID != "" {
		fresh, err := s.store.MarkWebhookProcessed(r.Context(), event.ID)
		if err != nil {
			s.log.Error("record webhook event", slog.Any("err", err))
			writeError(w, http.StatusInternalServerError, "internal server error")
			return
		}
		if !fresh {
			w.WriteHeader(http.StatusOK) // duplicate delivery, already handled
			return
		}
	}

	switch event.Type {
	case billing.EventSubscriptionActive:
		if event.WorkspaceID != "" {
			err = s.store.SetWorkspacePlanByID(r.Context(), event.WorkspaceID, event.Plan, event.Status, event.CustomerID, event.SubscriptionID)
		} else if event.CustomerID != "" {
			err = s.store.SetWorkspacePlanByCustomer(r.Context(), event.CustomerID, event.Plan, event.Status)
		}
	case billing.EventSubscriptionCanceled:
		if event.CustomerID != "" {
			err = s.store.SetWorkspacePlanByCustomer(r.Context(), event.CustomerID, billing.PlanFree, event.Status)
		}
	}
	if err != nil {
		// Log but still 200: a non-2xx makes Stripe retry, and a malformed id
		// will never succeed. The event is acknowledged.
		s.log.Error("apply stripe event", slog.String("type", string(event.Type)), slog.Any("err", err))
	}
	w.WriteHeader(http.StatusOK)
}

// requireOwner loads the workspace and verifies the caller is its owner.
func (s *Server) requireOwner(w http.ResponseWriter, r *http.Request) (store.Workspace, bool) {
	userID := userIDFrom(r.Context())
	workspaceID := chi.URLParam(r, "id")
	ws, err := s.store.GetWorkspaceForUser(r.Context(), userID, workspaceID)
	if errors.Is(err, store.ErrNotFound) {
		writeError(w, http.StatusNotFound, "workspace not found")
		return store.Workspace{}, false
	}
	if err != nil {
		s.serverError(w, err)
		return store.Workspace{}, false
	}
	if ws.Role != "owner" {
		writeError(w, http.StatusForbidden, "only the workspace owner can manage billing")
		return store.Workspace{}, false
	}
	return ws, true
}

// decodePlan reads the requested plan (defaulting to Pro) and rejects unknown or
// free-tier targets. Returns "" after writing an error.
func (s *Server) decodePlan(w http.ResponseWriter, r *http.Request) string {
	var req struct {
		Plan string `json:"plan"`
	}
	_ = json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096)).Decode(&req)
	if req.Plan == "" {
		req.Plan = billing.PlanPro
	}
	if req.Plan == billing.PlanFree {
		writeError(w, http.StatusBadRequest, "cannot checkout the free plan")
		return ""
	}
	if billing.PlanByID(req.Plan).ID != req.Plan {
		writeError(w, http.StatusBadRequest, "unknown plan")
		return ""
	}
	return req.Plan
}

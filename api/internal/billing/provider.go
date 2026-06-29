package billing

import (
	"context"
	"errors"
)

// ErrNotConfigured is returned by the no-op provider when a real Stripe
// operation is attempted without credentials.
var ErrNotConfigured = errors.New("billing: stripe is not configured")

// CheckoutParams describes a subscription checkout request.
type CheckoutParams struct {
	WorkspaceID string
	PlanID      string
	Email       string
	SuccessURL  string
	CancelURL   string
}

// EventType is the normalised outcome of a Stripe webhook.
type EventType string

const (
	EventSubscriptionActive   EventType = "subscription_active"
	EventSubscriptionCanceled EventType = "subscription_canceled"
	EventIgnored              EventType = "ignored"
)

// Event is a provider-agnostic billing event derived from a verified webhook.
// WorkspaceID is set for checkout completion; subscription lifecycle events
// instead carry CustomerID, which the store maps back to a workspace.
type Event struct {
	Type           EventType
	WorkspaceID    string
	CustomerID     string
	SubscriptionID string
	Plan           string
	Status         string
}

// Provider abstracts the payment backend so the app runs identically with or
// without Stripe configured. The latter (Noop) is the $0 local/CI path, where a
// signed dev-confirm endpoint stands in for hosted checkout.
type Provider interface {
	// Configured reports whether real payments are wired up.
	Configured() bool
	// Checkout starts a subscription and returns a redirect URL.
	Checkout(ctx context.Context, p CheckoutParams) (string, error)
	// ParseWebhook verifies a Stripe signature and normalises the payload.
	ParseWebhook(payload []byte, signature string) (Event, error)
}

// Noop is the provider used when Stripe credentials are absent.
type Noop struct{}

// Configured always reports false.
func (Noop) Configured() bool { return false }

// Checkout is never called in dev mode; it errors defensively.
func (Noop) Checkout(context.Context, CheckoutParams) (string, error) {
	return "", ErrNotConfigured
}

// ParseWebhook is never called in dev mode; it errors defensively.
func (Noop) ParseWebhook([]byte, string) (Event, error) {
	return Event{}, ErrNotConfigured
}

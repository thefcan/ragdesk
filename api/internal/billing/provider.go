package billing

import (
	"context"
	"errors"
)

// ErrNotConfigured is returned by the no-op provider when a real Stripe
// operation is attempted without credentials.
var ErrNotConfigured = errors.New("billing: stripe is not configured")

// CheckoutParams describes a subscription checkout request. CustomerID, when
// set, reuses an existing Stripe customer instead of creating a new one.
type CheckoutParams struct {
	WorkspaceID string
	PlanID      string
	Email       string
	CustomerID  string
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
// instead carry CustomerID, which the store maps back to a workspace. ID is the
// Stripe event id, used to make at-least-once delivery idempotent.
type Event struct {
	ID             string
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
	// Portal returns a customer billing-portal URL for self-serve management
	// (update card, cancel). returnURL is where the portal sends the user back.
	Portal(ctx context.Context, customerID, returnURL string) (string, error)
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

// Portal is never called in dev mode; it errors defensively.
func (Noop) Portal(context.Context, string, string) (string, error) {
	return "", ErrNotConfigured
}

// ParseWebhook is never called in dev mode; it errors defensively.
func (Noop) ParseWebhook([]byte, string) (Event, error) {
	return Event{}, ErrNotConfigured
}

package billing

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/stripe/stripe-go/v82"
	"github.com/stripe/stripe-go/v82/checkout/session"
	"github.com/stripe/stripe-go/v82/webhook"
)

// Stripe is the production payment provider: hosted Checkout for subscriptions
// and signature-verified webhooks for fulfilment.
type Stripe struct {
	prices        map[string]string // plan id -> Stripe price id
	webhookSecret string
}

// NewStripe sets the global Stripe key and returns a provider. pricePro is the
// recurring Price id for the Pro plan; webhookSecret verifies inbound events.
func NewStripe(secretKey, pricePro, webhookSecret string) *Stripe {
	stripe.Key = secretKey
	return &Stripe{
		prices:        map[string]string{PlanPro: pricePro},
		webhookSecret: webhookSecret,
	}
}

// Configured reports that real payments are wired up.
func (s *Stripe) Configured() bool { return true }

// Checkout creates a hosted Checkout Session and returns its URL. The workspace
// id rides along as client_reference_id so the webhook can fulfil the upgrade.
func (s *Stripe) Checkout(ctx context.Context, p CheckoutParams) (string, error) {
	price := s.prices[p.PlanID]
	if price == "" {
		return "", fmt.Errorf("billing: no Stripe price configured for plan %q", p.PlanID)
	}
	params := &stripe.CheckoutSessionParams{
		Mode:              stripe.String(string(stripe.CheckoutSessionModeSubscription)),
		ClientReferenceID: stripe.String(p.WorkspaceID),
		SuccessURL:        stripe.String(p.SuccessURL),
		CancelURL:         stripe.String(p.CancelURL),
		LineItems: []*stripe.CheckoutSessionLineItemParams{
			{Price: stripe.String(price), Quantity: stripe.Int64(1)},
		},
	}
	if p.Email != "" {
		params.CustomerEmail = stripe.String(p.Email)
	}
	params.Context = ctx
	sess, err := session.New(params)
	if err != nil {
		return "", err
	}
	return sess.URL, nil
}

// ParseWebhook verifies the Stripe-Signature and normalises the event. Unknown
// event types resolve to EventIgnored so the handler can 200 them safely.
//
// IgnoreAPIVersionMismatch is set deliberately: the signing secret still fully
// authenticates the payload, but a Stripe account's API version is independent
// of this library's pinned version, so enforcing a match would reject otherwise
// valid production webhooks.
func (s *Stripe) ParseWebhook(payload []byte, signature string) (Event, error) {
	event, err := webhook.ConstructEventWithOptions(payload, signature, s.webhookSecret,
		webhook.ConstructEventOptions{IgnoreAPIVersionMismatch: true})
	if err != nil {
		return Event{}, err
	}
	switch event.Type {
	case "checkout.session.completed":
		var sess stripe.CheckoutSession
		if err := json.Unmarshal(event.Data.Raw, &sess); err != nil {
			return Event{}, err
		}
		ev := Event{
			Type:        EventSubscriptionActive,
			WorkspaceID: sess.ClientReferenceID,
			Plan:        PlanPro,
			Status:      "active",
		}
		if sess.Customer != nil {
			ev.CustomerID = sess.Customer.ID
		}
		if sess.Subscription != nil {
			ev.SubscriptionID = sess.Subscription.ID
		}
		return ev, nil
	case "customer.subscription.updated", "customer.subscription.deleted":
		var sub stripe.Subscription
		if err := json.Unmarshal(event.Data.Raw, &sub); err != nil {
			return Event{}, err
		}
		ev := Event{SubscriptionID: sub.ID, Status: string(sub.Status)}
		if sub.Customer != nil {
			ev.CustomerID = sub.Customer.ID
		}
		// Active/trialing keep Pro; anything else (past_due, unpaid, canceled,
		// deleted) downgrades to Free.
		switch sub.Status {
		case stripe.SubscriptionStatusActive, stripe.SubscriptionStatusTrialing:
			ev.Type, ev.Plan = EventSubscriptionActive, PlanPro
		default:
			ev.Type, ev.Plan = EventSubscriptionCanceled, PlanFree
		}
		return ev, nil
	default:
		return Event{Type: EventIgnored}, nil
	}
}

package billing

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"testing"
	"time"
)

// sign builds a valid Stripe-Signature header for payload using secret, matching
// Stripe's scheme: HMAC-SHA256 over "<timestamp>.<payload>".
func sign(payload []byte, secret string) string {
	ts := time.Now().Unix()
	mac := hmac.New(sha256.New, []byte(secret))
	fmt.Fprintf(mac, "%d.%s", ts, payload)
	return fmt.Sprintf("t=%d,v1=%s", ts, hex.EncodeToString(mac.Sum(nil)))
}

func TestStripeParseWebhookCheckoutCompleted(t *testing.T) {
	const secret = "whsec_test"
	p := NewStripe("sk_test_x", "price_pro", secret)
	payload := []byte(`{"id":"evt_1","object":"event","type":"checkout.session.completed",` +
		`"data":{"object":{"id":"cs_1","object":"checkout.session",` +
		`"client_reference_id":"ws-123","customer":"cus_9","subscription":"sub_9"}}}`)

	ev, err := p.ParseWebhook(payload, sign(payload, secret))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if ev.Type != EventSubscriptionActive {
		t.Fatalf("type = %q, want %q", ev.Type, EventSubscriptionActive)
	}
	if ev.WorkspaceID != "ws-123" || ev.CustomerID != "cus_9" || ev.SubscriptionID != "sub_9" {
		t.Fatalf("ids = %+v", ev)
	}
	if ev.Plan != PlanPro {
		t.Fatalf("plan = %q, want pro", ev.Plan)
	}
}

func TestStripeParseWebhookSubscriptionDeleted(t *testing.T) {
	const secret = "whsec_test"
	p := NewStripe("sk_test_x", "price_pro", secret)
	payload := []byte(`{"type":"customer.subscription.deleted",` +
		`"data":{"object":{"id":"sub_9","object":"subscription","customer":"cus_9","status":"canceled"}}}`)

	ev, err := p.ParseWebhook(payload, sign(payload, secret))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if ev.Type != EventSubscriptionCanceled || ev.Plan != PlanFree || ev.CustomerID != "cus_9" {
		t.Fatalf("event = %+v", ev)
	}
}

func TestStripeParseWebhookRejectsForgedSignature(t *testing.T) {
	payload := []byte(`{"type":"checkout.session.completed","data":{"object":{}}}`)
	// Signed with one secret, verified with another: must fail.
	signed := sign(payload, "whsec_real")
	p := NewStripe("sk_test_x", "price_pro", "whsec_different")
	if _, err := p.ParseWebhook(payload, signed); err == nil {
		t.Fatal("expected signature verification to reject a forged event")
	}
}

func TestStripeParseWebhookIgnoresUnknownEvents(t *testing.T) {
	const secret = "whsec_test"
	p := NewStripe("sk_test_x", "price_pro", secret)
	payload := []byte(`{"type":"invoice.paid","data":{"object":{}}}`)
	ev, err := p.ParseWebhook(payload, sign(payload, secret))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if ev.Type != EventIgnored {
		t.Fatalf("type = %q, want ignored", ev.Type)
	}
}

func TestNoopProvider(t *testing.T) {
	var p Provider = Noop{}
	if p.Configured() {
		t.Fatal("noop provider must report not configured")
	}
	if _, err := p.Checkout(context.Background(), CheckoutParams{}); err == nil {
		t.Fatal("noop Checkout should error")
	}
	if _, err := p.ParseWebhook(nil, ""); err == nil {
		t.Fatal("noop ParseWebhook should error")
	}
}

func TestPlanLookup(t *testing.T) {
	if PlanByID("does-not-exist").ID != PlanFree {
		t.Fatal("unknown plan must fall back to free")
	}
	if PlanByID(PlanPro).MaxDocuments != 1000 {
		t.Fatalf("pro MaxDocuments = %d, want 1000", PlanByID(PlanPro).MaxDocuments)
	}
	if len(Catalog()) != 2 {
		t.Fatalf("catalog size = %d, want 2", len(Catalog()))
	}
}

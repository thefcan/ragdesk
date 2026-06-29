package store_test

import (
	"context"
	"errors"
	"testing"

	"github.com/thefcan/ragdesk/api/internal/store"
)

func TestUsageMeteringAndPlan(t *testing.T) {
	st := newTestStore(t)
	ctx := context.Background()

	alice, _ := mustUser(t, st, "alice@example.com")
	ws, err := st.CreateWorkspace(ctx, alice.ID, "WS", "ws")
	if err != nil {
		t.Fatalf("workspace: %v", err)
	}
	if ws.Plan != "free" || ws.SubscriptionStatus != "active" {
		t.Fatalf("new workspace plan/status = %q/%q, want free/active", ws.Plan, ws.SubscriptionStatus)
	}
	const period = "2026-06"

	// Metering: three increments land at 3; CurrentUsage agrees.
	for want := 1; want <= 3; want++ {
		got, err := st.IncrementUsage(ctx, ws.ID, "chat_messages", period)
		if err != nil {
			t.Fatalf("increment: %v", err)
		}
		if got != want {
			t.Fatalf("increment %d returned %d", want, got)
		}
	}
	if n, _ := st.CurrentUsage(ctx, ws.ID, "chat_messages", period); n != 3 {
		t.Fatalf("CurrentUsage = %d, want 3", n)
	}
	// A different period is independent.
	if n, _ := st.CurrentUsage(ctx, ws.ID, "chat_messages", "2026-07"); n != 0 {
		t.Fatalf("other period usage = %d, want 0", n)
	}

	// Documents are counted for the document limit.
	for i := 0; i < 2; i++ {
		if _, err := st.CreateDocument(ctx, alice.ID, ws.ID, "Doc", "body", -1); err != nil {
			t.Fatalf("doc: %v", err)
		}
	}
	if n, _ := st.CountDocuments(ctx, ws.ID); n != 2 {
		t.Fatalf("CountDocuments = %d, want 2", n)
	}

	// WorkspaceBilling snapshots plan + usage + role, membership enforced.
	b, err := st.WorkspaceBilling(ctx, alice.ID, ws.ID, period)
	if err != nil {
		t.Fatalf("billing: %v", err)
	}
	if b.Plan != "free" || b.Documents != 2 || b.ChatUsed != 3 || b.Role != "owner" {
		t.Fatalf("billing snapshot = %+v", b)
	}

	// Tenant isolation: a non-member cannot read billing.
	bob, _ := mustUser(t, st, "bob@example.com")
	if _, err := st.WorkspaceBilling(ctx, bob.ID, ws.ID, period); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("foreign billing err = %v, want ErrNotFound", err)
	}

	// Upgrade by id, recording the Stripe customer; verify it sticks.
	if err := st.SetWorkspacePlanByID(ctx, ws.ID, "pro", "active", "cus_123", "sub_123"); err != nil {
		t.Fatalf("set plan: %v", err)
	}
	if b, _ := st.WorkspaceBilling(ctx, alice.ID, ws.ID, period); b.Plan != "pro" {
		t.Fatalf("plan after upgrade = %q, want pro", b.Plan)
	}

	// Downgrade keyed by Stripe customer (the subscription-webhook path).
	if err := st.SetWorkspacePlanByCustomer(ctx, "cus_123", "free", "canceled"); err != nil {
		t.Fatalf("set plan by customer: %v", err)
	}
	got, _ := st.WorkspaceBilling(ctx, alice.ID, ws.ID, period)
	if got.Plan != "free" || got.Status != "canceled" {
		t.Fatalf("plan/status after cancel = %q/%q, want free/canceled", got.Plan, got.Status)
	}
}

func TestDocumentCapEnforcedAtomically(t *testing.T) {
	st := newTestStore(t)
	ctx := context.Background()
	alice, _ := mustUser(t, st, "alice@example.com")
	ws, err := st.CreateWorkspace(ctx, alice.ID, "WS", "ws")
	if err != nil {
		t.Fatalf("workspace: %v", err)
	}
	// Cap of 2: two inserts succeed, the third is refused inside the insert.
	for i := 0; i < 2; i++ {
		if _, err := st.CreateDocument(ctx, alice.ID, ws.ID, "Doc", "body", 2); err != nil {
			t.Fatalf("insert %d: %v", i, err)
		}
	}
	if _, err := st.CreateDocument(ctx, alice.ID, ws.ID, "Doc", "body", 2); !errors.Is(err, store.ErrLimitReached) {
		t.Fatalf("over-cap insert err = %v, want ErrLimitReached", err)
	}
	// Unlimited (-1) always inserts.
	if _, err := st.CreateDocument(ctx, alice.ID, ws.ID, "Doc", "body", -1); err != nil {
		t.Fatalf("unlimited insert: %v", err)
	}
}

func TestWebhookIdempotencyAndCustomerLookup(t *testing.T) {
	st := newTestStore(t)
	ctx := context.Background()
	alice, _ := mustUser(t, st, "alice@example.com")
	ws, err := st.CreateWorkspace(ctx, alice.ID, "WS", "ws")
	if err != nil {
		t.Fatalf("workspace: %v", err)
	}

	// First delivery of an event id is fresh; a duplicate is not.
	if fresh, err := st.MarkWebhookProcessed(ctx, "evt_1"); err != nil || !fresh {
		t.Fatalf("first MarkWebhookProcessed = (%v, %v), want (true, nil)", fresh, err)
	}
	if fresh, _ := st.MarkWebhookProcessed(ctx, "evt_1"); fresh {
		t.Fatal("duplicate event reported as fresh")
	}

	// Customer id is empty until recorded, then returned.
	if c, err := st.WorkspaceStripeCustomer(ctx, ws.ID); err != nil || c != "" {
		t.Fatalf("initial customer = (%q, %v), want empty", c, err)
	}
	if err := st.SetWorkspacePlanByID(ctx, ws.ID, "pro", "active", "cus_42", "sub_42"); err != nil {
		t.Fatalf("set plan: %v", err)
	}
	if c, _ := st.WorkspaceStripeCustomer(ctx, ws.ID); c != "cus_42" {
		t.Fatalf("customer = %q, want cus_42", c)
	}
}

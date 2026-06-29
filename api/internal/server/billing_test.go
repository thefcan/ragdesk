package server_test

import (
	"context"
	"net/http"
	"testing"
	"time"
)

// registerOwner creates an account and returns its token and bootstrapped
// workspace id.
func registerOwner(t *testing.T, h http.Handler, email string) (token, wsID string) {
	t.Helper()
	_, body := doJSON(t, h, http.MethodPost, "/auth/register", "", map[string]any{
		"email": email, "password": "supersecret",
	})
	token, _ = body["token"].(string)
	ws, _ := body["workspace"].(map[string]any)
	wsID, _ = ws["id"].(string)
	if token == "" || wsID == "" {
		t.Fatalf("register did not return token/workspace: %v", body)
	}
	return token, wsID
}

func TestBillingSnapshotAndDevUpgrade(t *testing.T) {
	h := newTestServer(t, "http://unused")
	token, wsID := registerOwner(t, h, "alice@example.com")

	// A fresh workspace is on the Free plan with its documented limits.
	code, body := doJSON(t, h, http.MethodGet, "/workspaces/"+wsID+"/billing", token, nil)
	if code != http.StatusOK {
		t.Fatalf("billing status = %d", code)
	}
	if body["plan"] != "free" {
		t.Fatalf("plan = %v, want free", body["plan"])
	}
	limits, _ := body["limits"].(map[string]any)
	if limits["documents"].(float64) != 25 || limits["chat_messages"].(float64) != 100 {
		t.Fatalf("free limits = %v", limits)
	}

	// Checkout in dev mode returns a dev-confirm URL, not a Stripe URL.
	code, body = doJSON(t, h, http.MethodPost, "/workspaces/"+wsID+"/billing/checkout", token, map[string]any{"plan": "pro"})
	if code != http.StatusOK || body["mode"] != "dev" {
		t.Fatalf("checkout = %d %v, want 200/dev", code, body)
	}

	// Dev-confirm upgrades the workspace to Pro.
	if code, _ := doJSON(t, h, http.MethodPost, "/workspaces/"+wsID+"/billing/dev-confirm", token, map[string]any{"plan": "pro"}); code != http.StatusOK {
		t.Fatalf("dev-confirm status = %d, want 200", code)
	}
	code, body = doJSON(t, h, http.MethodGet, "/workspaces/"+wsID+"/billing", token, nil)
	if code != http.StatusOK || body["plan"] != "pro" {
		t.Fatalf("plan after upgrade = %v (status %d)", body["plan"], code)
	}
	limits, _ = body["limits"].(map[string]any)
	if limits["documents"].(float64) != 1000 {
		t.Fatalf("pro document limit = %v, want 1000", limits["documents"])
	}
}

func TestDocumentPlanLimit(t *testing.T) {
	h, pool := newTestServerAndPool(t, "http://unused")
	token, wsID := registerOwner(t, h, "alice@example.com")

	// Seed the workspace to the Free document limit (25).
	if _, err := pool.Exec(context.Background(),
		`INSERT INTO documents (workspace_id, title, source_text, status)
		 SELECT $1, 'seed-'||g, 'x', 'ready' FROM generate_series(1, 25) g`, wsID); err != nil {
		t.Fatalf("seed docs: %v", err)
	}

	// The 26th upload is rejected with 402 Payment Required.
	code, _ := doJSON(t, h, http.MethodPost, "/workspaces/"+wsID+"/documents", token,
		map[string]any{"title": "one too many", "content": "hello"})
	if code != http.StatusPaymentRequired {
		t.Fatalf("over-limit upload status = %d, want 402", code)
	}
}

func TestChatPlanLimit(t *testing.T) {
	h, pool := newTestServerAndPool(t, "http://unused")
	token, wsID := registerOwner(t, h, "alice@example.com")

	// Seed chat usage to the Free monthly limit (100) for the current period.
	period := time.Now().UTC().Format("2006-01")
	if _, err := pool.Exec(context.Background(),
		`INSERT INTO usage_counters (workspace_id, period, metric, count) VALUES ($1, $2, 'chat_messages', 100)`,
		wsID, period); err != nil {
		t.Fatalf("seed usage: %v", err)
	}

	// The next chat is rejected with 402 before any LLM work happens.
	code, _ := doJSON(t, h, http.MethodPost, "/workspaces/"+wsID+"/chat", token,
		map[string]any{"question": "hello"})
	if code != http.StatusPaymentRequired {
		t.Fatalf("over-limit chat status = %d, want 402", code)
	}
}

func TestDevCancelDowngrades(t *testing.T) {
	h := newTestServer(t, "http://unused")
	token, wsID := registerOwner(t, h, "alice@example.com")

	// Upgrade, then cancel via the dev path (Stripe-less portal stand-in).
	if code, _ := doJSON(t, h, http.MethodPost, "/workspaces/"+wsID+"/billing/dev-confirm", token, map[string]any{"plan": "pro"}); code != http.StatusOK {
		t.Fatalf("dev-confirm status = %d, want 200", code)
	}
	if code, _ := doJSON(t, h, http.MethodPost, "/workspaces/"+wsID+"/billing/dev-cancel", token, nil); code != http.StatusOK {
		t.Fatalf("dev-cancel status = %d, want 200", code)
	}
	_, body := doJSON(t, h, http.MethodGet, "/workspaces/"+wsID+"/billing", token, nil)
	if body["plan"] != "free" || body["status"] != "canceled" {
		t.Fatalf("after cancel plan/status = %v/%v, want free/canceled", body["plan"], body["status"])
	}
}

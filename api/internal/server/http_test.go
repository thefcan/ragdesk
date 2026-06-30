package server_test

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"

	"github.com/thefcan/ragdesk/api/internal/ai"
	"github.com/thefcan/ragdesk/api/internal/auth"
	"github.com/thefcan/ragdesk/api/internal/billing"
	"github.com/thefcan/ragdesk/api/internal/server"
	"github.com/thefcan/ragdesk/api/internal/store"
)

func newTestServer(t *testing.T, aiURL string) http.Handler {
	h, _ := newTestServerAndPool(t, aiURL)
	return h
}

// newTestServerAndPool builds the test server and also returns the pool, so
// tests that need to seed rows directly (e.g. plan-limit fixtures) can do so.
func newTestServerAndPool(t *testing.T, aiURL string) (http.Handler, *pgxpool.Pool) {
	t.Helper()
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		t.Skip("DATABASE_URL not set; skipping integration test")
	}
	pool, err := pgxpool.New(context.Background(), dsn)
	if err != nil {
		t.Fatalf("pool: %v", err)
	}
	t.Cleanup(pool.Close)

	st := store.New(pool)
	if err := st.Migrate(context.Background()); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	if _, err := pool.Exec(context.Background(),
		`TRUNCATE users, workspaces, workspace_members, documents, chunks, usage_counters, processed_webhook_events RESTART IDENTITY CASCADE`); err != nil {
		t.Fatalf("truncate: %v", err)
	}

	rdb := redis.NewClient(&redis.Options{Addr: "localhost:6379"})
	t.Cleanup(func() { _ = rdb.Close() })
	iss := auth.NewIssuer("test-secret", time.Hour)
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	h := server.New(st, rdb, iss, ai.NewClient(aiURL, ""), billing.Noop{}, []string{"*"}, "http://localhost:3000", log).Handler()
	return h, pool
}

func TestMetricsEndpoint(t *testing.T) {
	h := newTestServer(t, "")

	// Exercise a route so a per-route HTTP metric is recorded.
	h.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/healthz", nil))

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/metrics", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("/metrics status = %d, want 200", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "ragdesk_http_requests_total") {
		t.Fatal("/metrics is missing the ragdesk_http_requests_total series")
	}
}

func doJSON(t *testing.T, h http.Handler, method, path, token string, body any) (int, map[string]any) {
	t.Helper()
	var buf bytes.Buffer
	if body != nil {
		if err := json.NewEncoder(&buf).Encode(body); err != nil {
			t.Fatalf("encode: %v", err)
		}
	}
	req := httptest.NewRequest(method, path, &buf)
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	var out map[string]any
	if rec.Body.Len() > 0 {
		_ = json.Unmarshal(rec.Body.Bytes(), &out)
	}
	return rec.Code, out
}

func TestAuthAndWorkspaceHTTP(t *testing.T) {
	h := newTestServer(t, "http://unused")
	cred := map[string]any{"email": "alice@example.com", "password": "supersecret"}

	// Register -> 201 with a token and a bootstrapped workspace.
	code, body := doJSON(t, h, http.MethodPost, "/auth/register", "", cred)
	if code != http.StatusCreated {
		t.Fatalf("register status = %d, body %v", code, body)
	}
	token, _ := body["token"].(string)
	if token == "" {
		t.Fatal("register returned no token")
	}

	// Workspace list without a token -> 401.
	if code, _ := doJSON(t, h, http.MethodGet, "/workspaces", "", nil); code != http.StatusUnauthorized {
		t.Fatalf("unauthenticated list status = %d, want 401", code)
	}

	// Authenticated list -> the single bootstrapped workspace.
	code, body = doJSON(t, h, http.MethodGet, "/workspaces", token, nil)
	if code != http.StatusOK {
		t.Fatalf("list status = %d", code)
	}
	if wss, _ := body["workspaces"].([]any); len(wss) != 1 {
		t.Fatalf("got %d workspaces, want 1", len(wss))
	}

	// Login with the right password -> 200 + token.
	if code, body := doJSON(t, h, http.MethodPost, "/auth/login", "", cred); code != http.StatusOK || body["token"] == "" {
		t.Fatalf("login status = %d, body %v", code, body)
	}

	// Login with a wrong password -> 401.
	bad := map[string]any{"email": "alice@example.com", "password": "nope"}
	if code, _ := doJSON(t, h, http.MethodPost, "/auth/login", "", bad); code != http.StatusUnauthorized {
		t.Fatalf("bad login status = %d, want 401", code)
	}

	// Create a second workspace -> 201.
	if code, _ := doJSON(t, h, http.MethodPost, "/workspaces", token, map[string]any{"name": "Team Space"}); code != http.StatusCreated {
		t.Fatalf("create workspace status = %d, want 201", code)
	}
}

func TestDocumentSizeLimit(t *testing.T) {
	h := newTestServer(t, "http://unused")
	_, body := doJSON(t, h, http.MethodPost, "/auth/register", "", map[string]any{
		"email": "alice@example.com", "password": "supersecret",
	})
	token, _ := body["token"].(string)
	ws, _ := body["workspace"].(map[string]any)
	wsID, _ := ws["id"].(string)
	if token == "" || wsID == "" {
		t.Fatalf("register did not return token/workspace: %v", body)
	}

	big := strings.Repeat("a", (1<<20)+1) // just over 1 MiB
	code, _ := doJSON(t, h, http.MethodPost, "/workspaces/"+wsID+"/documents", token,
		map[string]any{"title": "big", "content": big})
	if code != http.StatusRequestEntityTooLarge {
		t.Fatalf("oversized document status = %d, want 413", code)
	}
}

func TestChatStreaming(t *testing.T) {
	aiSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/x-ndjson")
		_, _ = io.WriteString(w, `{"type":"sources","sources":[{"document_id":"d","title":"Doc","snippet":"x"}]}`+"\n")
		_, _ = io.WriteString(w, `{"type":"token","content":"Hello"}`+"\n")
		_, _ = io.WriteString(w, `{"type":"done"}`+"\n")
	}))
	defer aiSrv.Close()

	h := newTestServer(t, aiSrv.URL)
	_, body := doJSON(t, h, http.MethodPost, "/auth/register", "", map[string]any{
		"email": "alice@example.com", "password": "supersecret",
	})
	token, _ := body["token"].(string)
	ws, _ := body["workspace"].(map[string]any)
	wsID, _ := ws["id"].(string)
	if token == "" || wsID == "" {
		t.Fatalf("register did not return token/workspace: %v", body)
	}

	req := httptest.NewRequest(http.MethodPost, "/workspaces/"+wsID+"/chat", strings.NewReader(`{"question":"hi"}`))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("chat status = %d", rec.Code)
	}
	out := rec.Body.String()
	for _, want := range []string{`"type":"sources"`, `"content":"Hello"`, `"type":"done"`} {
		if !strings.Contains(out, want) {
			t.Fatalf("stream missing %q; got %s", want, out)
		}
	}

	// A non-member must not be able to query the workspace.
	req2 := httptest.NewRequest(http.MethodPost, "/workspaces/00000000-0000-0000-0000-000000000000/chat", strings.NewReader(`{"question":"hi"}`))
	req2.Header.Set("Authorization", "Bearer "+token)
	rec2 := httptest.NewRecorder()
	h.ServeHTTP(rec2, req2)
	if rec2.Code != http.StatusNotFound {
		t.Fatalf("foreign workspace chat status = %d, want 404", rec2.Code)
	}
}

func TestChatQuestionSizeLimit(t *testing.T) {
	h := newTestServer(t, "http://unused")
	_, body := doJSON(t, h, http.MethodPost, "/auth/register", "", map[string]any{
		"email": "alice@example.com", "password": "supersecret",
	})
	token, _ := body["token"].(string)
	ws, _ := body["workspace"].(map[string]any)
	wsID, _ := ws["id"].(string)

	big := strings.Repeat("q", (4<<10)+1) // just over 4 KiB
	req := httptest.NewRequest(http.MethodPost, "/workspaces/"+wsID+"/chat",
		strings.NewReader(`{"question":"`+big+`"}`))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("oversized question status = %d, want 413", rec.Code)
	}
}

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
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"

	"github.com/thefcan/ragdesk/api/internal/auth"
	"github.com/thefcan/ragdesk/api/internal/server"
	"github.com/thefcan/ragdesk/api/internal/store"
)

func newTestServer(t *testing.T) http.Handler {
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
		`TRUNCATE users, workspaces, workspace_members RESTART IDENTITY CASCADE`); err != nil {
		t.Fatalf("truncate: %v", err)
	}

	rdb := redis.NewClient(&redis.Options{Addr: "localhost:6379"})
	t.Cleanup(func() { _ = rdb.Close() })
	iss := auth.NewIssuer("test-secret", time.Hour)
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	return server.New(st, rdb, iss, log).Handler()
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
	h := newTestServer(t)
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

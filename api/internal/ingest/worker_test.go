package ingest_test

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/thefcan/ragdesk/api/internal/ai"
	"github.com/thefcan/ragdesk/api/internal/ingest"
	"github.com/thefcan/ragdesk/api/internal/store"
)

func TestWorkerProcess(t *testing.T) {
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
		`TRUNCATE users, workspaces, workspace_members, documents, chunks RESTART IDENTITY CASCADE`); err != nil {
		t.Fatalf("truncate: %v", err)
	}

	ctx := context.Background()
	u, err := st.CreateUser(ctx, "alice@example.com", "h")
	if err != nil {
		t.Fatalf("user: %v", err)
	}
	ws, err := st.CreateWorkspace(ctx, u.ID, "WS", "ws")
	if err != nil {
		t.Fatalf("workspace: %v", err)
	}
	doc, err := st.CreateDocument(ctx, u.ID, ws.ID, "Doc", "some text to ingest")
	if err != nil {
		t.Fatalf("document: %v", err)
	}

	// Fake AI service returns a fixed chunk count.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"chunk_count": 3}`))
	}))
	defer srv.Close()

	worker := ingest.NewWorker(nil, st, ai.NewClient(srv.URL), slog.New(slog.NewTextHandler(io.Discard, nil)))
	worker.Process(ctx, doc.ID)

	docs, err := st.ListDocuments(ctx, u.ID, ws.ID)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(docs) != 1 || docs[0].Status != "ready" || docs[0].ChunkCount != 3 {
		t.Fatalf("after process: %+v, want ready/3", docs)
	}
}

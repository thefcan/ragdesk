package store_test

import (
	"context"
	"errors"
	"testing"

	"github.com/thefcan/ragdesk/api/internal/store"
)

func TestDocumentFlow(t *testing.T) {
	st := newTestStore(t)
	ctx := context.Background()

	u, err := st.CreateUser(ctx, "alice@example.com", "h")
	if err != nil {
		t.Fatalf("user: %v", err)
	}
	ws, err := st.CreateWorkspace(ctx, u.ID, "WS", "ws")
	if err != nil {
		t.Fatalf("workspace: %v", err)
	}

	doc, err := st.CreateDocument(ctx, u.ID, ws.ID, "Doc 1", "hello content")
	if err != nil {
		t.Fatalf("create document: %v", err)
	}
	if doc.Status != "pending" {
		t.Fatalf("new document status = %q, want pending", doc.Status)
	}

	wsID, text, err := st.DocumentText(ctx, doc.ID)
	if err != nil {
		t.Fatalf("document text: %v", err)
	}
	if wsID != ws.ID || text != "hello content" {
		t.Fatalf("DocumentText = (%s, %q), want (%s, hello content)", wsID, text, ws.ID)
	}

	if err := st.MarkDocumentReady(ctx, doc.ID, 5); err != nil {
		t.Fatalf("mark ready: %v", err)
	}
	docs, err := st.ListDocuments(ctx, u.ID, ws.ID)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(docs) != 1 || docs[0].Status != "ready" || docs[0].ChunkCount != 5 {
		t.Fatalf("after ready: %+v", docs)
	}
}

func TestDocumentTenantIsolation(t *testing.T) {
	st := newTestStore(t)
	ctx := context.Background()

	alice, err := st.CreateUser(ctx, "alice@example.com", "h")
	if err != nil {
		t.Fatalf("alice: %v", err)
	}
	bob, err := st.CreateUser(ctx, "bob@example.com", "h")
	if err != nil {
		t.Fatalf("bob: %v", err)
	}
	ws, err := st.CreateWorkspace(ctx, alice.ID, "WS", "ws")
	if err != nil {
		t.Fatalf("workspace: %v", err)
	}

	if _, err := st.CreateDocument(ctx, bob.ID, ws.ID, "x", "y"); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("create isolation: want ErrNotFound, got %v", err)
	}
	if _, err := st.ListDocuments(ctx, bob.ID, ws.ID); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("list isolation: want ErrNotFound, got %v", err)
	}
}

package store_test

import (
	"context"
	"errors"
	"os"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/thefcan/ragdesk/api/internal/store"
)

// newTestStore connects to DATABASE_URL, migrates and resets the schema.
// It skips when DATABASE_URL is unset so unit runs stay hermetic.
func newTestStore(t *testing.T) *store.Store {
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
	return st
}

func TestUserAndWorkspaceFlow(t *testing.T) {
	st := newTestStore(t)
	ctx := context.Background()

	u, err := st.CreateUser(ctx, "alice@example.com", "hash")
	if err != nil {
		t.Fatalf("create user: %v", err)
	}
	if _, err := st.CreateUser(ctx, "alice@example.com", "hash"); !errors.Is(err, store.ErrConflict) {
		t.Fatalf("duplicate email: want ErrConflict, got %v", err)
	}

	ws, err := st.CreateWorkspace(ctx, u.ID, "Alice WS", "alice-ws")
	if err != nil {
		t.Fatalf("create workspace: %v", err)
	}
	if ws.Role != "owner" {
		t.Fatalf("creator role = %q, want owner", ws.Role)
	}

	list, err := st.ListWorkspacesForUser(ctx, u.ID)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("got %d workspaces, want 1", len(list))
	}
}

func TestTenantIsolation(t *testing.T) {
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
	ws, err := st.CreateWorkspace(ctx, alice.ID, "Alice WS", "alice-ws")
	if err != nil {
		t.Fatalf("workspace: %v", err)
	}

	// Bob is not a member: must not be able to read Alice's workspace.
	if _, err := st.GetWorkspaceForUser(ctx, bob.ID, ws.ID); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("isolation breach: want ErrNotFound, got %v", err)
	}
	bws, err := st.ListWorkspacesForUser(ctx, bob.ID)
	if err != nil {
		t.Fatalf("bob list: %v", err)
	}
	if len(bws) != 0 {
		t.Fatalf("bob sees %d workspaces, want 0", len(bws))
	}
}

func TestAddMemberPermissions(t *testing.T) {
	st := newTestStore(t)
	ctx := context.Background()

	alice, _ := mustUser(t, st, "alice@example.com")
	bob, _ := mustUser(t, st, "bob@example.com")
	mustUser(t, st, "carol@example.com")
	ws, err := st.CreateWorkspace(ctx, alice.ID, "WS", "ws")
	if err != nil {
		t.Fatalf("workspace: %v", err)
	}

	// Owner adds Bob as a member.
	if _, err := st.AddMemberByEmail(ctx, alice.ID, ws.ID, "bob@example.com", "member"); err != nil {
		t.Fatalf("owner add member: %v", err)
	}
	if _, err := st.GetWorkspaceForUser(ctx, bob.ID, ws.ID); err != nil {
		t.Fatalf("bob should see workspace after being added: %v", err)
	}

	// A plain member cannot add others.
	if _, err := st.AddMemberByEmail(ctx, bob.ID, ws.ID, "carol@example.com", "member"); !errors.Is(err, store.ErrForbidden) {
		t.Fatalf("member add: want ErrForbidden, got %v", err)
	}
}

func mustUser(t *testing.T, st *store.Store, email string) (store.User, string) {
	t.Helper()
	u, err := st.CreateUser(context.Background(), email, "h")
	if err != nil {
		t.Fatalf("create %s: %v", email, err)
	}
	return u, u.ID
}

package store

import (
	"context"
	"time"
)

// Member is a user's membership in a workspace.
type Member struct {
	UserID    string    `json:"user_id"`
	Email     string    `json:"email"`
	Role      string    `json:"role"`
	CreatedAt time.Time `json:"created_at"`
}

// ListMembers returns the members of a workspace the requester belongs to.
func (s *Store) ListMembers(ctx context.Context, requesterID, workspaceID string) ([]Member, error) {
	if _, err := s.GetWorkspaceForUser(ctx, requesterID, workspaceID); err != nil {
		return nil, err // ErrNotFound also hides workspaces the user can't see
	}
	rows, err := s.pool.Query(ctx,
		`SELECT m.user_id::text, u.email, m.role, m.created_at
		 FROM workspace_members m
		 JOIN users u ON u.id = m.user_id
		 WHERE m.workspace_id = $1
		 ORDER BY m.created_at`,
		workspaceID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := []Member{}
	for rows.Next() {
		var m Member
		if err := rows.Scan(&m.UserID, &m.Email, &m.Role, &m.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

// AddMemberByEmail adds an existing user to a workspace. Only owners and admins
// may add members; the target user must already have an account.
func (s *Store) AddMemberByEmail(ctx context.Context, requesterID, workspaceID, email, role string) (Member, error) {
	ws, err := s.GetWorkspaceForUser(ctx, requesterID, workspaceID)
	if err != nil {
		return Member{}, err
	}
	if ws.Role != "owner" && ws.Role != "admin" {
		return Member{}, ErrForbidden
	}
	if role != "admin" && role != "member" {
		role = "member"
	}
	u, err := s.GetUserByEmail(ctx, email)
	if err != nil {
		return Member{}, err // ErrNotFound if no such user
	}
	if _, err := s.pool.Exec(ctx,
		`INSERT INTO workspace_members (workspace_id, user_id, role) VALUES ($1, $2, $3)
		 ON CONFLICT (workspace_id, user_id) DO UPDATE SET role = EXCLUDED.role`,
		workspaceID, u.ID, role,
	); err != nil {
		return Member{}, err
	}
	return Member{UserID: u.ID, Email: u.Email, Role: role, CreatedAt: time.Now()}, nil
}

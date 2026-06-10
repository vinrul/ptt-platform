package ws

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"

	"ptt-fleet/services/api-server/internal/db"
)

var (
	ErrUserNotFound    = errors.New("user not found")
	ErrUserDisabled    = errors.New("user is disabled")
	ErrGroupNotAllowed = errors.New("user is not a member of the group")
)

type Identity struct {
	UserID   string
	Username string
	Role     string
}

type PushTarget struct {
	UserID    string
	PushToken string
}

type AccessRepository interface {
	ActiveIdentity(ctx context.Context, userID string) (Identity, error)
	CanJoinGroup(ctx context.Context, userID string, groupID string) error
	GetOfflineGroupMembersPushTokens(ctx context.Context, groupID string, excludeUserID string) ([]PushTarget, error)
}

type Repository struct {
	store *db.Store
}

func NewRepository(store *db.Store) *Repository {
	return &Repository{store: store}
}

func (r *Repository) ActiveIdentity(ctx context.Context, userID string) (Identity, error) {
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()

	var identity Identity
	var status string
	err := r.store.Postgres.QueryRow(ctx, `
		SELECT id, username, role, status
		FROM users
		WHERE id = $1
	`, userID).Scan(&identity.UserID, &identity.Username, &identity.Role, &status)
	if errors.Is(err, pgx.ErrNoRows) {
		return Identity{}, ErrUserNotFound
	}
	if err != nil {
		return Identity{}, err
	}
	if status != "active" {
		return Identity{}, ErrUserDisabled
	}

	return identity, nil
}

func (r *Repository) CanJoinGroup(ctx context.Context, userID string, groupID string) error {
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()

	var allowed bool
	err := r.store.Postgres.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT 1
			FROM group_members gm
			JOIN users u ON u.id = gm.user_id
			WHERE gm.group_id = $1
			  AND gm.user_id = $2
			  AND u.status = 'active'
		)
	`, groupID, userID).Scan(&allowed)
	if err != nil {
		return err
	}
	if !allowed {
		return ErrGroupNotAllowed
	}
	return nil
}

func (r *Repository) GetOfflineGroupMembersPushTokens(ctx context.Context, groupID string, excludeUserID string) ([]PushTarget, error) {
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()

	rows, err := r.store.Postgres.Query(ctx, `
		SELECT DISTINCT gm.user_id, d.push_token
		FROM group_members gm
		JOIN users u ON u.id = gm.user_id
		JOIN devices d ON d.user_id = u.id
		WHERE gm.group_id = $1
		  AND gm.user_id <> $2
		  AND u.status = 'active'
		  AND d.status = 'active'
		  AND d.push_token IS NOT NULL
	`, groupID, excludeUserID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var targets []PushTarget
	for rows.Next() {
		var target PushTarget
		if err := rows.Scan(&target.UserID, &target.PushToken); err != nil {
			return nil, err
		}
		targets = append(targets, target)
	}
	return targets, rows.Err()
}

package groups

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"ptt-fleet/services/api-server/internal/db"
)

var (
	ErrNotFound       = errors.New("group not found")
	ErrNameConflict   = errors.New("group name already exists")
	ErrHasMembers     = errors.New("group still has members")
	ErrMemberConflict = errors.New("user is already a group member")
	ErrMemberNotFound = errors.New("group member not found")
)

type Group struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	CreatedAt   time.Time `json:"createdAt"`
	UpdatedAt   time.Time `json:"updatedAt"`
}

type Member struct {
	UserID      string    `json:"userId"`
	Username    string    `json:"username"`
	FullName    string    `json:"fullName"`
	RoleInGroup string    `json:"roleInGroup"`
	JoinedAt    time.Time `json:"joinedAt"`
}

type Detail struct {
	Group
	Members []Member `json:"members"`
}

type Service struct {
	store *db.Store
}

func NewService(store *db.Store) *Service {
	return &Service{store: store}
}

func (s *Service) List(ctx context.Context) ([]Group, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	rows, err := s.store.Postgres.Query(ctx, `
		SELECT id, name, description, created_at, updated_at
		FROM groups
		ORDER BY name
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]Group, 0)
	for rows.Next() {
		var group Group
		if err := rows.Scan(&group.ID, &group.Name, &group.Description, &group.CreatedAt, &group.UpdatedAt); err != nil {
			return nil, err
		}
		items = append(items, group)
	}
	return items, rows.Err()
}

func (s *Service) ListForUser(ctx context.Context, userID string) ([]Group, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	rows, err := s.store.Postgres.Query(ctx, `
		SELECT g.id, g.name, g.description, g.created_at, g.updated_at
		FROM groups g
		JOIN group_members gm ON gm.group_id = g.id
		WHERE gm.user_id = $1
		ORDER BY g.name
	`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]Group, 0)
	for rows.Next() {
		var group Group
		if err := rows.Scan(&group.ID, &group.Name, &group.Description, &group.CreatedAt, &group.UpdatedAt); err != nil {
			return nil, err
		}
		items = append(items, group)
	}
	return items, rows.Err()
}

func (s *Service) Create(ctx context.Context, actorID string, name string, description string) (Group, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	tx, err := s.store.Postgres.Begin(ctx)
	if err != nil {
		return Group{}, err
	}
	defer tx.Rollback(ctx)

	var group Group
	err = tx.QueryRow(ctx, `
		INSERT INTO groups (name, description)
		VALUES ($1, $2)
		RETURNING id, name, description, created_at, updated_at
	`, name, description).Scan(&group.ID, &group.Name, &group.Description, &group.CreatedAt, &group.UpdatedAt)
	if isUniqueViolation(err) {
		return Group{}, ErrNameConflict
	}
	if err != nil {
		return Group{}, err
	}
	if err := writeAudit(ctx, tx, actorID, "group.created", group.ID); err != nil {
		return Group{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return Group{}, err
	}
	return group, nil
}

func (s *Service) Get(ctx context.Context, groupID string) (Detail, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	var detail Detail
	err := s.store.Postgres.QueryRow(ctx, `
		SELECT id, name, description, created_at, updated_at
		FROM groups
		WHERE id = $1
	`, groupID).Scan(&detail.ID, &detail.Name, &detail.Description, &detail.CreatedAt, &detail.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return Detail{}, ErrNotFound
	}
	if err != nil {
		return Detail{}, err
	}

	rows, err := s.store.Postgres.Query(ctx, `
		SELECT u.id, u.username, u.full_name, gm.role_in_group, gm.joined_at
		FROM group_members gm
		JOIN users u ON u.id = gm.user_id
		WHERE gm.group_id = $1
		ORDER BY u.full_name
	`, groupID)
	if err != nil {
		return Detail{}, err
	}
	defer rows.Close()

	detail.Members = make([]Member, 0)
	for rows.Next() {
		var member Member
		if err := rows.Scan(&member.UserID, &member.Username, &member.FullName, &member.RoleInGroup, &member.JoinedAt); err != nil {
			return Detail{}, err
		}
		detail.Members = append(detail.Members, member)
	}
	return detail, rows.Err()
}

func (s *Service) Update(ctx context.Context, actorID string, groupID string, name *string, description *string) (Group, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	tx, err := s.store.Postgres.Begin(ctx)
	if err != nil {
		return Group{}, err
	}
	defer tx.Rollback(ctx)

	var group Group
	err = tx.QueryRow(ctx, `
		UPDATE groups
		SET name = COALESCE($2, name),
		    description = COALESCE($3, description),
		    updated_at = now()
		WHERE id = $1
		RETURNING id, name, description, created_at, updated_at
	`, groupID, name, description).Scan(
		&group.ID,
		&group.Name,
		&group.Description,
		&group.CreatedAt,
		&group.UpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return Group{}, ErrNotFound
	}
	if isUniqueViolation(err) {
		return Group{}, ErrNameConflict
	}
	if err != nil {
		return Group{}, err
	}
	if err := writeAudit(ctx, tx, actorID, "group.updated", group.ID); err != nil {
		return Group{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return Group{}, err
	}
	return group, nil
}

func (s *Service) Delete(ctx context.Context, actorID string, groupID string) error {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	tx, err := s.store.Postgres.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	var memberCount int
	err = tx.QueryRow(ctx, `SELECT count(*) FROM group_members WHERE group_id = $1`, groupID).Scan(&memberCount)
	if err != nil {
		return err
	}
	if memberCount > 0 {
		return ErrHasMembers
	}

	result, err := tx.Exec(ctx, `DELETE FROM groups WHERE id = $1`, groupID)
	if err != nil {
		return err
	}
	if result.RowsAffected() == 0 {
		return ErrNotFound
	}
	if err := writeAudit(ctx, tx, actorID, "group.deleted", groupID); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (s *Service) AddMember(ctx context.Context, actorID string, groupID string, userID string, roleInGroup string) (Member, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	tx, err := s.store.Postgres.Begin(ctx)
	if err != nil {
		return Member{}, err
	}
	defer tx.Rollback(ctx)

	var member Member
	err = tx.QueryRow(ctx, `
		INSERT INTO group_members (group_id, user_id, role_in_group)
		VALUES ($1, $2, $3)
		RETURNING user_id, role_in_group, joined_at
	`, groupID, userID, roleInGroup).Scan(&member.UserID, &member.RoleInGroup, &member.JoinedAt)
	if isUniqueViolation(err) {
		return Member{}, ErrMemberConflict
	}
	if isForeignKeyViolation(err) {
		return Member{}, ErrNotFound
	}
	if err != nil {
		return Member{}, err
	}
	if err := tx.QueryRow(ctx, `
		SELECT username, full_name
		FROM users
		WHERE id = $1
	`, userID).Scan(&member.Username, &member.FullName); err != nil {
		return Member{}, err
	}
	if err := writeAudit(ctx, tx, actorID, "group.member_added", groupID); err != nil {
		return Member{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return Member{}, err
	}
	return member, nil
}

func (s *Service) RemoveMember(ctx context.Context, actorID string, groupID string, userID string) error {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	tx, err := s.store.Postgres.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	result, err := tx.Exec(ctx, `
		DELETE FROM group_members
		WHERE group_id = $1 AND user_id = $2
	`, groupID, userID)
	if err != nil {
		return err
	}
	if result.RowsAffected() == 0 {
		return ErrMemberNotFound
	}
	if err := writeAudit(ctx, tx, actorID, "group.member_removed", groupID); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func writeAudit(ctx context.Context, tx pgx.Tx, actorID string, action string, entityID string) error {
	_, err := tx.Exec(ctx, `
		INSERT INTO audit_logs (actor_user_id, action, entity_type, entity_id)
		VALUES ($1, $2, 'group', $3)
	`, actorID, action, entityID)
	return err
}

func isUniqueViolation(err error) bool {
	var databaseError *pgconn.PgError
	return errors.As(err, &databaseError) && databaseError.Code == "23505"
}

func isForeignKeyViolation(err error) bool {
	var databaseError *pgconn.PgError
	return errors.As(err, &databaseError) && databaseError.Code == "23503"
}

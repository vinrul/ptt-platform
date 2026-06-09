package users

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"ptt-fleet/services/api-server/internal/auth"
	"ptt-fleet/services/api-server/internal/db"
)

var (
	ErrNotFound         = errors.New("user not found")
	ErrUsernameConflict = errors.New("username already exists")
)

type User struct {
	ID        string    `json:"id"`
	Username  string    `json:"username"`
	FullName  string    `json:"fullName"`
	Role      string    `json:"role"`
	Status    string    `json:"status"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

type ListInput struct {
	Page     int
	PageSize int
	Role     string
	Status   string
	Query    string
}

type ListResult struct {
	Items    []User `json:"items"`
	Page     int    `json:"page"`
	PageSize int    `json:"pageSize"`
	Total    int64  `json:"total"`
}

type CreateInput struct {
	Username string
	Password string
	FullName string
	Role     string
}

type UpdateInput struct {
	FullName *string
	Role     *string
	Status   *string
}

type Service struct {
	store *db.Store
}

func NewService(store *db.Store) *Service {
	return &Service{store: store}
}

func (s *Service) List(ctx context.Context, input ListInput) (ListResult, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	where, args := buildFilters(input)

	var total int64
	if err := s.store.Postgres.QueryRow(ctx, "SELECT count(*) FROM users"+where, args...).Scan(&total); err != nil {
		return ListResult{}, err
	}

	queryArgs := append(args, input.PageSize, (input.Page-1)*input.PageSize)
	rows, err := s.store.Postgres.Query(ctx, `
		SELECT id, username, full_name, role, status, created_at, updated_at
		FROM users`+where+fmt.Sprintf(`
		ORDER BY created_at DESC
		LIMIT $%d OFFSET $%d
	`, len(args)+1, len(args)+2), queryArgs...)
	if err != nil {
		return ListResult{}, err
	}
	defer rows.Close()

	items := make([]User, 0)
	for rows.Next() {
		var user User
		if err := rows.Scan(
			&user.ID,
			&user.Username,
			&user.FullName,
			&user.Role,
			&user.Status,
			&user.CreatedAt,
			&user.UpdatedAt,
		); err != nil {
			return ListResult{}, err
		}
		items = append(items, user)
	}
	if err := rows.Err(); err != nil {
		return ListResult{}, err
	}

	return ListResult{
		Items:    items,
		Page:     input.Page,
		PageSize: input.PageSize,
		Total:    total,
	}, nil
}

func (s *Service) Create(ctx context.Context, actorID string, input CreateInput) (User, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	passwordHash, err := auth.HashPassword(input.Password)
	if err != nil {
		return User{}, err
	}

	tx, err := s.store.Postgres.Begin(ctx)
	if err != nil {
		return User{}, err
	}
	defer tx.Rollback(ctx)

	var user User
	err = tx.QueryRow(ctx, `
		INSERT INTO users (username, password_hash, full_name, role)
		VALUES ($1, $2, $3, $4)
		RETURNING id, username, full_name, role, status, created_at, updated_at
	`, input.Username, passwordHash, input.FullName, input.Role).Scan(
		&user.ID,
		&user.Username,
		&user.FullName,
		&user.Role,
		&user.Status,
		&user.CreatedAt,
		&user.UpdatedAt,
	)
	if isUniqueViolation(err) {
		return User{}, ErrUsernameConflict
	}
	if err != nil {
		return User{}, err
	}

	if err := writeAudit(ctx, tx, actorID, "user.created", user.ID); err != nil {
		return User{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return User{}, err
	}

	return user, nil
}

func (s *Service) Get(ctx context.Context, userID string) (User, error) {
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()

	var user User
	err := s.store.Postgres.QueryRow(ctx, `
		SELECT id, username, full_name, role, status, created_at, updated_at
		FROM users
		WHERE id = $1
	`, userID).Scan(
		&user.ID,
		&user.Username,
		&user.FullName,
		&user.Role,
		&user.Status,
		&user.CreatedAt,
		&user.UpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return User{}, ErrNotFound
	}
	return user, err
}

func (s *Service) Update(ctx context.Context, actorID string, userID string, input UpdateInput) (User, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	tx, err := s.store.Postgres.Begin(ctx)
	if err != nil {
		return User{}, err
	}
	defer tx.Rollback(ctx)

	var user User
	err = tx.QueryRow(ctx, `
		UPDATE users
		SET full_name = COALESCE($2, full_name),
		    role = COALESCE($3, role),
		    status = COALESCE($4, status),
		    updated_at = now()
		WHERE id = $1
		RETURNING id, username, full_name, role, status, created_at, updated_at
	`, userID, input.FullName, input.Role, input.Status).Scan(
		&user.ID,
		&user.Username,
		&user.FullName,
		&user.Role,
		&user.Status,
		&user.CreatedAt,
		&user.UpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return User{}, ErrNotFound
	}
	if err != nil {
		return User{}, err
	}

	if err := writeAudit(ctx, tx, actorID, "user.updated", user.ID); err != nil {
		return User{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return User{}, err
	}

	return user, nil
}

func (s *Service) Disable(ctx context.Context, actorID string, userID string) error {
	status := "disabled"
	_, err := s.Update(ctx, actorID, userID, UpdateInput{Status: &status})
	return err
}

func buildFilters(input ListInput) (string, []any) {
	conditions := make([]string, 0, 3)
	args := make([]any, 0, 3)

	if input.Role != "" {
		args = append(args, input.Role)
		conditions = append(conditions, fmt.Sprintf("role = $%d", len(args)))
	}
	if input.Status != "" {
		args = append(args, input.Status)
		conditions = append(conditions, fmt.Sprintf("status = $%d", len(args)))
	}
	if input.Query != "" {
		args = append(args, "%"+input.Query+"%")
		conditions = append(conditions, fmt.Sprintf("(username ILIKE $%d OR full_name ILIKE $%d)", len(args), len(args)))
	}
	if len(conditions) == 0 {
		return "", args
	}
	return " WHERE " + strings.Join(conditions, " AND "), args
}

func writeAudit(ctx context.Context, tx pgx.Tx, actorID string, action string, entityID string) error {
	_, err := tx.Exec(ctx, `
		INSERT INTO audit_logs (actor_user_id, action, entity_type, entity_id)
		VALUES ($1, $2, 'user', $3)
	`, actorID, action, entityID)
	return err
}

func isUniqueViolation(err error) bool {
	var databaseError *pgconn.PgError
	return errors.As(err, &databaseError) && databaseError.Code == "23505"
}

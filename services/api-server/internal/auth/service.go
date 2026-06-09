package auth

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"

	"ptt-fleet/services/api-server/internal/db"
)

var (
	ErrInvalidCredentials  = errors.New("invalid username or password")
	ErrInvalidRefreshToken = errors.New("invalid refresh token")
	ErrUserDisabled        = errors.New("user is disabled")
	ErrUserNotFound        = errors.New("user not found")
)

type User struct {
	ID       string `json:"id"`
	Username string `json:"username"`
	FullName string `json:"fullName"`
	Role     string `json:"role"`
	Status   string `json:"status"`
}

type Session struct {
	AccessToken  string `json:"accessToken"`
	RefreshToken string `json:"refreshToken"`
	User         User   `json:"user"`
}

type Service struct {
	store      *db.Store
	tokens     *TokenManager
	refreshTTL time.Duration
}

func NewService(store *db.Store, tokens *TokenManager, refreshTTL time.Duration) *Service {
	return &Service{
		store:      store,
		tokens:     tokens,
		refreshTTL: refreshTTL,
	}
}

func (s *Service) Login(ctx context.Context, username string, password string, deviceName string, clientType string) (Session, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	var user User
	var passwordHash string
	err := s.store.Postgres.QueryRow(ctx, `
		SELECT id, username, password_hash, full_name, role, status
		FROM users
		WHERE username = $1
	`, username).Scan(&user.ID, &user.Username, &passwordHash, &user.FullName, &user.Role, &user.Status)
	if errors.Is(err, pgx.ErrNoRows) {
		return Session{}, ErrInvalidCredentials
	}
	if err != nil {
		return Session{}, err
	}
	if user.Status != "active" {
		return Session{}, ErrUserDisabled
	}
	if err := CheckPassword(passwordHash, password); err != nil {
		return Session{}, ErrInvalidCredentials
	}

	refreshToken, err := randomToken(32)
	if err != nil {
		return Session{}, err
	}
	accessToken, err := s.tokens.IssueAccessToken(user.ID, user.Username, user.Role)
	if err != nil {
		return Session{}, err
	}

	tx, err := s.store.Postgres.Begin(ctx)
	if err != nil {
		return Session{}, err
	}
	defer tx.Rollback(ctx)

	var deviceID string
	err = tx.QueryRow(ctx, `
		INSERT INTO devices (user_id, device_name, platform, last_seen_at)
		VALUES ($1, $2, $3, now())
		RETURNING id
	`, user.ID, deviceName, clientType).Scan(&deviceID)
	if err != nil {
		return Session{}, err
	}

	if err := insertRefreshToken(ctx, tx, user.ID, deviceID, refreshToken, s.refreshTTL); err != nil {
		return Session{}, err
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO audit_logs (actor_user_id, action, entity_type, entity_id)
		VALUES ($1, 'auth.login_success', 'user', $2)
	`, user.ID, user.ID); err != nil {
		return Session{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return Session{}, err
	}

	return Session{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		User:         user,
	}, nil
}

func (s *Service) Refresh(ctx context.Context, currentToken string) (Session, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	tx, err := s.store.Postgres.Begin(ctx)
	if err != nil {
		return Session{}, err
	}
	defer tx.Rollback(ctx)

	var user User
	var tokenID string
	var deviceID *string
	err = tx.QueryRow(ctx, `
		SELECT rt.id, rt.device_id, u.id, u.username, u.full_name, u.role, u.status
		FROM refresh_tokens rt
		JOIN users u ON u.id = rt.user_id
		WHERE rt.token_hash = $1
		  AND rt.revoked_at IS NULL
		  AND rt.expires_at > now()
		FOR UPDATE OF rt
	`, hashRefreshToken(currentToken)).Scan(
		&tokenID,
		&deviceID,
		&user.ID,
		&user.Username,
		&user.FullName,
		&user.Role,
		&user.Status,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return Session{}, ErrInvalidRefreshToken
	}
	if err != nil {
		return Session{}, err
	}
	if user.Status != "active" {
		return Session{}, ErrUserDisabled
	}

	nextToken, err := randomToken(32)
	if err != nil {
		return Session{}, err
	}
	accessToken, err := s.tokens.IssueAccessToken(user.ID, user.Username, user.Role)
	if err != nil {
		return Session{}, err
	}
	if _, err := tx.Exec(ctx, `UPDATE refresh_tokens SET revoked_at = now() WHERE id = $1`, tokenID); err != nil {
		return Session{}, err
	}
	if err := insertRefreshToken(ctx, tx, user.ID, nullableString(deviceID), nextToken, s.refreshTTL); err != nil {
		return Session{}, err
	}
	if deviceID != nil {
		if _, err := tx.Exec(ctx, `UPDATE devices SET last_seen_at = now() WHERE id = $1`, *deviceID); err != nil {
			return Session{}, err
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return Session{}, err
	}

	return Session{
		AccessToken:  accessToken,
		RefreshToken: nextToken,
		User:         user,
	}, nil
}

func (s *Service) Logout(ctx context.Context, userID string, refreshToken string) error {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	tx, err := s.store.Postgres.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	result, err := tx.Exec(ctx, `
		UPDATE refresh_tokens
		SET revoked_at = now()
		WHERE token_hash = $1 AND user_id = $2 AND revoked_at IS NULL
	`, hashRefreshToken(refreshToken), userID)
	if err != nil {
		return err
	}
	if result.RowsAffected() == 0 {
		return ErrInvalidRefreshToken
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO audit_logs (actor_user_id, action, entity_type, entity_id)
		VALUES ($1, 'auth.logout', 'user', $2)
	`, userID, userID); err != nil {
		return err
	}

	return tx.Commit(ctx)
}

func (s *Service) Me(ctx context.Context, userID string) (User, error) {
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()

	var user User
	err := s.store.Postgres.QueryRow(ctx, `
		SELECT id, username, full_name, role, status
		FROM users
		WHERE id = $1
	`, userID).Scan(&user.ID, &user.Username, &user.FullName, &user.Role, &user.Status)
	if errors.Is(err, pgx.ErrNoRows) {
		return User{}, ErrUserNotFound
	}
	return user, err
}

func insertRefreshToken(ctx context.Context, tx pgx.Tx, userID string, deviceID string, token string, ttl time.Duration) error {
	var nullableDeviceID any
	if deviceID != "" {
		nullableDeviceID = deviceID
	}

	_, err := tx.Exec(ctx, `
		INSERT INTO refresh_tokens (user_id, device_id, token_hash, expires_at)
		VALUES ($1, $2, $3, $4)
	`, userID, nullableDeviceID, hashRefreshToken(token), time.Now().UTC().Add(ttl))
	return err
}

func hashRefreshToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

func nullableString(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

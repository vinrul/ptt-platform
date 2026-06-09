package ptt

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5"

	"ptt-fleet/services/api-server/internal/db"
)

type PostgresRepository struct {
	store *db.Store
}

func NewRepository(store *db.Store) *PostgresRepository {
	return &PostgresRepository{store: store}
}

func (r *PostgresRepository) CreateSession(
	ctx context.Context,
	groupID string,
	speakerUserID string,
) (Session, error) {
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()

	tx, err := r.store.Postgres.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return Session{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var session Session
	err = tx.QueryRow(
		ctx,
		`INSERT INTO talk_sessions (group_id, speaker_user_id)
		 VALUES ($1, $2)
		 RETURNING id, group_id, speaker_user_id, started_at`,
		groupID,
		speakerUserID,
	).Scan(&session.ID, &session.GroupID, &session.SpeakerUserID, &session.StartedAt)
	if err != nil {
		return Session{}, err
	}

	_, err = tx.Exec(
		ctx,
		`INSERT INTO audit_logs (actor_user_id, action, entity_type, entity_id, metadata)
		 VALUES ($1, 'ptt.started', 'talk_session', $2, jsonb_build_object('groupId', $3::text))`,
		speakerUserID,
		session.ID,
		groupID,
	)
	if err != nil {
		return Session{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return Session{}, err
	}
	return session, nil
}

func (r *PostgresRepository) StopSession(
	ctx context.Context,
	sessionID string,
	reason string,
	startedAt time.Time,
) error {
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()

	tx, err := r.store.Postgres.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	duration := time.Since(startedAt).Milliseconds()
	var speakerUserID string
	_, err = tx.Exec(
		ctx,
		`UPDATE talk_sessions
		 SET ended_at = now(), duration_ms = $2, stop_reason = $3
		 WHERE id = $1 AND ended_at IS NULL`,
		sessionID,
		duration,
		reason,
	)
	if err != nil {
		return err
	}
	err = tx.QueryRow(
		ctx,
		`SELECT speaker_user_id FROM talk_sessions WHERE id = $1`,
		sessionID,
	).Scan(&speakerUserID)
	if err != nil {
		return err
	}
	_, err = tx.Exec(
		ctx,
		`INSERT INTO audit_logs (actor_user_id, action, entity_type, entity_id, metadata)
		 VALUES ($1, 'ptt.stopped', 'talk_session', $2, jsonb_build_object('reason', $3::text, 'durationMs', $4::bigint))`,
		speakerUserID,
		sessionID,
		reason,
		duration,
	)
	if err != nil {
		return err
	}
	return tx.Commit(ctx)
}

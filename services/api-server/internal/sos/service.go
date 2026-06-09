package sos

import (
	"context"
	"errors"
	"math"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"

	"ptt-fleet/services/api-server/internal/db"
)

var (
	ErrInvalidEvent = errors.New("invalid SOS event")
	ErrNotOpen      = errors.New("SOS event is not open")
)

type CreateInput struct {
	Lat     *float64 `json:"lat,omitempty"`
	Lng     *float64 `json:"lng,omitempty"`
	Message string   `json:"message"`
}

type Event struct {
	ID        string   `json:"id"`
	UserID    string   `json:"userId"`
	Lat       *float64 `json:"lat,omitempty"`
	Lng       *float64 `json:"lng,omitempty"`
	Message   string   `json:"message"`
	Status    string   `json:"status"`
	CreatedAt string   `json:"createdAt"`
}

type Acknowledgement struct {
	ID             string `json:"id"`
	Status         string `json:"status"`
	AcknowledgedBy string `json:"acknowledgedBy"`
	AcknowledgedAt string `json:"acknowledgedAt"`
}

type Service struct {
	store *db.Store
}

func NewService(store *db.Store) *Service {
	return &Service{store: store}
}

func (s *Service) Create(ctx context.Context, userID string, input CreateInput) (Event, error) {
	input.Message = strings.TrimSpace(input.Message)
	if input.Message == "" {
		input.Message = "Emergency"
	}
	if err := validateCreate(input); err != nil {
		return Event{}, err
	}

	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()

	tx, err := s.store.Postgres.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return Event{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var event Event
	var createdAt time.Time
	err = tx.QueryRow(
		ctx,
		`INSERT INTO sos_events (user_id, lat, lng, message)
		 VALUES ($1, $2, $3, $4)
		 RETURNING id, user_id, lat, lng, message, status, created_at`,
		userID,
		input.Lat,
		input.Lng,
		input.Message,
	).Scan(
		&event.ID,
		&event.UserID,
		&event.Lat,
		&event.Lng,
		&event.Message,
		&event.Status,
		&createdAt,
	)
	if err != nil {
		return Event{}, err
	}

	_, err = tx.Exec(
		ctx,
		`INSERT INTO audit_logs (actor_user_id, action, entity_type, entity_id, metadata)
		 VALUES ($1, 'sos.create', 'sos_event', $2, jsonb_build_object('lat', $3::double precision, 'lng', $4::double precision))`,
		userID,
		event.ID,
		input.Lat,
		input.Lng,
	)
	if err != nil {
		return Event{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return Event{}, err
	}

	event.CreatedAt = createdAt.UTC().Format(time.RFC3339Nano)
	return event, nil
}

func (s *Service) Acknowledge(ctx context.Context, operatorID string, eventID string) (Acknowledgement, error) {
	if strings.TrimSpace(eventID) == "" {
		return Acknowledgement{}, ErrInvalidEvent
	}

	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()

	tx, err := s.store.Postgres.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return Acknowledgement{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var acknowledgement Acknowledgement
	var acknowledgedAt time.Time
	err = tx.QueryRow(
		ctx,
		`UPDATE sos_events
		 SET status = 'ack', acknowledged_by = $2, acknowledged_at = now()
		 WHERE id = $1 AND status = 'open'
		 RETURNING id, status, acknowledged_by, acknowledged_at`,
		eventID,
		operatorID,
	).Scan(
		&acknowledgement.ID,
		&acknowledgement.Status,
		&acknowledgement.AcknowledgedBy,
		&acknowledgedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return Acknowledgement{}, ErrNotOpen
	}
	if err != nil {
		return Acknowledgement{}, err
	}

	_, err = tx.Exec(
		ctx,
		`INSERT INTO audit_logs (actor_user_id, action, entity_type, entity_id, metadata)
		 VALUES ($1, 'sos.ack', 'sos_event', $2, jsonb_build_object('status', 'ack'))`,
		operatorID,
		eventID,
	)
	if err != nil {
		return Acknowledgement{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return Acknowledgement{}, err
	}

	acknowledgement.AcknowledgedAt = acknowledgedAt.UTC().Format(time.RFC3339Nano)
	return acknowledgement, nil
}

func validateCreate(input CreateInput) error {
	if len(input.Message) > 500 {
		return ErrInvalidEvent
	}
	if (input.Lat == nil) != (input.Lng == nil) {
		return ErrInvalidEvent
	}
	if input.Lat != nil && (math.IsNaN(*input.Lat) || math.IsInf(*input.Lat, 0) || *input.Lat < -90 || *input.Lat > 90) {
		return ErrInvalidEvent
	}
	if input.Lng != nil && (math.IsNaN(*input.Lng) || math.IsInf(*input.Lng, 0) || *input.Lng < -180 || *input.Lng > 180) {
		return ErrInvalidEvent
	}
	return nil
}

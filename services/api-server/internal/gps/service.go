package gps

import (
	"context"
	"errors"
	"math"
	"time"

	"github.com/jackc/pgx/v5"

	"ptt-fleet/services/api-server/internal/db"
)

var (
	ErrInvalidLocation = errors.New("invalid GPS location")
	ErrUserNotFound    = errors.New("user not found")
)

type Update struct {
	Lat      float64  `json:"lat"`
	Lng      float64  `json:"lng"`
	Speed    *float64 `json:"speed,omitempty"`
	Heading  *float64 `json:"heading,omitempty"`
	Accuracy *float64 `json:"accuracy,omitempty"`
}

type Location struct {
	UserID     string   `json:"userId"`
	Lat        float64  `json:"lat"`
	Lng        float64  `json:"lng"`
	Speed      *float64 `json:"speed,omitempty"`
	Heading    *float64 `json:"heading,omitempty"`
	Accuracy   *float64 `json:"accuracy,omitempty"`
	RecordedAt string   `json:"recordedAt"`
}

type HistoryUser struct {
	ID       string `json:"id"`
	Username string `json:"username"`
	FullName string `json:"fullName"`
	Role     string `json:"role"`
	Status   string `json:"status"`
}

type HistoryResult struct {
	User  HistoryUser `json:"user"`
	Items []Location  `json:"items"`
}

type Service struct {
	store *db.Store
}

func NewService(store *db.Store) *Service {
	return &Service{store: store}
}

func (s *Service) Record(ctx context.Context, userID string, update Update) (Location, error) {
	if err := validate(update); err != nil {
		return Location{}, err
	}

	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()

	var recordedAt time.Time
	err := s.store.Postgres.QueryRow(
		ctx,
		`INSERT INTO gps_logs (user_id, lat, lng, speed, heading, accuracy)
		 VALUES ($1, $2, $3, $4, $5, $6)
		 RETURNING recorded_at`,
		userID,
		update.Lat,
		update.Lng,
		update.Speed,
		update.Heading,
		update.Accuracy,
	).Scan(&recordedAt)
	if err != nil {
		return Location{}, err
	}

	return Location{
		UserID:     userID,
		Lat:        update.Lat,
		Lng:        update.Lng,
		Speed:      update.Speed,
		Heading:    update.Heading,
		Accuracy:   update.Accuracy,
		RecordedAt: recordedAt.UTC().Format(time.RFC3339Nano),
	}, nil
}

func (s *Service) History(
	ctx context.Context,
	userID string,
	from time.Time,
	to time.Time,
	limit int,
) (HistoryResult, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	var result HistoryResult
	err := s.store.Postgres.QueryRow(ctx, `
		SELECT id, username, full_name, role, status
		FROM users
		WHERE id = $1
	`, userID).Scan(
		&result.User.ID,
		&result.User.Username,
		&result.User.FullName,
		&result.User.Role,
		&result.User.Status,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return HistoryResult{}, ErrUserNotFound
	}
	if err != nil {
		return HistoryResult{}, err
	}

	rows, err := s.store.Postgres.Query(ctx, `
		SELECT lat, lng, speed, heading, accuracy, recorded_at
		FROM gps_logs
		WHERE user_id = $1
		  AND recorded_at >= $2
		  AND recorded_at <= $3
		ORDER BY recorded_at DESC
		LIMIT $4
	`, userID, from, to, limit)
	if err != nil {
		return HistoryResult{}, err
	}
	defer rows.Close()

	result.Items = make([]Location, 0)
	for rows.Next() {
		var location Location
		var recordedAt time.Time
		location.UserID = userID
		if err := rows.Scan(
			&location.Lat,
			&location.Lng,
			&location.Speed,
			&location.Heading,
			&location.Accuracy,
			&recordedAt,
		); err != nil {
			return HistoryResult{}, err
		}
		location.RecordedAt = recordedAt.UTC().Format(time.RFC3339Nano)
		result.Items = append(result.Items, location)
	}
	if err := rows.Err(); err != nil {
		return HistoryResult{}, err
	}
	return result, nil
}

func validate(update Update) error {
	if math.IsNaN(update.Lat) || math.IsInf(update.Lat, 0) || update.Lat < -90 || update.Lat > 90 {
		return ErrInvalidLocation
	}
	if math.IsNaN(update.Lng) || math.IsInf(update.Lng, 0) || update.Lng < -180 || update.Lng > 180 {
		return ErrInvalidLocation
	}
	if update.Speed != nil && (math.IsNaN(*update.Speed) || math.IsInf(*update.Speed, 0) || *update.Speed < 0) {
		return ErrInvalidLocation
	}
	if update.Heading != nil && (math.IsNaN(*update.Heading) || math.IsInf(*update.Heading, 0) || *update.Heading < 0 || *update.Heading >= 360) {
		return ErrInvalidLocation
	}
	if update.Accuracy != nil && (math.IsNaN(*update.Accuracy) || math.IsInf(*update.Accuracy, 0) || *update.Accuracy < 0) {
		return ErrInvalidLocation
	}
	return nil
}

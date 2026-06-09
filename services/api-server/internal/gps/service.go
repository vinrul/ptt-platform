package gps

import (
	"context"
	"errors"
	"math"
	"time"

	"ptt-fleet/services/api-server/internal/db"
)

var ErrInvalidLocation = errors.New("invalid GPS location")

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

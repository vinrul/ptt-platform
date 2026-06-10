package devices

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"

	"ptt-fleet/services/api-server/internal/db"
)

var ErrNotFound = errors.New("device not found")

type Device struct {
	ID         string     `json:"id"`
	UserID     string     `json:"userId"`
	Username   string     `json:"username"`
	FullName   string     `json:"fullName"`
	DeviceName string     `json:"deviceName"`
	DeviceIMEI *string    `json:"deviceImei,omitempty"`
	Platform   string     `json:"platform"`
	Status     string     `json:"status"`
	PushToken  *string    `json:"pushToken,omitempty"`
	LastSeenAt *time.Time `json:"lastSeenAt,omitempty"`
	CreatedAt  time.Time  `json:"createdAt"`
}

type Service struct {
	store *db.Store
}

func NewService(store *db.Store) *Service {
	return &Service{store: store}
}

func (s *Service) List(ctx context.Context) ([]Device, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	rows, err := s.store.Postgres.Query(ctx, `
		SELECT d.id, d.user_id, u.username, u.full_name, d.device_name,
		       d.device_imei, d.platform, d.status, d.push_token, d.last_seen_at, d.created_at
		FROM devices d
		JOIN users u ON u.id = d.user_id
		ORDER BY d.last_seen_at DESC NULLS LAST, d.created_at DESC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]Device, 0)
	for rows.Next() {
		var device Device
		if err := rows.Scan(
			&device.ID,
			&device.UserID,
			&device.Username,
			&device.FullName,
			&device.DeviceName,
			&device.DeviceIMEI,
			&device.Platform,
			&device.Status,
			&device.PushToken,
			&device.LastSeenAt,
			&device.CreatedAt,
		); err != nil {
			return nil, err
		}
		items = append(items, device)
	}
	return items, rows.Err()
}

func (s *Service) Get(ctx context.Context, deviceID string) (Device, error) {
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()

	var device Device
	err := s.store.Postgres.QueryRow(ctx, `
		SELECT d.id, d.user_id, u.username, u.full_name, d.device_name,
		       d.device_imei, d.platform, d.status, d.push_token, d.last_seen_at, d.created_at
		FROM devices d
		JOIN users u ON u.id = d.user_id
		WHERE d.id = $1
	`, deviceID).Scan(
		&device.ID,
		&device.UserID,
		&device.Username,
		&device.FullName,
		&device.DeviceName,
		&device.DeviceIMEI,
		&device.Platform,
		&device.Status,
		&device.PushToken,
		&device.LastSeenAt,
		&device.CreatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return Device{}, ErrNotFound
	}
	return device, err
}

func (s *Service) UpdatePushToken(ctx context.Context, deviceID string, token string) error {
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()

	var nullableToken any
	if token != "" {
		nullableToken = token
	}

	res, err := s.store.Postgres.Exec(ctx, `
		UPDATE devices
		SET push_token = $1
		WHERE id = $2
	`, nullableToken, deviceID)
	if err != nil {
		return err
	}
	if res.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

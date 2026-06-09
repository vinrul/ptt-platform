package audit

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"ptt-fleet/services/api-server/internal/db"
)

type Log struct {
	ID            int64           `json:"id"`
	ActorUserID   *string         `json:"actorUserId,omitempty"`
	ActorUsername *string         `json:"actorUsername,omitempty"`
	Action        string          `json:"action"`
	EntityType    string          `json:"entityType"`
	EntityID      *string         `json:"entityId,omitempty"`
	Metadata      json.RawMessage `json:"metadata"`
	CreatedAt     time.Time       `json:"createdAt"`
}

type ListInput struct {
	Page        int
	PageSize    int
	Action      string
	ActorUserID string
}

type ListResult struct {
	Items    []Log `json:"items"`
	Page     int   `json:"page"`
	PageSize int   `json:"pageSize"`
	Total    int64 `json:"total"`
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

	where, args := auditFilters(input)
	var total int64
	if err := s.store.Postgres.QueryRow(ctx, "SELECT count(*) FROM audit_logs a"+where, args...).Scan(&total); err != nil {
		return ListResult{}, err
	}

	queryArgs := append(args, input.PageSize, (input.Page-1)*input.PageSize)
	rows, err := s.store.Postgres.Query(ctx, `
		SELECT a.id, a.actor_user_id, u.username, a.action, a.entity_type,
		       a.entity_id, a.metadata, a.created_at
		FROM audit_logs a
		LEFT JOIN users u ON u.id = a.actor_user_id`+where+fmt.Sprintf(`
		ORDER BY a.created_at DESC
		LIMIT $%d OFFSET $%d
	`, len(args)+1, len(args)+2), queryArgs...)
	if err != nil {
		return ListResult{}, err
	}
	defer rows.Close()

	items := make([]Log, 0)
	for rows.Next() {
		var log Log
		if err := rows.Scan(
			&log.ID,
			&log.ActorUserID,
			&log.ActorUsername,
			&log.Action,
			&log.EntityType,
			&log.EntityID,
			&log.Metadata,
			&log.CreatedAt,
		); err != nil {
			return ListResult{}, err
		}
		items = append(items, log)
	}
	if err := rows.Err(); err != nil {
		return ListResult{}, err
	}
	return ListResult{Items: items, Page: input.Page, PageSize: input.PageSize, Total: total}, nil
}

func auditFilters(input ListInput) (string, []any) {
	conditions := make([]string, 0, 2)
	args := make([]any, 0, 2)
	if input.Action != "" {
		args = append(args, input.Action)
		conditions = append(conditions, fmt.Sprintf("a.action = $%d", len(args)))
	}
	if input.ActorUserID != "" {
		args = append(args, input.ActorUserID)
		conditions = append(conditions, fmt.Sprintf("a.actor_user_id = $%d", len(args)))
	}
	if len(conditions) == 0 {
		return "", args
	}
	return " WHERE " + strings.Join(conditions, " AND "), args
}

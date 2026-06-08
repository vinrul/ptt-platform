package db

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
)

type Store struct {
	Postgres *pgxpool.Pool
	Redis    *redis.Client
}

func New(ctx context.Context, databaseURL string, redisURL string) (*Store, error) {
	pgPool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		return nil, err
	}

	redisOptions, err := redis.ParseURL(redisURL)
	if err != nil {
		pgPool.Close()
		return nil, err
	}

	store := &Store{
		Postgres: pgPool,
		Redis:    redis.NewClient(redisOptions),
	}

	return store, nil
}

func (s *Store) Close() {
	if s == nil {
		return
	}
	if s.Redis != nil {
		_ = s.Redis.Close()
	}
	if s.Postgres != nil {
		s.Postgres.Close()
	}
}

func (s *Store) Ready(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()

	if err := s.Postgres.Ping(ctx); err != nil {
		return err
	}

	if err := s.Redis.Ping(ctx).Err(); err != nil {
		return err
	}

	return nil
}

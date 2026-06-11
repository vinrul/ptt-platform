package main

import (
	"context"
	"flag"
	"log"
	"os"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"ptt-fleet/services/api-server/internal/auth"
	"ptt-fleet/services/api-server/internal/config"
)

const (
	superuserUsername = "superuser"
	superuserFullName = "Super User"
)

func main() {
	password := flag.String("password", os.Getenv("SUPERUSER_PASSWORD"), "password for the superuser account")
	flag.Parse()

	if len(*password) < 8 {
		log.Fatal("SUPERUSER_PASSWORD or -password must contain at least 8 characters")
	}

	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("load config: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	pool, err := pgxpool.New(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("connect database: %v", err)
	}
	defer pool.Close()

	passwordHash, err := auth.HashPassword(*password)
	if err != nil {
		log.Fatalf("hash password: %v", err)
	}

	tx, err := pool.Begin(ctx)
	if err != nil {
		log.Fatalf("begin transaction: %v", err)
	}
	defer tx.Rollback(ctx)

	var userID string
	err = tx.QueryRow(ctx, `
		INSERT INTO users (username, password_hash, full_name, role, status)
		VALUES ($1, $2, $3, 'super_admin', 'active')
		ON CONFLICT (username) DO UPDATE
		SET password_hash = EXCLUDED.password_hash,
		    full_name = EXCLUDED.full_name,
		    role = 'super_admin',
		    status = 'active',
		    updated_at = now()
		RETURNING id
	`, superuserUsername, passwordHash, superuserFullName).Scan(&userID)
	if err != nil {
		log.Fatalf("seed superuser: %v", err)
	}

	if _, err := tx.Exec(ctx, `
		UPDATE refresh_tokens
		SET revoked_at = now()
		WHERE user_id = $1 AND revoked_at IS NULL
	`, userID); err != nil {
		log.Fatalf("revoke existing superuser sessions: %v", err)
	}

	if err := tx.Commit(ctx); err != nil {
		log.Fatalf("commit superuser seed: %v", err)
	}

	log.Printf("superuser account is ready with role super_admin")
}

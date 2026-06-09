package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"ptt-fleet/services/api-server/internal/auth"
	"ptt-fleet/services/api-server/internal/config"
)

type seedUser struct {
	Username string
	FullName string
	Role     string
}

var localUsers = []seedUser{
	{Username: "admin", FullName: "Local Admin", Role: "super_admin"},
	{Username: "dispatcher", FullName: "Local Dispatcher", Role: "dispatcher"},
	{Username: "field1", FullName: "Field User One", Role: "field_user"},
	{Username: "field2", FullName: "Field User Two", Role: "field_user"},
}

func main() {
	password := flag.String("password", envOrDefault("SEED_PASSWORD", "ptt-local-123"), "password for local seed users")
	flag.Parse()

	if len(*password) < 8 {
		log.Fatal("seed password must contain at least 8 characters")
	}

	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("load config: %v", err)
	}
	if cfg.AppEnv != "local" && os.Getenv("ALLOW_NON_LOCAL_SEED") != "true" {
		log.Fatal("seed command only runs in APP_ENV=local unless ALLOW_NON_LOCAL_SEED=true")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	pool, err := pgxpool.New(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("connect database: %v", err)
	}
	defer pool.Close()

	hash, err := auth.HashPassword(*password)
	if err != nil {
		log.Fatalf("hash password: %v", err)
	}

	tx, err := pool.Begin(ctx)
	if err != nil {
		log.Fatalf("begin transaction: %v", err)
	}
	defer tx.Rollback(ctx)

	userIDs := make(map[string]string, len(localUsers))
	for _, user := range localUsers {
		var userID string
		err := tx.QueryRow(ctx, `
			INSERT INTO users (username, password_hash, full_name, role, status)
			VALUES ($1, $2, $3, $4, 'active')
			ON CONFLICT (username) DO UPDATE
			SET username = EXCLUDED.username,
			    password_hash = EXCLUDED.password_hash,
			    full_name = EXCLUDED.full_name,
			    role = EXCLUDED.role,
			    status = 'active',
			    updated_at = now()
			RETURNING id
		`, user.Username, hash, user.FullName, user.Role).Scan(&userID)
		if err != nil {
			log.Fatalf("seed user %s: %v", user.Username, err)
		}
		userIDs[user.Username] = userID
	}

	var groupID string
	err = tx.QueryRow(ctx, `
		INSERT INTO groups (name, description)
		VALUES ('Default Patrol', 'Default local PTT test group')
		ON CONFLICT (name) DO UPDATE
		SET name = EXCLUDED.name,
		    description = EXCLUDED.description,
		    updated_at = now()
		RETURNING id
	`).Scan(&groupID)
	if err != nil {
		log.Fatalf("seed group: %v", err)
	}

	for _, user := range localUsers {
		roleInGroup := "member"
		if user.Role == "dispatcher" || user.Role == "super_admin" {
			roleInGroup = "dispatcher"
		}
		_, err := tx.Exec(ctx, `
			INSERT INTO group_members (group_id, user_id, role_in_group)
			VALUES ($1, $2, $3)
			ON CONFLICT (group_id, user_id) DO UPDATE
			SET role_in_group = EXCLUDED.role_in_group
		`, groupID, userIDs[user.Username], roleInGroup)
		if err != nil {
			log.Fatalf("seed group member %s: %v", user.Username, err)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		log.Fatalf("commit seed: %v", err)
	}

	fmt.Printf("Local seed ready. Password for admin, dispatcher, field1, field2: %s\n", *password)
}

func envOrDefault(key string, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

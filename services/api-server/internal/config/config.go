package config

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"time"
)

type Config struct {
	AppEnv              string
	APIPort             string
	DatabaseURL         string
	RedisURL            string
	JWTSecret           string
	JWTAccessTTLMinutes int
	JWTRefreshTTLHours  int
}

func Load() (Config, error) {
	appEnv := getEnv("APP_ENV", "local")

	cfg := Config{
		AppEnv:              appEnv,
		APIPort:             getEnv("API_PORT", "8080"),
		DatabaseURL:         getEnv("DATABASE_URL", defaultDatabaseURL(appEnv)),
		RedisURL:            getEnv("REDIS_URL", defaultRedisURL(appEnv)),
		JWTSecret:           getEnv("JWT_SECRET", defaultJWTSecret(appEnv)),
		JWTAccessTTLMinutes: getEnvInt("JWT_ACCESS_TTL_MINUTES", 15),
		JWTRefreshTTLHours:  getEnvInt("JWT_REFRESH_TTL_HOURS", 720),
	}

	if err := cfg.Validate(); err != nil {
		return Config{}, err
	}

	return cfg, nil
}

func (c Config) Validate() error {
	if c.DatabaseURL == "" {
		return errors.New("DATABASE_URL is required")
	}
	if c.RedisURL == "" {
		return errors.New("REDIS_URL is required")
	}
	if c.JWTSecret == "" {
		return errors.New("JWT_SECRET is required")
	}
	if len(c.JWTSecret) < 32 {
		return errors.New("JWT_SECRET must be at least 32 characters")
	}
	if c.APIPort == "" {
		return errors.New("API_PORT is required")
	}
	if _, err := strconv.Atoi(c.APIPort); err != nil {
		return fmt.Errorf("API_PORT must be numeric: %w", err)
	}
	return nil
}

func defaultDatabaseURL(appEnv string) string {
	if appEnv == "production" {
		return ""
	}
	return "postgres://ptt:ptt@localhost:5432/ptt_fleet?sslmode=disable"
}

func defaultRedisURL(appEnv string) string {
	if appEnv == "production" {
		return ""
	}
	return "redis://localhost:6379"
}

func defaultJWTSecret(appEnv string) string {
	if appEnv == "production" {
		return ""
	}
	return "local-development-jwt-secret-32-bytes"
}

func (c Config) Addr() string {
	return ":" + c.APIPort
}

func (c Config) AccessTTL() time.Duration {
	return time.Duration(c.JWTAccessTTLMinutes) * time.Minute
}

func (c Config) RefreshTTL() time.Duration {
	return time.Duration(c.JWTRefreshTTLHours) * time.Hour
}

func getEnv(key string, fallback string) string {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	return value
}

func getEnvInt(key string, fallback int) int {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}

	parsed, err := strconv.Atoi(value)
	if err != nil {
		return fallback
	}

	return parsed
}

package config

import (
	"testing"
	"time"
)

func TestLoadRequiresDatabaseURLInProduction(t *testing.T) {
	t.Setenv("APP_ENV", "production")
	t.Setenv("DATABASE_URL", "")
	t.Setenv("REDIS_URL", "redis://localhost:6379")
	t.Setenv("JWT_SECRET", "12345678901234567890123456789012")

	if _, err := Load(); err == nil {
		t.Fatal("expected DATABASE_URL validation error")
	}
}

func TestLoadUsesLocalDefaults(t *testing.T) {
	t.Setenv("APP_ENV", "local")
	t.Setenv("DATABASE_URL", "")
	t.Setenv("REDIS_URL", "")
	t.Setenv("JWT_SECRET", "")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("expected local defaults to load: %v", err)
	}

	if cfg.DatabaseURL == "" {
		t.Fatal("expected default database url")
	}
	if cfg.RedisURL == "" {
		t.Fatal("expected default redis url")
	}
}

func TestLoadWithRequiredEnv(t *testing.T) {
	t.Setenv("APP_ENV", "test")
	t.Setenv("API_PORT", "9090")
	t.Setenv("DATABASE_URL", "postgres://ptt:ptt@localhost:5432/ptt_fleet?sslmode=disable")
	t.Setenv("REDIS_URL", "redis://localhost:6379")
	t.Setenv("JWT_SECRET", "12345678901234567890123456789012")
	t.Setenv("JWT_ACCESS_TTL_MINUTES", "30")
	t.Setenv("JWT_REFRESH_TTL_HOURS", "24")
	t.Setenv("CORS_ALLOWED_ORIGINS", "https://ptt.example.com, https://dispatcher.example.com/")
	t.Setenv("TRUSTED_PROXIES", "127.0.0.1,172.16.0.0/12")
	t.Setenv("LOGIN_RATE_LIMIT", "5")
	t.Setenv("LOGIN_RATE_WINDOW_SECONDS", "120")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("expected config to load: %v", err)
	}

	if cfg.Addr() != ":9090" {
		t.Fatalf("expected addr :9090, got %s", cfg.Addr())
	}
	if cfg.JWTAccessTTLMinutes != 30 {
		t.Fatalf("expected access ttl 30, got %d", cfg.JWTAccessTTLMinutes)
	}
	if cfg.JWTRefreshTTLHours != 24 {
		t.Fatalf("expected refresh ttl 24, got %d", cfg.JWTRefreshTTLHours)
	}
	if len(cfg.AllowedOrigins) != 2 || cfg.AllowedOrigins[1] != "https://dispatcher.example.com" {
		t.Fatalf("unexpected allowed origins: %#v", cfg.AllowedOrigins)
	}
	if cfg.LoginRateLimit != 5 || cfg.LoginRateWindow != 2*time.Minute {
		t.Fatalf("unexpected login rate config: %d %s", cfg.LoginRateLimit, cfg.LoginRateWindow)
	}
}

func TestProductionRequiresHTTPSOriginsAndTrustedProxies(t *testing.T) {
	cfg := Config{
		AppEnv:          "production",
		APIPort:         "8080",
		DatabaseURL:     "postgres://example",
		RedisURL:        "redis://example",
		JWTSecret:       "12345678901234567890123456789012",
		AllowedOrigins:  []string{"http://ptt.example.com"},
		TrustedProxies:  []string{"172.16.0.0/12"},
		LoginRateLimit:  10,
		LoginRateWindow: time.Minute,
	}

	if err := cfg.Validate(); err == nil {
		t.Fatal("expected non-HTTPS production origin to fail")
	}

	cfg.AllowedOrigins = []string{"https://ptt.example.com"}
	cfg.TrustedProxies = nil
	if err := cfg.Validate(); err == nil {
		t.Fatal("expected missing trusted proxies to fail")
	}
}

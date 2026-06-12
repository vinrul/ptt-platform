package config

import (
	"errors"
	"fmt"
	"net"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	AppEnv                  string
	APIPort                 string
	DatabaseURL             string
	RedisURL                string
	JWTSecret               string
	JWTAccessTTLMinutes     int
	JWTRefreshTTLHours      int
	AllowedOrigins          []string
	TrustedProxies          []string
	LoginRateLimit          int
	LoginRateWindow         time.Duration
	FirebaseCredentialsPath string
	ReverseGeocodeURL       string
	RouteServiceURL         string
}

func Load() (Config, error) {
	appEnv := getEnv("APP_ENV", "local")

	cfg := Config{
		AppEnv:                  appEnv,
		APIPort:                 getEnv("API_PORT", "8080"),
		DatabaseURL:             getEnv("DATABASE_URL", defaultDatabaseURL(appEnv)),
		RedisURL:                getEnv("REDIS_URL", defaultRedisURL(appEnv)),
		JWTSecret:               getEnv("JWT_SECRET", defaultJWTSecret(appEnv)),
		JWTAccessTTLMinutes:     getEnvInt("JWT_ACCESS_TTL_MINUTES", 15),
		JWTRefreshTTLHours:      getEnvInt("JWT_REFRESH_TTL_HOURS", 720),
		AllowedOrigins:          getEnvList("CORS_ALLOWED_ORIGINS", defaultAllowedOrigins(appEnv)),
		TrustedProxies:          getEnvList("TRUSTED_PROXIES", defaultTrustedProxies(appEnv)),
		LoginRateLimit:          getEnvInt("LOGIN_RATE_LIMIT", 10),
		LoginRateWindow:         time.Duration(getEnvInt("LOGIN_RATE_WINDOW_SECONDS", 60)) * time.Second,
		FirebaseCredentialsPath: getEnv("FIREBASE_CREDENTIALS_PATH", ""),
		ReverseGeocodeURL:       getEnv("REVERSE_GEOCODE_URL", "https://nominatim.openstreetmap.org/reverse"),
		RouteServiceURL:         getEnv("ROUTE_SERVICE_URL", "https://route.vinrul.my.id"),
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
	if c.LoginRateLimit < 1 {
		return errors.New("LOGIN_RATE_LIMIT must be at least 1")
	}
	if c.LoginRateWindow < time.Second {
		return errors.New("LOGIN_RATE_WINDOW_SECONDS must be at least 1")
	}
	if c.AppEnv == "production" && len(c.AllowedOrigins) == 0 {
		return errors.New("CORS_ALLOWED_ORIGINS is required in production")
	}
	if c.AppEnv == "production" && len(c.TrustedProxies) == 0 {
		return errors.New("TRUSTED_PROXIES is required in production")
	}
	if err := validateHTTPURL("REVERSE_GEOCODE_URL", c.ReverseGeocodeURL); err != nil {
		return err
	}
	if err := validateHTTPURL("ROUTE_SERVICE_URL", c.RouteServiceURL); err != nil {
		return err
	}
	for _, origin := range c.AllowedOrigins {
		parsed, err := url.Parse(origin)
		if err != nil ||
			(parsed.Scheme != "http" && parsed.Scheme != "https") ||
			parsed.Host == "" ||
			parsed.Path != "" ||
			parsed.RawQuery != "" ||
			parsed.Fragment != "" ||
			parsed.User != nil {
			return fmt.Errorf("CORS_ALLOWED_ORIGINS contains invalid origin %q", origin)
		}
		if c.AppEnv == "production" && parsed.Scheme != "https" {
			return fmt.Errorf("production origin must use https: %s", origin)
		}
	}
	for _, proxy := range c.TrustedProxies {
		if net.ParseIP(proxy) == nil {
			if _, _, err := net.ParseCIDR(proxy); err != nil {
				return fmt.Errorf("TRUSTED_PROXIES contains invalid IP or CIDR %q", proxy)
			}
		}
	}
	return nil
}

func validateHTTPURL(name string, value string) error {
	parsed, err := url.Parse(value)
	if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" {
		return fmt.Errorf("%s must be an http(s) URL", name)
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

func defaultAllowedOrigins(appEnv string) string {
	if appEnv == "production" {
		return ""
	}
	return "http://localhost:5173,http://127.0.0.1:5173"
}

func defaultTrustedProxies(appEnv string) string {
	if appEnv == "production" {
		return ""
	}
	return "127.0.0.1,::1"
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

func getEnvList(key string, fallback string) []string {
	value := getEnv(key, fallback)
	if strings.TrimSpace(value) == "" {
		return nil
	}

	items := strings.Split(value, ",")
	result := make([]string, 0, len(items))
	for _, item := range items {
		if trimmed := strings.TrimSpace(item); trimmed != "" {
			result = append(result, strings.TrimSuffix(trimmed, "/"))
		}
	}
	return result
}

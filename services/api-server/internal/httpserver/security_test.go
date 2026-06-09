package httpserver

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
)

type fakeLoginRateLimiter struct {
	allowed bool
	err     error
	key     string
}

func (l *fakeLoginRateLimiter) Allow(_ context.Context, key string) (bool, error) {
	l.key = key
	return l.allowed, l.err
}

func TestLoginRateLimitBlocksExcessAttempts(t *testing.T) {
	gin.SetMode(gin.TestMode)
	limiter := &fakeLoginRateLimiter{allowed: false}
	router := gin.New()
	router.POST("/login", loginRateLimit(limiter, 90*time.Second), func(c *gin.Context) {
		c.Status(http.StatusNoContent)
	})

	request := httptest.NewRequest(http.MethodPost, "/login", strings.NewReader(`{"username":" Admin "}`))
	request.Header.Set("Content-Type", "application/json")
	request.RemoteAddr = "192.0.2.10:1234"
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)

	if response.Code != http.StatusTooManyRequests {
		t.Fatalf("expected status 429, got %d", response.Code)
	}
	if response.Header().Get("Retry-After") != "90" {
		t.Fatalf("expected Retry-After 90, got %q", response.Header().Get("Retry-After"))
	}
	if limiter.key == "" {
		t.Fatal("expected a rate limit key")
	}
}

func TestLoginRateLimitFailsOpenWhenRedisUnavailable(t *testing.T) {
	gin.SetMode(gin.TestMode)
	limiter := &fakeLoginRateLimiter{err: errors.New("redis unavailable")}
	router := gin.New()
	router.POST("/login", loginRateLimit(limiter, time.Minute), func(c *gin.Context) {
		var payload struct {
			Username string `json:"username"`
		}
		if err := c.ShouldBindJSON(&payload); err != nil {
			t.Fatalf("expected request body to remain readable: %v", err)
		}
		c.Status(http.StatusNoContent)
	})

	request := httptest.NewRequest(http.MethodPost, "/login", strings.NewReader(`{"username":"admin"}`))
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)

	if response.Code != http.StatusNoContent {
		t.Fatalf("expected status 204, got %d", response.Code)
	}
}

func TestCORSAllowsConfiguredOrigin(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(cors([]string{"https://ptt.example.com"}))
	router.GET("/api/version", func(c *gin.Context) {
		c.Status(http.StatusNoContent)
	})

	request := httptest.NewRequest(http.MethodGet, "/api/version", nil)
	request.Header.Set("Origin", "https://ptt.example.com")
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)

	if response.Code != http.StatusNoContent {
		t.Fatalf("expected status 204, got %d", response.Code)
	}
	if response.Header().Get("Access-Control-Allow-Origin") != "https://ptt.example.com" {
		t.Fatal("expected configured CORS origin")
	}
}

func TestCORSRejectsUnknownOrigin(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(cors([]string{"https://ptt.example.com"}))
	router.GET("/api/version", func(c *gin.Context) {
		c.Status(http.StatusNoContent)
	})

	request := httptest.NewRequest(http.MethodGet, "/api/version", nil)
	request.Header.Set("Origin", "https://evil.example")
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)

	if response.Code != http.StatusForbidden {
		t.Fatalf("expected status 403, got %d", response.Code)
	}
}

func TestProductionRequiresHTTPS(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(requireHTTPS("production"))
	router.GET("/api/version", func(c *gin.Context) {
		c.Status(http.StatusNoContent)
	})

	request := httptest.NewRequest(http.MethodGet, "/api/version", nil)
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)

	if response.Code != http.StatusUpgradeRequired {
		t.Fatalf("expected status 426, got %d", response.Code)
	}

	request = httptest.NewRequest(http.MethodGet, "/api/version", nil)
	request.Header.Set("X-Forwarded-Proto", "https")
	response = httptest.NewRecorder()
	router.ServeHTTP(response, request)
	if response.Code != http.StatusNoContent {
		t.Fatalf("expected HTTPS request status 204, got %d", response.Code)
	}
}

func TestRequestLoggerDoesNotLogQueryString(t *testing.T) {
	gin.SetMode(gin.TestMode)
	var output bytes.Buffer
	previousWriter := gin.DefaultWriter
	gin.DefaultWriter = &output
	t.Cleanup(func() {
		gin.DefaultWriter = previousWriter
	})

	router := gin.New()
	router.Use(requestLogger())
	router.GET("/ws", func(c *gin.Context) {
		c.Status(http.StatusNoContent)
	})

	request := httptest.NewRequest(http.MethodGet, "/ws?token=secret-jwt", nil)
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)

	if strings.Contains(output.String(), "secret-jwt") || strings.Contains(output.String(), "token=") {
		t.Fatalf("expected query string to be redacted, log=%q", output.String())
	}
	if !strings.Contains(output.String(), "/ws") {
		t.Fatalf("expected request path in log, log=%q", output.String())
	}
}

package httpserver

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"

	"ptt-fleet/services/api-server/internal/auth"
	"ptt-fleet/services/api-server/internal/config"
)

func TestProtectedRouteRequiresAccessToken(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := NewRouter(testConfig(), nil, nil)

	request := httptest.NewRequest(http.MethodGet, "/api/users", nil)
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)

	if response.Code != http.StatusUnauthorized {
		t.Fatalf("expected status 401, got %d", response.Code)
	}
}

func TestLoginRejectsInvalidPayload(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := NewRouter(testConfig(), nil, nil)

	request := httptest.NewRequest(
		http.MethodPost,
		"/api/auth/login",
		strings.NewReader(`{"username":"admin"}`),
	)
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)

	if response.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d", response.Code)
	}
}

func TestChangePasswordRequiresAccessToken(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := NewRouter(testConfig(), nil, nil)

	request := httptest.NewRequest(
		http.MethodPost,
		"/api/auth/change-password",
		strings.NewReader(`{"currentPassword":"old-password","newPassword":"new-password"}`),
	)
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)

	if response.Code != http.StatusUnauthorized {
		t.Fatalf("expected status 401, got %d", response.Code)
	}
}

func TestResetUserPasswordRequiresSuperAdmin(t *testing.T) {
	gin.SetMode(gin.TestMode)
	cfg := testConfig()
	router := NewRouter(cfg, nil, nil)
	token, err := auth.NewTokenManager(cfg.JWTSecret, cfg.AccessTTL()).
		IssueAccessToken("dispatcher-1", "dispatcher", "dispatcher")
	if err != nil {
		t.Fatalf("issue access token: %v", err)
	}

	request := httptest.NewRequest(
		http.MethodPost,
		"/api/users/user-1/reset-password",
		strings.NewReader(`{"newPassword":"new-password"}`),
	)
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Authorization", "Bearer "+token)
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)

	if response.Code != http.StatusForbidden {
		t.Fatalf("expected status 403, got %d", response.Code)
	}
}

func TestGpsHistoryRejectsSupervisor(t *testing.T) {
	gin.SetMode(gin.TestMode)
	cfg := testConfig()
	router := NewRouter(cfg, nil, nil)
	token, err := auth.NewTokenManager(cfg.JWTSecret, cfg.AccessTTL()).
		IssueAccessToken("supervisor-1", "supervisor", "supervisor")
	if err != nil {
		t.Fatalf("issue access token: %v", err)
	}

	request := httptest.NewRequest(http.MethodGet, "/api/users/user-1/gps-history", nil)
	request.Header.Set("Authorization", "Bearer "+token)
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)

	if response.Code != http.StatusForbidden {
		t.Fatalf("expected status 403, got %d", response.Code)
	}
}

func testConfig() config.Config {
	return config.Config{
		AppEnv:              "test",
		JWTSecret:           "12345678901234567890123456789012",
		JWTAccessTTLMinutes: 15,
		JWTRefreshTTLHours:  720,
	}
}

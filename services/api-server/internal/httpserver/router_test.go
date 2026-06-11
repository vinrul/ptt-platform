package httpserver

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"

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

func testConfig() config.Config {
	return config.Config{
		AppEnv:              "test",
		JWTSecret:           "12345678901234567890123456789012",
		JWTAccessTTLMinutes: 15,
		JWTRefreshTTLHours:  720,
	}
}

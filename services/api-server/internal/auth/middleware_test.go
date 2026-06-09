package auth

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
)

func TestMiddlewareRequiresBearerToken(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(Middleware(NewTokenManager("12345678901234567890123456789012", time.Minute)))
	router.GET("/protected", func(c *gin.Context) {
		c.Status(http.StatusNoContent)
	})

	request := httptest.NewRequest(http.MethodGet, "/protected", nil)
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)

	if response.Code != http.StatusUnauthorized {
		t.Fatalf("expected status 401, got %d", response.Code)
	}
}

func TestMiddlewareStoresClaims(t *testing.T) {
	gin.SetMode(gin.TestMode)
	manager := NewTokenManager("12345678901234567890123456789012", time.Minute)
	rawToken, err := manager.IssueAccessToken("user-1", "dispatcher1", "dispatcher")
	if err != nil {
		t.Fatalf("issue token: %v", err)
	}

	router := gin.New()
	router.Use(Middleware(manager))
	router.GET("/protected", func(c *gin.Context) {
		claims, claimsErr := ClaimsFromContext(c)
		if claimsErr != nil {
			c.Status(http.StatusInternalServerError)
			return
		}
		c.JSON(http.StatusOK, gin.H{"userId": claims.Subject})
	})

	request := httptest.NewRequest(http.MethodGet, "/protected", nil)
	request.Header.Set("Authorization", "Bearer "+rawToken)
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", response.Code)
	}
}

package auth

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"ptt-fleet/services/api-server/internal/apiutil"
)

func Middleware(tokens *TokenManager) gin.HandlerFunc {
	return func(c *gin.Context) {
		header := c.GetHeader("Authorization")
		scheme, rawToken, found := strings.Cut(header, " ")
		if !found || !strings.EqualFold(scheme, "Bearer") || strings.TrimSpace(rawToken) == "" {
			apiutil.Error(c, http.StatusUnauthorized, "unauthorized", "Bearer access token is required", nil)
			return
		}

		claims, err := tokens.ParseAccessToken(strings.TrimSpace(rawToken))
		if err != nil {
			apiutil.Error(c, http.StatusUnauthorized, "unauthorized", "Access token is invalid or expired", nil)
			return
		}

		SetClaims(c, claims)
		c.Next()
	}
}

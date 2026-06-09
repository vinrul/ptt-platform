package auth

import (
	"errors"

	"github.com/gin-gonic/gin"
)

const claimsContextKey = "auth.claims"

var ErrMissingClaims = errors.New("authentication claims missing")

func SetClaims(c *gin.Context, claims Claims) {
	c.Set(claimsContextKey, claims)
}

func ClaimsFromContext(c *gin.Context) (Claims, error) {
	value, exists := c.Get(claimsContextKey)
	if !exists {
		return Claims{}, ErrMissingClaims
	}

	claims, ok := value.(Claims)
	if !ok {
		return Claims{}, ErrMissingClaims
	}

	return claims, nil
}

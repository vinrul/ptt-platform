package auth

import (
	"crypto/rand"
	"encoding/base64"
	"errors"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

var ErrInvalidAccessToken = errors.New("invalid access token")

type Claims struct {
	Username string `json:"username"`
	Role     string `json:"role"`
	jwt.RegisteredClaims
}

type TokenManager struct {
	secret    []byte
	accessTTL time.Duration
}

func NewTokenManager(secret string, accessTTL time.Duration) *TokenManager {
	return &TokenManager{
		secret:    []byte(secret),
		accessTTL: accessTTL,
	}
}

func (m *TokenManager) IssueAccessToken(userID string, username string, role string) (string, error) {
	now := time.Now().UTC()
	tokenID, err := randomToken(16)
	if err != nil {
		return "", err
	}

	claims := Claims{
		Username: username,
		Role:     role,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   userID,
			ID:        tokenID,
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(m.accessTTL)),
		},
	}

	return jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(m.secret)
}

func (m *TokenManager) ParseAccessToken(rawToken string) (Claims, error) {
	token, err := jwt.ParseWithClaims(rawToken, &Claims{}, func(token *jwt.Token) (any, error) {
		if token.Method != jwt.SigningMethodHS256 {
			return nil, ErrInvalidAccessToken
		}
		return m.secret, nil
	})
	if err != nil || !token.Valid {
		return Claims{}, ErrInvalidAccessToken
	}

	claims, ok := token.Claims.(*Claims)
	if !ok || claims.Subject == "" || claims.Role == "" {
		return Claims{}, ErrInvalidAccessToken
	}

	return *claims, nil
}

func randomToken(size int) (string, error) {
	value := make([]byte, size)
	if _, err := rand.Read(value); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(value), nil
}

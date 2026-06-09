package auth

import (
	"testing"
	"time"
)

func TestTokenManagerIssueAndParse(t *testing.T) {
	manager := NewTokenManager("12345678901234567890123456789012", time.Minute)

	rawToken, err := manager.IssueAccessToken("user-1", "field1", "field_user")
	if err != nil {
		t.Fatalf("issue token: %v", err)
	}

	claims, err := manager.ParseAccessToken(rawToken)
	if err != nil {
		t.Fatalf("parse token: %v", err)
	}
	if claims.Subject != "user-1" {
		t.Fatalf("expected user-1 subject, got %s", claims.Subject)
	}
	if claims.Username != "field1" {
		t.Fatalf("expected field1 username, got %s", claims.Username)
	}
	if claims.Role != "field_user" {
		t.Fatalf("expected field_user role, got %s", claims.Role)
	}
}

func TestTokenManagerRejectsWrongSecret(t *testing.T) {
	issuer := NewTokenManager("12345678901234567890123456789012", time.Minute)
	validator := NewTokenManager("abcdefghijklmnopqrstuvwxyz123456", time.Minute)

	rawToken, err := issuer.IssueAccessToken("user-1", "field1", "field_user")
	if err != nil {
		t.Fatalf("issue token: %v", err)
	}
	if _, err := validator.ParseAccessToken(rawToken); err == nil {
		t.Fatal("expected token signed with another secret to be rejected")
	}
}

func TestTokenManagerRejectsExpiredToken(t *testing.T) {
	manager := NewTokenManager("12345678901234567890123456789012", -time.Minute)

	rawToken, err := manager.IssueAccessToken("user-1", "field1", "field_user")
	if err != nil {
		t.Fatalf("issue token: %v", err)
	}
	if _, err := manager.ParseAccessToken(rawToken); err == nil {
		t.Fatal("expected expired token to be rejected")
	}
}

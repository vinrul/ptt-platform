package auth

import "testing"

func TestHashAndCheckPassword(t *testing.T) {
	hash, err := HashPassword("strong-password")
	if err != nil {
		t.Fatalf("hash password: %v", err)
	}
	if hash == "strong-password" {
		t.Fatal("password hash must not equal plaintext password")
	}
	if err := CheckPassword(hash, "strong-password"); err != nil {
		t.Fatalf("check password: %v", err)
	}
	if err := CheckPassword(hash, "wrong-password"); err == nil {
		t.Fatal("expected wrong password to be rejected")
	}
}

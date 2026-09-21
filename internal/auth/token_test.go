package auth

import (
	"testing"
	"time"
)

func TestTokenRoundTrip(t *testing.T) {
	secret := "01234567890123456789012345678901"
	token, err := IssueToken(secret, "user-1", "user@example.com", time.Minute)
	if err != nil {
		t.Fatalf("issue token: %v", err)
	}
	claims, err := ParseToken(secret, token)
	if err != nil {
		t.Fatalf("parse token: %v", err)
	}
	if claims.Subject != "user-1" {
		t.Fatalf("subject = %q", claims.Subject)
	}
}

func TestTokenRejectsWrongSecret(t *testing.T) {
	token, _ := IssueToken("01234567890123456789012345678901", "user-1", "user@example.com", time.Minute)
	if _, err := ParseToken("wrong-secret-that-is-long-enough-123", token); err == nil {
		t.Fatal("expected wrong secret to be rejected")
	}
}

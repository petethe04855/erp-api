package jwt_test

import (
	"testing"

	"chawy-erp-api/pkg/jwt"
)

func TestJWT_GenerateAndValidate(t *testing.T) {
	secret := "my-secret-test-key"
	userID := uint(42)
	email := "admin@example.com"
	role := "admin"

	token, err := jwt.GenerateToken(userID, email, role, secret, 1)
	if err != nil {
		t.Fatalf("GenerateToken failed: %v", err)
	}

	if token == "" {
		t.Fatalf("expected non-empty token")
	}

	claims, err := jwt.ValidateToken(token, secret)
	if err != nil {
		t.Fatalf("ValidateToken failed: %v", err)
	}

	if claims.UserID != userID {
		t.Errorf("expected userID %d, got %d", userID, claims.UserID)
	}
	if claims.Email != email {
		t.Errorf("expected email %s, got %s", email, claims.Email)
	}
	if claims.Role != role {
		t.Errorf("expected role %s, got %s", role, claims.Role)
	}
}

func TestJWT_InvalidSecret(t *testing.T) {
	token, _ := jwt.GenerateToken(1, "test@test.com", "user", "secret1", 1)
	_, err := jwt.ValidateToken(token, "wrong-secret")
	if err == nil {
		t.Errorf("expected error when validating with wrong secret")
	}
}

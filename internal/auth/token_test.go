package auth

import (
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

func sign(t *testing.T, key interface{}, method jwt.SigningMethod, claims jwt.Claims) string {
	t.Helper()
	tok := jwt.NewWithClaims(method, claims)
	s, err := tok.SignedString(key)
	if err != nil {
		t.Fatalf("could not sign test token: %v", err)
	}
	return s
}

func validClaims(subject string, rooms []string, exp time.Time) Claims {
	return Claims{
		Rooms: rooms,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   subject,
			ExpiresAt: jwt.NewNumericDate(exp),
		},
	}
}

func TestVerifyValidToken(t *testing.T) {
	secret := []byte("test-secret")
	claims := validClaims("Mina", []string{"dev", "ops"}, time.Now().Add(time.Hour))
	tokenStr := sign(t, secret, jwt.SigningMethodHS256, claims)

	got, err := Verify(secret, tokenStr)
	if err != nil {
		t.Fatalf("Verify returned error for a valid token: %v", err)
	}
	if got.Subject != "Mina" {
		t.Errorf("Subject = %q, want %q", got.Subject, "Mina")
	}
	if len(got.Rooms) != 2 || got.Rooms[0] != "dev" || got.Rooms[1] != "ops" {
		t.Errorf("Rooms = %v, want [dev ops]", got.Rooms)
	}
}

func TestVerifyExpiredToken(t *testing.T) {
	secret := []byte("test-secret")
	claims := validClaims("Mina", []string{"dev"}, time.Now().Add(-time.Hour))
	tokenStr := sign(t, secret, jwt.SigningMethodHS256, claims)

	if _, err := Verify(secret, tokenStr); err == nil {
		t.Error("Verify did not return an error for an expired token")
	}
}

func TestVerifyWrongSecret(t *testing.T) {
	claims := validClaims("Mina", []string{"dev"}, time.Now().Add(time.Hour))
	tokenStr := sign(t, []byte("right-secret"), jwt.SigningMethodHS256, claims)

	if _, err := Verify([]byte("wrong-secret"), tokenStr); err == nil {
		t.Error("Verify did not return an error for a token signed with a different secret")
	}
}

func TestVerifyRejectsNoneAlgorithm(t *testing.T) {
	claims := validClaims("Mina", []string{"dev"}, time.Now().Add(time.Hour))
	tokenStr := sign(t, jwt.UnsafeAllowNoneSignatureType, jwt.SigningMethodNone, claims)

	if _, err := Verify([]byte("any-secret"), tokenStr); err == nil {
		t.Error("Verify accepted a token signed with alg=none")
	}
}

func TestVerifyRejectsOtherAlgorithm(t *testing.T) {
	secret := []byte("test-secret")
	claims := validClaims("Mina", []string{"dev"}, time.Now().Add(time.Hour))
	tokenStr := sign(t, secret, jwt.SigningMethodHS512, claims)

	if _, err := Verify(secret, tokenStr); err == nil {
		t.Error("Verify accepted a token signed with HS512, expected HS256 only")
	}
}

func TestVerifyMalformedToken(t *testing.T) {
	if _, err := Verify([]byte("secret"), "not-a-jwt"); err == nil {
		t.Error("Verify did not return an error for a malformed token")
	}
}

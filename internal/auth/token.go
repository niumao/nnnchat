// Package auth verifies JWTs presented by chat clients. Tokens are minted by
// an external issuer; this package never signs or issues one, only checks
// that a token is validly signed and reads its claims.
package auth

import (
	"fmt"

	"github.com/golang-jwt/jwt/v5"
)

// Claims is the shape nnnchat expects in a client's JWT: who they are
// (standard "sub" claim) and which rooms they're allowed to join.
type Claims struct {
	Rooms []string `json:"rooms"`
	jwt.RegisteredClaims
}

// Verify checks tokenStr's signature (HS256 only, rejecting "none" and any
// other algorithm so a forged header can't bypass verification) and
// expiry, returning its claims if valid.
func Verify(secret []byte, tokenStr string) (*Claims, error) {
	claims := new(Claims)
	_, err := jwt.ParseWithClaims(tokenStr, claims, func(t *jwt.Token) (interface{}, error) {
		if t.Method.Alg() != jwt.SigningMethodHS256.Alg() {
			return nil, fmt.Errorf("unexpected signing method: %s", t.Method.Alg())
		}
		return secret, nil
	})
	if err != nil {
		return nil, err
	}
	return claims, nil
}

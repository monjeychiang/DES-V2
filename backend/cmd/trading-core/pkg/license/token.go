package license

import (
	"time"

	"github.com/golang-jwt/jwt/v5"
)

type Claims struct {
	Machine string `json:"machine"`
	jwt.RegisteredClaims
}

// CreateToken signs a short-lived token for license verification.
func CreateToken(secret, machine string, ttl time.Duration) (string, error) {
	claims := Claims{
		Machine: machine,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(ttl)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}
	return jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte(secret))
}

// ParseToken verifies the signature and returns claims.
func ParseToken(secret, token string) (*Claims, error) {
	tok, err := jwt.ParseWithClaims(token, &Claims{}, func(t *jwt.Token) (any, error) {
		return []byte(secret), nil
	})
	if err != nil {
		return nil, err
	}
	if claims, ok := tok.Claims.(*Claims); ok && tok.Valid {
		return claims, nil
	}
	return nil, jwt.ErrTokenInvalidClaims
}

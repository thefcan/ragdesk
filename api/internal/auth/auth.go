// Package auth handles password hashing and JWT issuing/verification.
package auth

import (
	"errors"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
)

// ErrInvalidToken is returned when a token fails verification.
var ErrInvalidToken = errors.New("invalid token")

// HashPassword returns a bcrypt hash of the password.
func HashPassword(password string) (string, error) {
	b, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

// CheckPassword reports whether password matches the bcrypt hash.
func CheckPassword(hash, password string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)) == nil
}

// Issuer issues and verifies HS256 JWTs.
type Issuer struct {
	secret []byte
	ttl    time.Duration
}

// NewIssuer builds an Issuer with the given signing secret and token TTL.
func NewIssuer(secret string, ttl time.Duration) *Issuer {
	return &Issuer{secret: []byte(secret), ttl: ttl}
}

// Issue returns a signed token whose subject is the user id.
func (i *Issuer) Issue(userID string) (string, error) {
	now := time.Now()
	claims := jwt.RegisteredClaims{
		Subject:   userID,
		IssuedAt:  jwt.NewNumericDate(now),
		ExpiresAt: jwt.NewNumericDate(now.Add(i.ttl)),
	}
	return jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(i.secret)
}

// Verify validates the token, pinning HS256 to resist algorithm-substitution
// attacks, and returns the user id (subject).
func (i *Issuer) Verify(tokenString string) (string, error) {
	claims := &jwt.RegisteredClaims{}
	_, err := jwt.ParseWithClaims(tokenString, claims, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
		}
		return i.secret, nil
	})
	if err != nil {
		return "", ErrInvalidToken
	}
	return claims.Subject, nil
}

// dummyHash equalises login timing: comparing against it costs the same as a
// real bcrypt comparison, so a missing account does not respond faster and
// leak its (non-)existence.
var dummyHash, _ = bcrypt.GenerateFromPassword([]byte("ragdesk-timing-equaliser"), bcrypt.DefaultCost)

// DummyCompare performs a throwaway bcrypt comparison to keep login response
// time constant whether or not the account exists.
func DummyCompare(password string) {
	_ = bcrypt.CompareHashAndPassword(dummyHash, []byte(password))
}

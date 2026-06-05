// Package auth holds the cross-cutting authentication helpers used
// by both the LDAP module (issue) and the middleware (verify). The
// issue/verify pair is intentionally small and HS256-only: the spec
// mandates a shared secret read from config, and HS256 is the right
// algorithm for a single trust domain.
package auth

import (
	"errors"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"

	"github.com/devops-toolkit/backend/pkg/contracts"
)

// SigningAlgorithm is fixed at HS256 for now. The constant is
// exported so callers can introspect it in logs and tests.
const SigningAlgorithm = "HS256"

// Issuer is the value of the `iss` claim on every token we issue.
// Exposed for tests that want to assert on it.
const Issuer = "devops-toolkit"

// Signer is the minimal helper an LDAP handler needs. It hides
// the underlying jwt library so the rest of the codebase can stay
// in terms of contracts.JWTClaims.
type Signer struct {
	secret []byte
	ttl    time.Duration
	now    func() time.Time // injectable for tests
}

// NewSigner builds a Signer. The secret must be non-empty; an empty
// secret would let an attacker forge tokens, so we fail loudly.
func NewSigner(secret string, ttl time.Duration) (*Signer, error) {
	if secret == "" {
		return nil, errors.New("auth: jwt secret is required")
	}
	if ttl <= 0 {
		ttl = time.Hour
	}
	return &Signer{secret: []byte(secret), ttl: ttl, now: time.Now}, nil
}

// Issue builds a signed JWT for the supplied user. The token carries
// the iss, sub, and standard exp/iat claims plus the fields declared
// on contracts.JWTClaims. The same struct is used to verify, so the
// claim names cannot drift between the two paths.
func (s *Signer) Issue(u *contracts.User) (string, int64, error) {
	now := s.now()
	exp := now.Add(s.ttl)
	claims := contracts.JWTClaims{
		UserID:    u.ID,
		Username:  u.Username,
		Role:      u.Role,
		ExpiresAt: exp.Unix(),
	}
	tok := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"iss": Issuer,
		"sub": u.ID,
		"iat": now.Unix(),
		"exp": exp.Unix(),
		"uid": claims.UserID,
		"usr": claims.Username,
		"rol": string(claims.Role),
	})
	signed, err := tok.SignedString(s.secret)
	if err != nil {
		return "", 0, fmt.Errorf("auth: sign token: %w", err)
	}
	return signed, exp.Unix(), nil
}

// Verify parses and validates a token, returning the decoded claims
// on success. We reject anything that is not HS256 and anything
// whose signature does not match our secret.
func (s *Signer) Verify(raw string) (*contracts.JWTClaims, error) {
	parsed, err := jwt.Parse(raw, func(t *jwt.Token) (any, error) {
		if t.Method.Alg() != SigningAlgorithm {
			return nil, fmt.Errorf("auth: unexpected signing method %q", t.Method.Alg())
		}
		return s.secret, nil
	})
	if err != nil {
		return nil, fmt.Errorf("auth: parse token: %w", err)
	}
	mc, ok := parsed.Claims.(jwt.MapClaims)
	if !ok || !parsed.Valid {
		return nil, errors.New("auth: invalid token")
	}
	uid, _ := mc["uid"].(string)
	usr, _ := mc["usr"].(string)
	rol, _ := mc["rol"].(string)
	var exp int64
	switch v := mc["exp"].(type) {
	case float64:
		exp = int64(v)
	case int64:
		exp = v
	case int:
		exp = int64(v)
	}
	c := &contracts.JWTClaims{
		UserID:    uid,
		Username:  usr,
		Role:      contracts.Role(rol),
		ExpiresAt: exp,
	}
	if c.IsExpired() {
		return nil, errors.New("auth: token expired")
	}
	return c, nil
}

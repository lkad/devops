package hub

import (
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"

	"github.com/devops-toolkit/backend/pkg/contracts"
)

// buildHS256 mints a hand-crafted HS256 token. Used by the expired-
// token test where Signer.Issue would clamp the TTL.
func buildHS256(t *testing.T, secret, uid, usr string, role contracts.Role, exp int64) string {
	t.Helper()
	tok := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"iss": "devops-toolkit",
		"sub": uid,
		"iat": time.Now().Add(-time.Hour).Unix(),
		"exp": exp,
		"uid": uid,
		"usr": usr,
		"rol": string(role),
	})
	signed, err := tok.SignedString([]byte(secret))
	if err != nil {
		t.Fatalf("SignedString: %v", err)
	}
	return signed
}

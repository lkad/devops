package contracts

import "testing"

func TestJWTClaims_ExpiredInPast(t *testing.T) {
	// GIVEN a token whose exp is in the past
	// WHEN IsExpired is called
	// THEN it returns true
	c := &JWTClaims{Username: "u", Role: RoleSuperAdmin, ExpiresAt: 1}
	if !c.IsExpired() {
		t.Error("past exp should be expired")
	}
}

func TestNowFn_Override(t *testing.T) {
	// GIVEN a frozen clock
	// WHEN nowFn is replaced
	// THEN IsExpired uses the new clock
	orig := nowFn
	defer func() { nowFn = orig }()

	nowFn = func() int64 { return 1000 }

	c := &JWTClaims{ExpiresAt: 1000} // exactly now
	if !c.IsExpired() {
		t.Error("token at exact current time should be expired (exp <= now)")
	}

	c = &JWTClaims{ExpiresAt: 1001}
	if c.IsExpired() {
		t.Error("future exp should not be expired")
	}
}

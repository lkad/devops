package servicecatalog

import (
	"testing"
)

// TestService_Create_ValidatesName pins the name rules
// from the spec: non-blank, ≤ 64 chars, must trim.
func TestService_Create_ValidatesName(t *testing.T) {
	svc := NewCatalog(NewRepository(openTestDB(t)))

	for _, name := range []string{"", "   ", "\t"} {
		_, err := svc.Create(CreateInput{Name: name, Tier: TierStandard})
		if err == nil {
			t.Errorf("name=%q should be rejected", name)
		}
	}

	// 65-char name rejected.
	long := make([]byte, 65)
	for i := range long {
		long[i] = 'a'
	}
	if _, err := svc.Create(CreateInput{Name: string(long), Tier: TierStandard}); err == nil {
		t.Errorf("65-char name should be rejected")
	}

	// Valid name accepted.
	out, err := svc.Create(CreateInput{Name: "ok", Tier: TierStandard})
	if err != nil {
		t.Fatalf("valid name: %v", err)
	}
	if out.Name != "ok" {
		t.Errorf("name = %q, want %q", out.Name, "ok")
	}
}

// TestService_Create_ValidatesTier pins the tier enum
// from the spec: critical | important | standard.
func TestService_Create_ValidatesTier(t *testing.T) {
	svc := NewCatalog(NewRepository(openTestDB(t)))
	if _, err := svc.Create(CreateInput{Name: "x", Tier: "super-critical"}); err == nil {
		t.Errorf("unknown tier should be rejected")
	}
}

// TestService_Create_TrimsWhitespace pins that the input
// name is trimmed before persisting. Operators paste
// names from chat / ticket comments; surrounding
// whitespace is noise.
func TestService_Create_TrimsWhitespace(t *testing.T) {
	svc := NewCatalog(NewRepository(openTestDB(t)))
	out, err := svc.Create(CreateInput{Name: "  payments  ", Tier: TierStandard})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if out.Name != "payments" {
		t.Errorf("name = %q, want %q (trimmed)", out.Name, "payments")
	}
}

// TestService_Update_AllowsNilTier pins the partial-update
// shape: nil means "do not change". (Tier change requires
// an explicit string, not nil, otherwise we cannot
// distinguish "keep" from "set to ''".)
func TestService_Update_AllowsNilTier(t *testing.T) {
	svc := NewCatalog(NewRepository(openTestDB(t)))
	in, _ := svc.Create(CreateInput{Name: "x", Tier: TierCritical})

	// Update without touching tier. We need a variable so
	// the *string field can take its address.
	desc := "new"
	out, err := svc.Update(in.ID, UpdateInput{Description: &desc})
	if err != nil {
		t.Fatalf("update: %v", err)
	}
	if out.Tier != TierCritical {
		t.Errorf("tier changed to %q when not requested", out.Tier)
	}
	if out.Description != "new" {
		t.Errorf("description = %q, want %q", out.Description, "new")
	}
}

// TestService_DuplicateName_ReturnsConflict pins the
// 409 path on a duplicate create.
func TestService_DuplicateName_ReturnsConflict(t *testing.T) {
	svc := NewCatalog(NewRepository(openTestDB(t)))
	_, _ = svc.Create(CreateInput{Name: "dupe", Tier: TierStandard})
	_, err := svc.Create(CreateInput{Name: "dupe", Tier: TierStandard})
	if err == nil {
		t.Fatalf("expected conflict error")
	}
	if !IsConflict(err) {
		t.Errorf("err = %v, want IsConflict", err)
	}
}

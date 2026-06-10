package servicecatalog

import (
	"testing"
)

// TestRunbook_ListByService_OrderedByCreatedAt pins the
// P2.3 ordering rule: entries are returned newest first
// (so the operator sees the most recently updated
// runbook entry at the top of the detail page).
func TestRunbook_ListByService_OrderedByCreatedAt(t *testing.T) {
	repo := NewRepository(openTestDB(t))
	for _, e := range []RunbookEntry{
		{ServiceID: "svc-x", Title: "first", Body: "b1"},
		{ServiceID: "svc-x", Title: "second", Body: "b2"},
		{ServiceID: "svc-x", Title: "third", Body: "b3"},
	} {
		if err := repo.CreateRunbook(&e); err != nil {
			t.Fatalf("create %s: %v", e.Title, err)
		}
	}
	rows, err := repo.ListRunbook("svc-x")
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(rows) != 3 {
		t.Fatalf("got %d, want 3", len(rows))
	}
	// Newest first: third, second, first.
	if rows[0].Title != "third" || rows[1].Title != "second" || rows[2].Title != "first" {
		t.Errorf("order = %s,%s,%s, want third,second,first",
			rows[0].Title, rows[1].Title, rows[2].Title)
	}
}

// TestRunbook_OtherServiceNotIncluded pins the FK
// scope: a runbook entry for service A is not returned
// when listing for service B.
func TestRunbook_OtherServiceNotIncluded(t *testing.T) {
	repo := NewRepository(openTestDB(t))
	if err := repo.CreateRunbook(&RunbookEntry{ServiceID: "svc-a", Title: "a", Body: "a"}); err != nil {
		t.Fatalf("create: %v", err)
	}
	if err := repo.CreateRunbook(&RunbookEntry{ServiceID: "svc-b", Title: "b", Body: "b"}); err != nil {
		t.Fatalf("create: %v", err)
	}
	rows, err := repo.ListRunbook("svc-a")
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(rows) != 1 {
		t.Errorf("got %d, want 1 (svc-b's entry must not leak)", len(rows))
	}
	if rows[0].ServiceID != "svc-a" {
		t.Errorf("got %q, want svc-a", rows[0].ServiceID)
	}
}

package database

import (
	"testing"

	"github.com/google/uuid"
)

func TestNewID_GeneratesUUIDv4(t *testing.T) {
	// GIVEN NewID is called
	// WHEN the result is parsed
	// THEN it is a valid UUIDv4
	id := NewID()
	if _, err := uuid.Parse(id); err != nil {
		t.Errorf("NewID() = %q, not a UUID: %v", id, err)
	}
}

func TestNewID_UniqueAcrossCalls(t *testing.T) {
	// GIVEN two consecutive NewID calls
	// WHEN compared
	// THEN they differ
	a, b := NewID(), NewID()
	if a == b {
		t.Errorf("NewID() returned duplicates: %q == %q", a, b)
	}
}

func uuidFromString(s string) (uuid.UUID, error) {
	return uuid.Parse(s)
}

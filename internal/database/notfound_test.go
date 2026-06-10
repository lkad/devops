package database

import (
	"errors"
	"testing"

	"gorm.io/gorm"
)

func TestMapNotFound_TranslatesRecordNotFound(t *testing.T) {
	sentinel := errors.New("not-found")
	got := MapNotFound(gorm.ErrRecordNotFound, sentinel)
	if !errors.Is(got, sentinel) {
		t.Errorf("MapNotFound(ErrRecordNotFound, sentinel) = %v, want sentinel", got)
	}
}

func TestMapNotFound_PreservesOtherErrors(t *testing.T) {
	other := errors.New("connection refused")
	got := MapNotFound(other, errors.New("not-found"))
	if !errors.Is(got, other) {
		t.Errorf("MapNotFound(other, sentinel) = %v, want other", got)
	}
}

func TestMapNotFound_NilIsNil(t *testing.T) {
	got := MapNotFound(nil, errors.New("not-found"))
	if got != nil {
		t.Errorf("MapNotFound(nil, sentinel) = %v, want nil", got)
	}
}

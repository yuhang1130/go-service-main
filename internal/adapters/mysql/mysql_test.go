package mysql

import (
	"errors"
	"testing"

	"github.com/yuhang1130/go-service-main/internal/foundation/persistence"
	"gorm.io/gorm"
)

func TestNormalizeErrorMarksDuplicatedKeyAsConflict(t *testing.T) {
	err := NormalizeError(gorm.ErrDuplicatedKey)
	if !errors.Is(err, persistence.ErrConflict) {
		t.Fatalf("NormalizeError() = %v, want persistence conflict", err)
	}
	if !errors.Is(err, gorm.ErrDuplicatedKey) {
		t.Fatalf("NormalizeError() = %v, want original cause preserved", err)
	}
}

func TestNormalizeErrorPreservesUnrelatedError(t *testing.T) {
	want := errors.New("unavailable")
	if got := NormalizeError(want); got != want {
		t.Fatalf("NormalizeError() = %v, want %v", got, want)
	}
}

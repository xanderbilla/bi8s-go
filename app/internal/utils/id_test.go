package utils

import (
	"testing"

	"github.com/google/uuid"
)

func TestGenerateIDIsValidUUID(t *testing.T) {
	id := GenerateID()
	if _, err := uuid.Parse(id); err != nil {
		t.Fatalf("GenerateID returned invalid UUID %q: %v", id, err)
	}
}

func TestGenerateIDUnique(t *testing.T) {
	seen := make(map[string]struct{}, 256)
	for i := 0; i < 256; i++ {
		id := GenerateID()
		if _, dup := seen[id]; dup {
			t.Fatalf("GenerateID returned duplicate %q", id)
		}
		seen[id] = struct{}{}
	}
}

func TestGenerateNumericIDFormat(t *testing.T) {
	for i := 0; i < 100; i++ {
		id := GenerateNumericID()
		if len(id) != 6 {
			t.Fatalf("GenerateNumericID len = %d, want 6: %q", len(id), id)
		}
		if id[0] == '0' {
			t.Fatalf("GenerateNumericID has leading zero: %q", id)
		}
		for _, r := range id {
			if r < '0' || r > '9' {
				t.Fatalf("GenerateNumericID has non-digit: %q", id)
			}
		}
	}
}

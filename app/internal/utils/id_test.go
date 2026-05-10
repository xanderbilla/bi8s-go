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

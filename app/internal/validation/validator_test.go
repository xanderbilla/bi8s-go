package validation

import "testing"

type sample struct {
	Name string `validate:"required"`
	Age  int    `validate:"gte=18"`
}

func TestFieldErrors(t *testing.T) {
	err := ValidateStruct(sample{Name: "", Age: 10})
	if err == nil {
		t.Fatal("expected validation error")
	}
	fes := FieldErrors(err)
	if len(fes) != 2 {
		t.Fatalf("got %d field errors, want 2: %+v", len(fes), fes)
	}
	seen := map[string]string{}
	for _, fe := range fes {
		seen[fe.Field] = fe.Code
	}
	if seen["Name"] != "required" {
		t.Errorf("Name code = %q, want required", seen["Name"])
	}
	if seen["Age"] != "gte" {
		t.Errorf("Age code = %q, want gte", seen["Age"])
	}
}

func TestFieldErrors_NilAndNonValidator(t *testing.T) {
	if FieldErrors(nil) != nil {
		t.Error("nil err should yield nil slice")
	}
	if FieldErrors(errString("x")) != nil {
		t.Error("non-validator err should yield nil slice")
	}
}

type errString string

func (e errString) Error() string { return string(e) }

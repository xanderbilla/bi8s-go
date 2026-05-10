package env

import (
	"os"
	"path/filepath"
	"testing"
)

func TestGetString(t *testing.T) {
	t.Setenv("BI8S_TEST_STR", "hello")
	if got := GetString("BI8S_TEST_STR", "fallback"); got != "hello" {
		t.Fatalf("GetString = %q", got)
	}
	if got := GetString("BI8S_TEST_STR_MISSING", "fallback"); got != "fallback" {
		t.Fatalf("fallback = %q", got)
	}
}

func TestGetInt(t *testing.T) {
	t.Setenv("BI8S_TEST_INT", "42")
	if got := GetInt("BI8S_TEST_INT", 7); got != 42 {
		t.Fatalf("GetInt = %d", got)
	}
	t.Setenv("BI8S_TEST_INT_BAD", "nope")
	if got := GetInt("BI8S_TEST_INT_BAD", 7); got != 7 {
		t.Fatalf("invalid fallback = %d", got)
	}
	if got := GetInt("BI8S_TEST_INT_MISSING", 9); got != 9 {
		t.Fatalf("missing fallback = %d", got)
	}
}

func TestGetIntStrict(t *testing.T) {
	t.Setenv("BI8S_TEST_INT_STRICT", "42")
	got, err := GetIntStrict("BI8S_TEST_INT_STRICT", 7)
	if err != nil || got != 42 {
		t.Fatalf("GetIntStrict = %d, %v", got, err)
	}

	got, err = GetIntStrict("BI8S_TEST_INT_STRICT_MISSING", 9)
	if err != nil || got != 9 {
		t.Fatalf("GetIntStrict missing = %d, %v", got, err)
	}

	t.Setenv("BI8S_TEST_INT_STRICT_BAD", "x")
	if _, err := GetIntStrict("BI8S_TEST_INT_STRICT_BAD", 1); err == nil {
		t.Fatal("expected parse error for invalid int")
	}
}

func TestGetBoolStrict(t *testing.T) {
	t.Setenv("BI8S_TEST_BOOL_STRICT", "true")
	got, err := GetBoolStrict("BI8S_TEST_BOOL_STRICT", false)
	if err != nil || !got {
		t.Fatalf("GetBoolStrict true = %v, %v", got, err)
	}

	got, err = GetBoolStrict("BI8S_TEST_BOOL_STRICT_MISSING", false)
	if err != nil || got {
		t.Fatalf("GetBoolStrict missing = %v, %v", got, err)
	}

	t.Setenv("BI8S_TEST_BOOL_STRICT_BAD", "truthy")
	if _, err := GetBoolStrict("BI8S_TEST_BOOL_STRICT_BAD", false); err == nil {
		t.Fatal("expected parse error for invalid bool")
	}
}

func TestLoadDotEnv(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".env")
	content := "# comment\nA=1\nB=hello\nC=\"a b\"\nD='x y'\n"
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write dotenv: %v", err)
	}

	t.Setenv("B", "already-set")
	loaded, err := LoadDotEnv(path)
	if err != nil {
		t.Fatalf("LoadDotEnv error: %v", err)
	}
	if !loaded {
		t.Fatal("expected loaded=true")
	}

	if got := os.Getenv("A"); got != "1" {
		t.Fatalf("A = %q", got)
	}
	if got := os.Getenv("B"); got != "already-set" {
		t.Fatalf("B should not be overridden, got %q", got)
	}
	if got := os.Getenv("C"); got != "a b" {
		t.Fatalf("C = %q", got)
	}
	if got := os.Getenv("D"); got != "x y" {
		t.Fatalf("D = %q", got)
	}
}

func TestLoadDotEnv_InvalidLine(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".env")
	if err := os.WriteFile(path, []byte("NOT_VALID"), 0o600); err != nil {
		t.Fatalf("write dotenv: %v", err)
	}
	if _, err := LoadDotEnv(path); err == nil {
		t.Fatal("expected parse error")
	}
}

func TestGetSecret(t *testing.T) {
	t.Setenv("BI8S_TEST_SECRET", "shh")
	if got := GetSecret("BI8S_TEST_SECRET"); got != "shh" {
		t.Fatalf("GetSecret = %q", got)
	}
}

func TestGetBool(t *testing.T) {
	if got := GetBool("BI8S_TEST_BOOL_UNSET", true); got != true {
		t.Fatalf("unset fallback = %v", got)
	}
	for _, v := range []string{"1", "t", "TRUE", "true"} {
		t.Setenv("BI8S_TEST_BOOL", v)
		if !GetBool("BI8S_TEST_BOOL", false) {
			t.Fatalf("value %q expected true", v)
		}
	}
	for _, v := range []string{"0", "f", "FALSE", "false"} {
		t.Setenv("BI8S_TEST_BOOL", v)
		if GetBool("BI8S_TEST_BOOL", true) {
			t.Fatalf("value %q expected false", v)
		}
	}
	t.Setenv("BI8S_TEST_BOOL", "notabool")
	if got := GetBool("BI8S_TEST_BOOL", true); got != true {
		t.Fatalf("invalid value should fall back, got %v", got)
	}
}

func TestMustString(t *testing.T) {
	t.Setenv("BI8S_TEST_REQ", "ok")
	v, err := MustString("BI8S_TEST_REQ")
	if err != nil || v != "ok" {
		t.Fatalf("MustString set: %q, %v", v, err)
	}
	if _, err := MustString("BI8S_TEST_REQ_MISSING"); err == nil {
		t.Fatal("MustString missing: expected error")
	}
}

func TestIntInRange(t *testing.T) {
	t.Setenv("BI8S_TEST_RANGE", "5")
	if got := IntInRange("BI8S_TEST_RANGE", 1, 1, 10); got != 5 {
		t.Fatalf("in range = %d", got)
	}
	t.Setenv("BI8S_TEST_RANGE_LOW", "-5")
	if got := IntInRange("BI8S_TEST_RANGE_LOW", 1, 0, 10); got != 0 {
		t.Fatalf("clamp low = %d", got)
	}
	t.Setenv("BI8S_TEST_RANGE_HIGH", "100")
	if got := IntInRange("BI8S_TEST_RANGE_HIGH", 1, 0, 10); got != 10 {
		t.Fatalf("clamp high = %d", got)
	}
}

func TestParseLogLevel(t *testing.T) {
	cases := map[string]string{
		"debug":   "DEBUG",
		"info":    "INFO",
		"warn":    "WARN",
		"warning": "WARN",
		"error":   "ERROR",
		"":        "INFO",
		"BOGUS":   "INFO",
	}
	for in, want := range cases {
		if got := ParseLogLevel(in).String(); got != want {
			t.Fatalf("ParseLogLevel(%q) = %s, want %s", in, got, want)
		}
	}
}

func TestParseCommaSeparated(t *testing.T) {
	got := ParseCommaSeparated(" a , b ,, c,")
	want := []string{"a", "b", "c"}
	if len(got) != len(want) {
		t.Fatalf("len = %d, want %d (%v)", len(got), len(want), got)
	}
	for i, v := range want {
		if got[i] != v {
			t.Fatalf("got[%d] = %q, want %q", i, got[i], v)
		}
	}
	if out := ParseCommaSeparated(""); len(out) != 0 {
		t.Fatalf("empty input => %v", out)
	}
}

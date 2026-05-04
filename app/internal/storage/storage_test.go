package storage

import "testing"

func TestSanitizeSegment(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"abc-123", "abc-123"},
		{"  spaced  ", "spaced"},
		{"", ""},
		{".", ""},
		{"..", ""},
		{"a/b", ""},
		{"a\\b", ""},
		{"a\x00b", ""},
		{"a\nb", ""},
	}
	for _, tc := range cases {
		if got := sanitizeSegment(tc.in); got != tc.want {
			t.Errorf("sanitizeSegment(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestIsSafeKey(t *testing.T) {
	cases := []struct {
		in   string
		want bool
	}{
		{"movies/abc/index.m3u8", true},
		{"tv/show/s1/e1.ts", true},
		{"", false},
		{"/leading", false},
		{"a//b", false},
		{"a/../b", false},
		{"a/./b", false},
		{"a\\b", false},
		{"a\x00b", false},
	}
	for _, tc := range cases {
		if got := isSafeKey(tc.in); got != tc.want {
			t.Errorf("isSafeKey(%q) = %v, want %v", tc.in, got, tc.want)
		}
	}
}

func TestVerifyMagicBytes(t *testing.T) {
	png := []byte{0x89, 'P', 'N', 'G', 0x0D, 0x0A, 0x1A, 0x0A, 0, 0, 0, 0}
	html := []byte("<!doctype html><html></html>")

	if err := VerifyMagicBytes("image/png", png); err != nil {
		t.Errorf("png/image: %v", err)
	}
	if err := VerifyMagicBytes("image/png", html); err == nil {
		t.Error("html declared as image/png must be rejected")
	}

	if err := VerifyMagicBytes("video/mp4", html); err == nil {
		t.Error("html declared as video/mp4 must be rejected")
	}

	if err := VerifyMagicBytes("video/x-matroska", []byte{0x1A, 0x45, 0xDF, 0xA3}); err != nil {
		t.Errorf("mkv-ish: %v", err)
	}
}

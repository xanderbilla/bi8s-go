package http

import (
	"net/http/httptest"
	"testing"
)

func TestNormalizeOrigin(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		origin string
		want   string
		ok     bool
	}{
		{name: "https default port", origin: "https://dev.d3uphs6spds7z1.amplifyapp.com:443", want: "https://dev.d3uphs6spds7z1.amplifyapp.com", ok: true},
		{name: "http default port", origin: "http://localhost:80", want: "http://localhost", ok: true},
		{name: "host casing", origin: "https://DEV.D3UPHS6SPDS7Z1.AMPLIFYAPP.COM", want: "https://dev.d3uphs6spds7z1.amplifyapp.com", ok: true},
		{name: "trailing slash", origin: "https://dev.d3uphs6spds7z1.amplifyapp.com/", want: "https://dev.d3uphs6spds7z1.amplifyapp.com", ok: true},
		{name: "reject query", origin: "https://dev.d3uphs6spds7z1.amplifyapp.com?x=1", ok: false},
		{name: "reject invalid scheme", origin: "ftp://example.com", ok: false},
	}

	for _, tt := range tests {
		tc := tt
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got, ok := normalizeOrigin(tc.origin)
			if ok != tc.ok {
				t.Fatalf("ok = %v, want %v", ok, tc.ok)
			}
			if got != tc.want {
				t.Fatalf("normalizeOrigin(%q) = %q, want %q", tc.origin, got, tc.want)
			}
		})
	}
}

func TestCORSOriginMatcher(t *testing.T) {
	t.Parallel()

	match := newCORSOriginMatcher([]string{
		"https://dev.d3uphs6spds7z1.amplifyapp.com",
		"https://localhost",
	})
	req := httptest.NewRequest("GET", "/v1/c/banner", nil)

	tests := []struct {
		name   string
		origin string
		want   bool
	}{
		{name: "exact match", origin: "https://dev.d3uphs6spds7z1.amplifyapp.com", want: true},
		{name: "default https port", origin: "https://dev.d3uphs6spds7z1.amplifyapp.com:443", want: true},
		{name: "different origin", origin: "https://example.com", want: false},
		{name: "trailing slash accepted", origin: "https://dev.d3uphs6spds7z1.amplifyapp.com/", want: true},
	}

	for _, tt := range tests {
		tc := tt
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			if got := match(req, tc.origin); got != tc.want {
				t.Fatalf("match(%q) = %v, want %v", tc.origin, got, tc.want)
			}
		})
	}
}

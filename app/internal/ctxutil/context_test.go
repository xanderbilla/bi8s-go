package ctxutil

import (
	"context"
	"testing"
	"time"
)

func TestRequestIDRoundTrip(t *testing.T) {
	ctx := WithRequestID(context.Background(), "req-1")
	if got := GetRequestID(ctx); got != "req-1" {
		t.Fatalf("GetRequestID = %q, want %q", got, "req-1")
	}
	if got := GetRequestID(context.Background()); got != "" {
		t.Fatalf("missing key: GetRequestID = %q, want \"\"", got)
	}
}

func TestUserIDRoundTrip(t *testing.T) {
	ctx := WithUserID(context.Background(), "user-42")
	if got := GetUserID(ctx); got != "user-42" {
		t.Fatalf("GetUserID = %q, want %q", got, "user-42")
	}
	if got := GetUserID(context.Background()); got != "" {
		t.Fatalf("missing key: GetUserID = %q, want \"\"", got)
	}
}

func TestRolesRoundTrip(t *testing.T) {
	roles := []string{"admin", "editor"}
	ctx := WithRoles(context.Background(), roles)
	got := GetRoles(ctx)
	if len(got) != 2 || got[0] != "admin" || got[1] != "editor" {
		t.Fatalf("GetRoles = %v, want %v", got, roles)
	}
	if GetRoles(context.Background()) != nil {
		t.Fatalf("missing key: GetRoles should return nil")
	}
}

func TestTimeoutHelpers(t *testing.T) {
	tests := []struct {
		name string
		fn   func(context.Context) (context.Context, context.CancelFunc)
		want time.Duration
	}{
		{"db", WithDBTimeout, DefaultDBTimeout},
		{"s3", WithS3Timeout, DefaultS3Timeout},
		{"api", WithAPITimeout, DefaultAPITimeout},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctx, cancel := tc.fn(context.Background())
			defer cancel()
			deadline, ok := ctx.Deadline()
			if !ok {
				t.Fatalf("expected deadline")
			}
			remaining := time.Until(deadline)
			if remaining <= 0 || remaining > tc.want {
				t.Fatalf("remaining = %v, want in (0, %v]", remaining, tc.want)
			}
		})
	}
}

func TestWithCustomTimeout(t *testing.T) {
	ctx, cancel := WithCustomTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	if _, ok := ctx.Deadline(); !ok {
		t.Fatal("expected deadline")
	}
}

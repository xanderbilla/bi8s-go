package redis

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
)

func TestNew_PingsAndReturnsClient(t *testing.T) {
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("miniredis: %v", err)
	}
	t.Cleanup(mr.Close)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	c, err := New(ctx, Options{URL: "redis://" + mr.Addr() + "/0"})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	t.Cleanup(func() { _ = c.Close() })

	if err := c.Set(ctx, "k", "v", 0).Err(); err != nil {
		t.Fatalf("set: %v", err)
	}
	got, err := c.Get(ctx, "k").Result()
	if err != nil || got != "v" {
		t.Fatalf("get: %q err=%v", got, err)
	}
}

func TestNew_RejectsEmptyURL(t *testing.T) {
	if _, err := New(context.Background(), Options{}); err == nil {
		t.Fatalf("expected error on empty URL")
	}
}

func TestNew_BadURL(t *testing.T) {
	if _, err := New(context.Background(), Options{URL: "not-a-url"}); err == nil {
		t.Fatalf("expected parse error")
	}
}

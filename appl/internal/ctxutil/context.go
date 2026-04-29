package ctxutil

import (
	"context"
	"time"
)

type contextKey struct{ name string }

var (
	requestIDKey = contextKey{"request_id"}
	userIDKey    = contextKey{"user_id"}
	rolesKey     = contextKey{"roles"}
)

var (
	DefaultDBTimeout     = 5 * time.Second
	DefaultS3Timeout     = 30 * time.Second
	DefaultAPITimeout    = 10 * time.Second
	DefaultLongOpTimeout = 60 * time.Second
)

type Timeouts struct {
	DB     time.Duration
	S3     time.Duration
	API    time.Duration
	LongOp time.Duration
}

func ConfigureTimeouts(t Timeouts) {
	if t.DB > 0 {
		DefaultDBTimeout = t.DB
	}
	if t.S3 > 0 {
		DefaultS3Timeout = t.S3
	}
	if t.API > 0 {
		DefaultAPITimeout = t.API
	}
	if t.LongOp > 0 {
		DefaultLongOpTimeout = t.LongOp
	}
}

func WithRequestID(ctx context.Context, requestID string) context.Context {
	return context.WithValue(ctx, requestIDKey, requestID)
}

func GetRequestID(ctx context.Context) string {
	if reqID, ok := ctx.Value(requestIDKey).(string); ok {
		return reqID
	}
	return ""
}

func WithUserID(ctx context.Context, userID string) context.Context {
	return context.WithValue(ctx, userIDKey, userID)
}

func GetUserID(ctx context.Context) string {
	if userID, ok := ctx.Value(userIDKey).(string); ok {
		return userID
	}
	return ""
}

func WithRoles(ctx context.Context, roles []string) context.Context {
	return context.WithValue(ctx, rolesKey, roles)
}

func GetRoles(ctx context.Context) []string {
	if roles, ok := ctx.Value(rolesKey).([]string); ok {
		return roles
	}
	return nil
}

func WithDBTimeout(ctx context.Context) (context.Context, context.CancelFunc) {
	return context.WithTimeout(ctx, DefaultDBTimeout)
}

func WithS3Timeout(ctx context.Context) (context.Context, context.CancelFunc) {
	return context.WithTimeout(ctx, DefaultS3Timeout)
}

func WithAPITimeout(ctx context.Context) (context.Context, context.CancelFunc) {
	return context.WithTimeout(ctx, DefaultAPITimeout)
}

func WithCustomTimeout(ctx context.Context, timeout time.Duration) (context.Context, context.CancelFunc) {
	return context.WithTimeout(ctx, timeout)
}

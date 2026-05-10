package service

import (
	"context"
	"encoding/json"
	"time"

	goredis "github.com/redis/go-redis/v9"

	"github.com/xanderbilla/bi8s-go/internal/logger"
)

func cacheGetJSON[T any](ctx context.Context, rc *goredis.Client, key string) (*T, bool) {
	if rc == nil {
		return nil, false
	}
	raw, err := rc.Get(ctx, key).Bytes()
	if err != nil {
		return nil, false
	}
	var v T
	if err := json.Unmarshal(raw, &v); err != nil {
		return nil, false
	}
	return &v, true
}

func cacheSetJSON(ctx context.Context, rc *goredis.Client, key string, v any, ttl time.Duration, label, idKey, idValue string) {
	if rc == nil || v == nil {
		return
	}
	data, err := json.Marshal(v)
	if err != nil {
		return
	}
	if err := rc.Set(ctx, key, data, ttl).Err(); err != nil {
		logger.WarnContext(ctx, label+" Redis SET failed", idKey, idValue, "error", err)
	}
}

func cacheDel(ctx context.Context, rc *goredis.Client, key, label, idKey, idValue string) {
	if rc == nil {
		return
	}
	if err := rc.Del(ctx, key).Err(); err != nil {
		logger.WarnContext(ctx, label+" Redis DEL failed", idKey, idValue, "error", err)
	}
}

package app

import (
	"context"

	goredis "github.com/redis/go-redis/v9"

	awsinfra "github.com/xanderbilla/bi8s-go/internal/aws"
	"github.com/xanderbilla/bi8s-go/internal/http/middleware/ratelimit"
	"github.com/xanderbilla/bi8s-go/internal/observability"
	"github.com/xanderbilla/bi8s-go/internal/service"
)

type HealthCheck func(ctx context.Context) error

type Application struct {
	Config           Config
	Clients          *awsinfra.Clients
	MovieService     *service.MovieService
	PersonService    *service.PersonService
	AttributeService *service.AttributeService
	EncoderService   *service.EncoderService
	HealthChecks     map[string]HealthCheck
	HTTPMetrics      *observability.HTTPMetrics

	// RateLimitFactory builds per-route rate-limit Backends. Always non-nil.
	RateLimitFactory ratelimit.Factory
	// RedisClient is set only when RateLimitBackend == "redis"; nil otherwise.
	// Closed during graceful shutdown.
	RedisClient *goredis.Client
}

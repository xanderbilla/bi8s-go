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
	ContentService   *service.ContentService
	PersonService    *service.PersonService
	SearchService    *service.SearchService
	AttributeService *service.AttributeService
	EncoderService   *service.EncoderService
	HealthChecks     map[string]HealthCheck
	HTTPMetrics      *observability.HTTPMetrics

	RateLimitFactory ratelimit.Factory

	RedisClient *goredis.Client
}

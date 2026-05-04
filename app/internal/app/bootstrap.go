package app

import (
	"context"
	"errors"
	"log/slog"
	"os/exec"
	"sort"
	"strings"
	"time"

	awsSDK "github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	goredis "github.com/redis/go-redis/v9"

	"github.com/xanderbilla/bi8s-go/internal/aws"
	"github.com/xanderbilla/bi8s-go/internal/ctxutil"
	"github.com/xanderbilla/bi8s-go/internal/env"
	"github.com/xanderbilla/bi8s-go/internal/http/middleware/ratelimit"
	redispkg "github.com/xanderbilla/bi8s-go/internal/redis"
	"github.com/xanderbilla/bi8s-go/internal/repository"
	"github.com/xanderbilla/bi8s-go/internal/service"
	"github.com/xanderbilla/bi8s-go/internal/storage"
)

func ConfigureRuntime() {
	ctxutil.ConfigureTimeouts(ctxutil.Timeouts{
		DB:     time.Duration(env.GetInt("CTX_DB_TIMEOUT_MS", 5000)) * time.Millisecond,
		S3:     time.Duration(env.GetInt("CTX_S3_TIMEOUT_MS", 30000)) * time.Millisecond,
		API:    time.Duration(env.GetInt("CTX_API_TIMEOUT_MS", 10000)) * time.Millisecond,
		LongOp: time.Duration(env.GetInt("CTX_LONG_OP_TIMEOUT_MS", 60000)) * time.Millisecond,
	})
	repository.ConfigureMaxScanPages(env.GetInt("DYNAMODB_MAX_SCAN_PAGES", 10))
}

func LoadConfigFromEnv() Config {
	defaultCORSOrigins := env.GetString("DEFAULT_CORS_ORIGINS", DefaultCORSOrigins)

	return Config{
		Addr:                    env.GetString("PORT", ":8080"),
		Env:                     env.GetString("APP_ENV", "prod"),
		TableName:               env.GetString("DYNAMODB_MOVIE_TABLE", "bi8s-dev"),
		PersonTableName:         env.GetString("DYNAMODB_PERSON_TABLE", "bi8s-person-dev"),
		AttributeTableName:      env.GetString("DYNAMODB_ATTRIBUTE_TABLE", "bi8s-attribute-dev"),
		EncoderTableName:        env.GetString("DYNAMODB_ENCODER_TABLE", "bi8s-video-dev"),
		EncoderContentIDIndex:   env.GetString("DYNAMODB_ENCODER_CONTENT_ID_INDEX", "contentId-index"),
		AttributeNameIndex:      env.GetString("DYNAMODB_ATTRIBUTE_NAME_INDEX", "name-index"),
		S3Bucket:                env.GetSecret("S3_BUCKET"),
		CORSAllowedOrigins:      env.ParseCommaSeparated(env.GetString("CORS_ALLOWED_ORIGINS", defaultCORSOrigins)),
		CORSAllowPrivateNetwork: env.GetBool("CORS_ALLOW_PRIVATE_NETWORK", true),
		RateLimitBackend:        env.GetString("RATE_LIMIT_BACKEND", "memory"),
		RateLimitRedisFailMode:  env.GetString("RATE_LIMIT_REDIS_FAIL_MODE", "fail-open"),
		RateLimitRedisTimeoutMS: env.GetInt("RATE_LIMIT_REDIS_TIMEOUT_MS", 50),
		RedisURL:                env.GetString("REDIS_URL", ""),
		AWS: AWSCredentials{
			AccessKey:       env.GetSecret("AWS_ACCESS_KEY_ID"),
			SecretAccessKey: env.GetSecret("AWS_SECRET_ACCESS_KEY"),
			Region:          env.GetString("AWS_REGION", "us-east-1"),
		},
	}
}

func ConfigureTrustedProxies(cfg Config) error {
	trusted := strings.TrimSpace(env.GetString("TRUSTED_PROXIES", ""))
	if trusted != "" {
		if err := ratelimit.SetTrustedProxies(env.ParseCommaSeparated(trusted)); err != nil {
			return err
		}
		slog.Info("trusted proxies configured", "cidrs", trusted)
		return nil
	}
	if cfg.IsProd() {
		return errors.New("TRUSTED_PROXIES is required in production (set to your load-balancer/CDN CIDRs)")
	}
	slog.Warn("TRUSTED_PROXIES not set; X-Forwarded-For headers will be ignored and rate limiting will use peer IP only")
	return nil
}

func Build(ctx context.Context, cfg Config) (*Application, error) {
	awsCfg, err := aws.LoadConfig(ctx, cfg.AWS.Region, cfg.AWS.AccessKey, cfg.AWS.SecretAccessKey)
	if err != nil {
		return nil, err
	}

	clients := aws.NewClients(awsCfg)
	uploader := storage.NewS3FileUploader(clients.S3, cfg.S3Bucket)

	rlFactory, redisClient, err := buildRateLimitFactory(ctx, cfg)
	if err != nil {
		return nil, err
	}

	attributeRepo := repository.NewAttributeDynamoRepository(clients.Dynamo, cfg.AttributeTableName, cfg.AttributeNameIndex)
	personRepo := repository.NewPersonDynamoRepository(clients.Dynamo, cfg.PersonTableName)
	movieRepo := repository.NewMovieRepository(clients.Dynamo, cfg.TableName)
	encoderRepo := repository.NewEncoderRepository(clients.Dynamo, cfg.EncoderTableName, cfg.EncoderContentIDIndex)

	attributeService := service.NewAttributeService(attributeRepo)
	personService := service.NewPersonService(personRepo, attributeRepo, uploader)
	movieService := service.NewMovieService(movieRepo, personRepo, attributeRepo, encoderRepo, uploader)
	encoderService := service.NewEncoderService(encoderRepo, uploader)

	return &Application{
		Config:           cfg,
		RateLimitFactory: rlFactory,
		RedisClient:      redisClient,
		Clients:          clients,
		MovieService:     movieService,
		PersonService:    personService,
		AttributeService: attributeService,
		EncoderService:   encoderService,
		HealthChecks: map[string]HealthCheck{
			"dynamodb": func(ctx context.Context) error {
				_, err := clients.Dynamo.DescribeTable(ctx, &dynamodb.DescribeTableInput{TableName: awsSDK.String(cfg.TableName)})
				return err
			},
			"s3": func(ctx context.Context) error {
				_, err := clients.S3.HeadBucket(ctx, &s3.HeadBucketInput{Bucket: awsSDK.String(cfg.S3Bucket)})
				return err
			},

			"ffmpeg": func(ctx context.Context) error {
				if _, err := exec.LookPath("ffmpeg"); err != nil {
					return err
				}
				return exec.CommandContext(ctx, "ffmpeg", "-version").Run()
			},
			"ffprobe": func(ctx context.Context) error {
				if _, err := exec.LookPath("ffprobe"); err != nil {
					return err
				}
				return exec.CommandContext(ctx, "ffprobe", "-version").Run()
			},
		},
	}, nil
}

func RunStartupHealthChecks(ctx context.Context, app *Application) error {
	if !env.GetBool("STARTUP_HEALTH_CHECK", true) {
		return nil
	}
	names := make([]string, 0, len(app.HealthChecks))
	for name := range app.HealthChecks {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		check := app.HealthChecks[name]
		if err := check(ctx); err != nil {
			return errors.New("startup health check failed: " + name + ": " + err.Error())
		}
	}
	slog.Info("startup health checks passed", "checks", len(app.HealthChecks))
	return nil
}

// buildRateLimitFactory wires the configured rate-limit backend. For
// "memory" (default) we return a process-local factory; for "redis" we
// dial Redis up-front (failure is fatal so misconfiguration is caught at
// boot rather than per-request).
func buildRateLimitFactory(ctx context.Context, cfg Config) (ratelimit.Factory, *goredis.Client, error) {
	switch strings.ToLower(strings.TrimSpace(cfg.RateLimitBackend)) {
	case "", "memory":
		return ratelimit.MemoryFactory{}, nil, nil
	case "redis":
		dialCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
		defer cancel()
		client, err := redispkg.New(dialCtx, redispkg.Options{URL: cfg.RedisURL})
		if err != nil {
			return nil, nil, errors.New("rate-limit redis backend: " + err.Error())
		}
		factory := ratelimit.RedisFactory{
			Client:   client,
			Timeout:  time.Duration(cfg.RateLimitRedisTimeoutMS) * time.Millisecond,
			FailMode: ratelimit.ParseFailMode(cfg.RateLimitRedisFailMode),
		}
		slog.Info("rate-limit redis backend ready",
			"timeout_ms", cfg.RateLimitRedisTimeoutMS,
			"fail_mode", cfg.RateLimitRedisFailMode)
		return factory, client, nil
	default:
		return nil, nil, errors.New("unknown RATE_LIMIT_BACKEND: " + cfg.RateLimitBackend)
	}
}

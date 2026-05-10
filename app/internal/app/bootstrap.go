package app

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os/exec"
	"sort"
	"strings"
	"time"

	awsSDK "github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	goredis "github.com/redis/go-redis/v9"

	"github.com/xanderbilla/bi8s-go/internal/aws"
	"github.com/xanderbilla/bi8s-go/internal/ctxutil"
	"github.com/xanderbilla/bi8s-go/internal/env"
	"github.com/xanderbilla/bi8s-go/internal/http/middleware/ratelimit"
	redispkg "github.com/xanderbilla/bi8s-go/internal/redis"
	"github.com/xanderbilla/bi8s-go/internal/repository"
	"github.com/xanderbilla/bi8s-go/internal/search"
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

func LoadConfigFromEnv() (Config, error) {
	var firstErr error
	getInt := func(key string, fallback int) int {
		if firstErr != nil {
			return 0
		}
		v, err := env.GetIntStrict(key, fallback)
		if err != nil {
			firstErr = err
			return 0
		}
		return v
	}
	getBool := func(key string, fallback bool) bool {
		if firstErr != nil {
			return false
		}
		v, err := env.GetBoolStrict(key, fallback)
		if err != nil {
			firstErr = err
			return false
		}
		return v
	}

	httpMaxJSONBytes := getInt("HTTP_MAX_JSON_BYTES", 1<<20)
	httpMaxMultipartBytes := getInt("HTTP_MAX_MULTIPART_BYTES", 1<<30)
	routerTimeoutSeconds := getInt("ROUTER_TIMEOUT_SECONDS", 60)
	rateLimitRedisTimeoutMS := getInt("RATE_LIMIT_REDIS_TIMEOUT_MS", 50)
	rateLimitGlobalBurst := getInt("RATELIMIT_GLOBAL_BURST", 100)
	rateLimitGlobalPerMin := getInt("RATELIMIT_GLOBAL_PER_MIN", 100)
	rateLimitEncoderBurst := getInt("RATELIMIT_ENCODER_WRITE_BURST", 5)
	rateLimitEncoderPerMin := getInt("RATELIMIT_ENCODER_WRITE_PER_MIN", 5)
	rateLimitMovieBurst := getInt("RATELIMIT_MOVIE_WRITE_BURST", 20)
	rateLimitMoviePerMin := getInt("RATELIMIT_MOVIE_WRITE_PER_MIN", 20)
	rateLimitPersonBurst := getInt("RATELIMIT_PERSON_WRITE_BURST", 20)
	rateLimitPersonPerMin := getInt("RATELIMIT_PERSON_WRITE_PER_MIN", 20)
	searchRequestTimeoutMS := getInt("SEARCH_REQUEST_TIMEOUT_MS", 3000)
	corsAllowPrivateNetwork := getBool("CORS_ALLOW_PRIVATE_NETWORK", false)
	if firstErr != nil {
		return Config{}, firstErr
	}

	defaultCORSOrigins := env.GetString("DEFAULT_CORS_ORIGINS", DefaultCORSOrigins)

	return Config{
		Addr:                              env.GetString("PORT", ""),
		Env:                               env.GetString("APP_ENV", ""),
		HTTPMaxJSONBytes:                  httpMaxJSONBytes,
		HTTPMaxMultipartBytes:             int64(httpMaxMultipartBytes),
		RouterTimeoutSecond:               routerTimeoutSeconds,
		TableName:                         env.GetString("DYNAMODB_CONTENT_TABLE", ""),
		PersonTableName:                   env.GetString("DYNAMODB_PERSON_TABLE", ""),
		AttributeTableName:                env.GetString("DYNAMODB_ATTRIBUTE_TABLE", ""),
		EncoderTableName:                  env.GetString("DYNAMODB_ENCODER_TABLE", ""),
		EncoderContentIDIndex:             env.GetString("DYNAMODB_ENCODER_CONTENT_ID_INDEX", ""),
		AttributeNameIndex:                env.GetString("DYNAMODB_ATTRIBUTE_NAME_INDEX", "name-index"),
		ContentCastTableName:              env.GetString("DYNAMODB_CONTENT_CAST_TABLE", ""),
		ContentAttributeTableName:         env.GetString("DYNAMODB_CONTENT_ATTRIBUTE_TABLE", ""),
		ContentVisibilityCreatedAtIndex:   env.GetString("DYNAMODB_CONTENT_VISIBILITY_CREATED_AT_INDEX", ""),
		ContentVisibilityContentTypeIndex: env.GetString("DYNAMODB_CONTENT_VISIBILITY_CONTENT_TYPE_INDEX", "visibility-contentType-index"),
		ContentVisibilityReleaseDateIndex: env.GetString("DYNAMODB_CONTENT_VISIBILITY_RELEASE_DATE_INDEX", ""),
		S3Bucket:                          env.GetSecret("S3_BUCKET"),
		CORSAllowedOrigins:                env.ParseCommaSeparated(env.GetString("CORS_ALLOWED_ORIGINS", defaultCORSOrigins)),
		CORSAllowPrivateNetwork:           corsAllowPrivateNetwork,
		RateLimitBackend:                  env.GetString("RATE_LIMIT_BACKEND", "memory"),
		RateLimitRedisFailMode:            env.GetString("RATE_LIMIT_REDIS_FAIL_MODE", "fail-open"),
		RateLimitRedisTimeoutMS:           rateLimitRedisTimeoutMS,
		RateLimitGlobalBurst:              rateLimitGlobalBurst,
		RateLimitGlobalPerMin:             rateLimitGlobalPerMin,
		RateLimitEncoderBurst:             rateLimitEncoderBurst,
		RateLimitEncoderPerMin:            rateLimitEncoderPerMin,
		RateLimitMovieBurst:               rateLimitMovieBurst,
		RateLimitMoviePerMin:              rateLimitMoviePerMin,
		RateLimitPersonBurst:              rateLimitPersonBurst,
		RateLimitPersonPerMin:             rateLimitPersonPerMin,
		RedisURL:                          env.GetString("REDIS_URL", ""),
		RedisDialTimeoutMS:                getInt("REDIS_DIAL_TIMEOUT_MS", 0),
		RedisReadTimeoutMS:                getInt("REDIS_READ_TIMEOUT_MS", 0),
		RedisWriteTimeoutMS:               getInt("REDIS_WRITE_TIMEOUT_MS", 0),
		RedisPoolSize:                     getInt("REDIS_POOL_SIZE", 0),
		SearchEnabled:                     env.GetBool("SEARCH_ENABLED", false),
		SearchProvider:                    env.GetString("SEARCH_PROVIDER", "none"),
		SearchEndpoint:                    env.GetString("SEARCH_ENDPOINT", ""),
		SearchUsername:                    env.GetString("SEARCH_USERNAME", ""),
		SearchPassword:                    env.GetString("SEARCH_PASSWORD", ""),
		SearchContentIndexName:            env.GetString("SEARCH_CONTENT_INDEX", "bi8s-content-search"),
		SearchPeopleIndexName:             env.GetString("SEARCH_PEOPLE_INDEX", "bi8s-people-search"),
		SearchRequestTimeoutMS:            searchRequestTimeoutMS,
		AWS: AWSCredentials{
			AccessKey:       env.GetSecret("AWS_ACCESS_KEY_ID"),
			SecretAccessKey: env.GetSecret("AWS_SECRET_ACCESS_KEY"),
			Region:          env.GetString("AWS_REGION", ""),
		},
	}, nil
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
		return fmt.Errorf("TRUSTED_PROXIES is required in production (set to your load-balancer/CDN CIDRs)")
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
	contentCastRepo := repository.NewContentCastRepository(clients.Dynamo, cfg.ContentCastTableName)
	contentAttributeRepo := repository.NewContentAttributeRepository(clients.Dynamo, cfg.ContentAttributeTableName)
	contentRepo := repository.NewContentRepository(
		clients.Dynamo,
		cfg.TableName,
		cfg.ContentVisibilityCreatedAtIndex,
		cfg.ContentVisibilityContentTypeIndex,
		cfg.ContentVisibilityReleaseDateIndex,
		contentCastRepo,
		contentAttributeRepo,
	)
	encoderRepo := repository.NewEncoderRepository(clients.Dynamo, cfg.EncoderTableName, cfg.EncoderContentIDIndex)

	attributeService := service.NewAttributeService(attributeRepo)
	personService := service.NewPersonService(personRepo, attributeRepo, uploader)
	contentService := service.NewContentService(contentRepo, personRepo, attributeRepo, encoderRepo, uploader)
	encoderService := service.NewEncoderService(encoderRepo, uploader)
	contentService.SetRedisClient(redisClient)
	personService.SetRedisClient(redisClient)
	contentService.SetPlaybackURLTTL(time.Duration(env.GetInt("PLAYBACK_URL_TTL_MINUTES", 20)) * time.Minute)

	searchProvider, err := buildSearchProvider(ctx, cfg)
	if err != nil {
		return nil, err
	}
	searchService := service.NewSearchService(searchProvider, cfg.SearchEnabled)
	contentService.SetSearchService(searchService)
	personService.SetSearchService(searchService)

	if cfg.SearchEnabled {
		autoReindexOnEmpty(ctx, searchProvider, contentService, personService, searchService)
	}

	return &Application{
		Config:           cfg,
		RateLimitFactory: rlFactory,
		RedisClient:      redisClient,
		Clients:          clients,
		ContentService:   contentService,
		PersonService:    personService,
		SearchService:    searchService,
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

func buildSearchProvider(ctx context.Context, cfg Config) (search.Provider, error) {
	if !cfg.SearchEnabled {
		return search.NoopProvider{}, nil
	}
	switch strings.ToLower(strings.TrimSpace(cfg.SearchProvider)) {
	case "opensearch":
		provider, err := search.NewOpenSearchProvider(search.OpenSearchConfig{
			Endpoint:       cfg.SearchEndpoint,
			Username:       cfg.SearchUsername,
			Password:       cfg.SearchPassword,
			ContentIndex:   cfg.SearchContentIndexName,
			PeopleIndex:    cfg.SearchPeopleIndexName,
			RequestTimeout: time.Duration(cfg.SearchRequestTimeoutMS) * time.Millisecond,
		})
		if err != nil {
			return nil, err
		}
		if err := provider.EnsureIndexes(ctx); err != nil {
			return nil, err
		}
		return provider, nil
	default:
		return nil, errors.New("unsupported SEARCH_PROVIDER: " + cfg.SearchProvider)
	}
}

func autoReindexOnEmpty(
	ctx context.Context,
	provider search.Provider,
	contentService *service.ContentService,
	personService *service.PersonService,
	searchService *service.SearchService,
) {
	count, err := provider.DocCount(ctx)
	if err != nil {
		slog.Warn("search startup: unable to check doc count", "error", err)
		return
	}
	if count > 0 {
		slog.Info("search startup: indexes populated", "totalDocs", count)
		return
	}

	slog.Warn("search indexes are empty — auto-reindexing from DynamoDB")
	reindexCtx, cancel := context.WithTimeout(ctx, time.Duration(env.GetInt("SEARCH_AUTO_REINDEX_TIMEOUT_SECONDS", 300))*time.Second)
	defer cancel()

	if err := reindexSearchFromDynamo(reindexCtx, contentService, personService, searchService); err != nil {
		slog.Error("search auto-reindex failed", "error", err)
		return
	}
	slog.Info("search auto-reindex completed")
}

func reindexSearchFromDynamo(
	ctx context.Context,
	contentService *service.ContentService,
	personService *service.PersonService,
	searchService *service.SearchService,
) error {
	people, err := personService.GetAll(ctx)
	if err != nil {
		return err
	}
	for _, person := range people {
		if err := searchService.IndexPerson(ctx, person); err != nil {
			return err
		}
	}

	var startKey map[string]types.AttributeValue
	for {
		movies, nextKey, err := contentService.GetAllAdmin(ctx, 100, startKey)
		if err != nil {
			return err
		}
		for _, movie := range movies {
			if err := searchService.IndexContent(ctx, movie); err != nil {
				return err
			}
		}
		if len(nextKey) == 0 {
			break
		}
		startKey = nextKey
	}

	return nil
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

func buildRateLimitFactory(ctx context.Context, cfg Config) (ratelimit.Factory, *goredis.Client, error) {
	switch strings.ToLower(strings.TrimSpace(cfg.RateLimitBackend)) {
	case "", "memory":
		return ratelimit.MemoryFactory{}, nil, nil
	case "redis":
		dialCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
		defer cancel()
		client, err := redispkg.New(dialCtx, redispkg.Options{
			URL:          cfg.RedisURL,
			DialTimeout:  time.Duration(cfg.RedisDialTimeoutMS) * time.Millisecond,
			ReadTimeout:  time.Duration(cfg.RedisReadTimeoutMS) * time.Millisecond,
			WriteTimeout: time.Duration(cfg.RedisWriteTimeoutMS) * time.Millisecond,
			PoolSize:     cfg.RedisPoolSize,
		})
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

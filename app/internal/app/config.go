package app

import (
	"errors"
	"net/url"
	"strings"
)

const DefaultCORSOrigins = "http://localhost:3000,http://localhost:8443,https://localhost:8443,http://127.0.0.1:8443,https://127.0.0.1:8443"

type AWSCredentials struct {
	AccessKey       string
	SecretAccessKey string
	Region          string
}

type Config struct {
	Addr string
	Env  string

	HTTPMaxJSONBytes      int
	HTTPMaxMultipartBytes int64
	RouterTimeoutSecond   int

	TableName          string
	PersonTableName    string
	AttributeTableName string
	EncoderTableName   string

	EncoderContentIDIndex string
	AttributeNameIndex    string

	ContentCastTableName              string
	ContentAttributeTableName         string
	ContentVisibilityCreatedAtIndex   string
	ContentVisibilityContentTypeIndex string
	ContentVisibilityReleaseDateIndex string

	S3Bucket string

	CORSAllowedOrigins      []string
	CORSAllowPrivateNetwork bool

	RateLimitBackend        string
	RateLimitRedisFailMode  string
	RateLimitRedisTimeoutMS int
	RateLimitGlobalBurst    int
	RateLimitGlobalPerMin   int
	RateLimitEncoderBurst   int
	RateLimitEncoderPerMin  int
	RateLimitMovieBurst     int
	RateLimitMoviePerMin    int
	RateLimitPersonBurst    int
	RateLimitPersonPerMin   int

	RedisURL            string
	RedisDialTimeoutMS  int
	RedisReadTimeoutMS  int
	RedisWriteTimeoutMS int
	RedisPoolSize       int

	SearchEnabled          bool
	SearchProvider         string
	SearchEndpoint         string
	SearchUsername         string
	SearchPassword         string
	SearchContentIndexName string
	SearchPeopleIndexName  string
	SearchRequestTimeoutMS int

	AWS AWSCredentials
}

func (c Config) IsProd() bool {
	switch strings.ToLower(strings.TrimSpace(c.Env)) {
	case "prod", "production":
		return true
	}
	return false
}

func (c Config) Validate() error {
	switch strings.ToLower(strings.TrimSpace(c.Env)) {
	case "dev", "development", "staging", "prod", "production":
	default:
		return errors.New("APP_ENV must be one of: dev, staging, prod")
	}
	if strings.TrimSpace(c.Addr) == "" {
		return errors.New("PORT is required")
	}
	if c.HTTPMaxJSONBytes <= 0 {
		return errors.New("HTTP_MAX_JSON_BYTES must be > 0")
	}
	if c.HTTPMaxMultipartBytes <= 0 {
		return errors.New("HTTP_MAX_MULTIPART_BYTES must be > 0")
	}
	if c.RouterTimeoutSecond <= 0 {
		return errors.New("ROUTER_TIMEOUT_SECONDS must be > 0")
	}
	if strings.TrimSpace(c.S3Bucket) == "" {
		return errors.New("S3_BUCKET is required")
	}
	if strings.TrimSpace(c.AWS.Region) == "" {
		return errors.New("AWS_REGION is required")
	}
	if strings.TrimSpace(c.TableName) == "" ||
		strings.TrimSpace(c.PersonTableName) == "" ||
		strings.TrimSpace(c.AttributeTableName) == "" ||
		strings.TrimSpace(c.EncoderTableName) == "" {
		return errors.New("all DYNAMODB_*_TABLE values are required")
	}
	if strings.TrimSpace(c.EncoderContentIDIndex) == "" {
		return errors.New("DYNAMODB_ENCODER_CONTENT_ID_INDEX is required")
	}
	if strings.TrimSpace(c.ContentCastTableName) == "" {
		return errors.New("DYNAMODB_CONTENT_CAST_TABLE is required")
	}
	if strings.TrimSpace(c.ContentAttributeTableName) == "" {
		return errors.New("DYNAMODB_CONTENT_ATTRIBUTE_TABLE is required")
	}
	if strings.TrimSpace(c.ContentVisibilityCreatedAtIndex) == "" {
		return errors.New("DYNAMODB_CONTENT_VISIBILITY_CREATED_AT_INDEX is required")
	}
	if strings.TrimSpace(c.ContentVisibilityReleaseDateIndex) == "" {
		return errors.New("DYNAMODB_CONTENT_VISIBILITY_RELEASE_DATE_INDEX is required")
	}
	if c.SearchEnabled {
		if c.SearchRequestTimeoutMS <= 0 {
			return errors.New("SEARCH_REQUEST_TIMEOUT_MS must be > 0 when SEARCH_ENABLED=true")
		}
		switch strings.ToLower(strings.TrimSpace(c.SearchProvider)) {
		case "opensearch":
			if strings.TrimSpace(c.SearchEndpoint) == "" {
				return errors.New("SEARCH_ENDPOINT is required when SEARCH_ENABLED=true")
			}
			if strings.TrimSpace(c.SearchContentIndexName) == "" || strings.TrimSpace(c.SearchPeopleIndexName) == "" {
				return errors.New("SEARCH_CONTENT_INDEX and SEARCH_PEOPLE_INDEX are required when SEARCH_ENABLED=true")
			}
		case "", "none":
			return errors.New("SEARCH_PROVIDER must be set to 'opensearch' when SEARCH_ENABLED=true")
		default:
			return errors.New("SEARCH_PROVIDER must be one of: opensearch")
		}
	}
	switch strings.ToLower(strings.TrimSpace(c.RateLimitBackend)) {
	case "", "memory":
	case "redis":
		if c.RateLimitRedisTimeoutMS <= 0 {
			return errors.New("RATE_LIMIT_REDIS_TIMEOUT_MS must be > 0 when RATE_LIMIT_BACKEND=redis")
		}
		if strings.TrimSpace(c.RedisURL) == "" {
			return errors.New("REDIS_URL is required when RATE_LIMIT_BACKEND=redis")
		}
	default:
		return errors.New("RATE_LIMIT_BACKEND must be one of: memory, redis")
	}
	if strings.TrimSpace(c.RedisURL) != "" {
		if c.RedisDialTimeoutMS < 0 || c.RedisReadTimeoutMS < 0 || c.RedisWriteTimeoutMS < 0 {
			return errors.New("REDIS_*_TIMEOUT_MS must be >= 0")
		}
		if c.RedisPoolSize < 0 {
			return errors.New("REDIS_POOL_SIZE must be >= 0")
		}
	}
	rateLimitBuckets := []struct {
		name    string
		burst   int
		perMin  int
		envName string
	}{
		{"global", c.RateLimitGlobalBurst, c.RateLimitGlobalPerMin, "RATELIMIT_GLOBAL"},
		{"encoder", c.RateLimitEncoderBurst, c.RateLimitEncoderPerMin, "RATELIMIT_ENCODER_WRITE"},
		{"movie", c.RateLimitMovieBurst, c.RateLimitMoviePerMin, "RATELIMIT_MOVIE_WRITE"},
		{"person", c.RateLimitPersonBurst, c.RateLimitPersonPerMin, "RATELIMIT_PERSON_WRITE"},
	}
	for _, b := range rateLimitBuckets {
		if b.burst <= 0 || b.perMin <= 0 {
			return errors.New(b.envName + "_BURST and " + b.envName + "_PER_MIN must be > 0")
		}
	}
	prod := c.IsProd()
	for _, origin := range c.CORSAllowedOrigins {
		if origin == "*" {
			return errors.New("CORS_ALLOWED_ORIGINS must not contain '*' when credentials are allowed")
		}
		u, perr := url.Parse(origin)
		if perr != nil || u.Host == "" || (u.Scheme != "http" && u.Scheme != "https") {
			return errors.New("CORS_ALLOWED_ORIGINS contains invalid URL: " + origin)
		}
		if prod && u.Scheme != "https" {
			return errors.New("CORS_ALLOWED_ORIGINS in prod must use https:// scheme: " + origin)
		}
	}
	return nil
}

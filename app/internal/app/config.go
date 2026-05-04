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

	TableName          string
	PersonTableName    string
	AttributeTableName string
	EncoderTableName   string

	EncoderContentIDIndex string
	AttributeNameIndex    string

	S3Bucket string

	CORSAllowedOrigins      []string
	CORSAllowPrivateNetwork bool

	// Rate-limit backend selection. "memory" (default) keeps state in-process;
	// "redis" enables a shared, multi-replica-safe backend. Redis* fields are
	// only consulted when RateLimitBackend == "redis".
	RateLimitBackend        string
	RateLimitRedisFailMode  string
	RateLimitRedisTimeoutMS int

	RedisURL string

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
	switch strings.ToLower(strings.TrimSpace(c.RateLimitBackend)) {
	case "", "memory":
	case "redis":
		if strings.TrimSpace(c.RedisURL) == "" {
			return errors.New("REDIS_URL is required when RATE_LIMIT_BACKEND=redis")
		}
	default:
		return errors.New("RATE_LIMIT_BACKEND must be one of: memory, redis")
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

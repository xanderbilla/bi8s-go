package app

import (
	"strings"
	"testing"
)

func setRequiredEnv(t *testing.T) {
	t.Helper()
	t.Setenv("APP_ENV", "dev")
	t.Setenv("PORT", ":8080")
	t.Setenv("AWS_REGION", "us-east-1")
	t.Setenv("S3_BUCKET", "bucket")
	t.Setenv("DYNAMODB_CONTENT_TABLE", "content")
	t.Setenv("DYNAMODB_PERSON_TABLE", "people")
	t.Setenv("DYNAMODB_ATTRIBUTE_TABLE", "attrs")
	t.Setenv("DYNAMODB_ENCODER_TABLE", "encoder")
	t.Setenv("DYNAMODB_ENCODER_CONTENT_ID_INDEX", "content-id-index")
	t.Setenv("DYNAMODB_CONTENT_CAST_TABLE", "cast")
	t.Setenv("DYNAMODB_CONTENT_ATTRIBUTE_TABLE", "content-attribute")
	t.Setenv("DYNAMODB_CONTENT_VISIBILITY_CREATED_AT_INDEX", "visibility-createdAt-index")
	t.Setenv("DYNAMODB_CONTENT_VISIBILITY_RELEASE_DATE_INDEX", "visibility-releaseDate-index")
}

func TestLoadConfigFromEnv_DefaultsAndStrictValues(t *testing.T) {
	setRequiredEnv(t)
	t.Setenv("CORS_ALLOW_PRIVATE_NETWORK", "")
	t.Setenv("CORS_ALLOWED_ORIGINS", "http://localhost:3000")
	t.Setenv("RATELIMIT_GLOBAL_BURST", "101")
	t.Setenv("RATELIMIT_GLOBAL_PER_MIN", "102")

	cfg, err := LoadConfigFromEnv()
	if err != nil {
		t.Fatalf("LoadConfigFromEnv() error = %v", err)
	}
	if cfg.HTTPMaxJSONBytes <= 0 {
		t.Fatalf("HTTPMaxJSONBytes must be positive, got %d", cfg.HTTPMaxJSONBytes)
	}
	if cfg.HTTPMaxMultipartBytes <= 0 {
		t.Fatalf("HTTPMaxMultipartBytes must be positive, got %d", cfg.HTTPMaxMultipartBytes)
	}
	if cfg.RouterTimeoutSecond <= 0 {
		t.Fatalf("RouterTimeoutSecond must be positive, got %d", cfg.RouterTimeoutSecond)
	}
	if cfg.RateLimitGlobalBurst != 101 || cfg.RateLimitGlobalPerMin != 102 {
		t.Fatalf("unexpected ratelimit values: burst=%d rpm=%d", cfg.RateLimitGlobalBurst, cfg.RateLimitGlobalPerMin)
	}
	if cfg.CORSAllowPrivateNetwork {
		t.Fatal("CORSAllowPrivateNetwork should default to false")
	}
}

func TestLoadConfigFromEnv_InvalidIntFails(t *testing.T) {
	setRequiredEnv(t)
	t.Setenv("RATELIMIT_GLOBAL_PER_MIN", "not-an-int")
	if _, err := LoadConfigFromEnv(); err == nil {
		t.Fatal("expected error for invalid integer env value")
	}
}

func TestLoadConfigFromEnv_InvalidBoolFails(t *testing.T) {
	setRequiredEnv(t)
	t.Setenv("CORS_ALLOW_PRIVATE_NETWORK", "not-a-bool")
	_, err := LoadConfigFromEnv()
	if err == nil {
		t.Fatal("expected error for invalid boolean env value")
	}
	if !strings.Contains(err.Error(), "CORS_ALLOW_PRIVATE_NETWORK") {
		t.Fatalf("unexpected error: %v", err)
	}
}

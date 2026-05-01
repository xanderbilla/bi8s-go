package app

import (
	"strings"
	"testing"
)

func baseConfig() Config {
	return Config{
		Addr:                  ":8080",
		Env:                   "dev",
		TableName:             "movies",
		PersonTableName:       "people",
		AttributeTableName:    "attrs",
		EncoderTableName:      "encoder",
		EncoderContentIDIndex: "encoder-content-id-index",
		S3Bucket:              "bucket",
		AWS:                   AWSCredentials{Region: "us-east-1"},
	}
}

func TestConfigValidate_OK(t *testing.T) {
	if err := baseConfig().Validate(); err != nil {
		t.Fatalf("expected ok, got %v", err)
	}
}

func TestConfigValidate_Errors(t *testing.T) {
	cases := []struct {
		name string
		mut  func(*Config)
		want string
	}{
		{"bad env", func(c *Config) { c.Env = "qa" }, "APP_ENV"},
		{"missing port", func(c *Config) { c.Addr = "" }, "PORT"},
		{"missing bucket", func(c *Config) { c.S3Bucket = "" }, "S3_BUCKET"},
		{"missing region", func(c *Config) { c.AWS.Region = "" }, "AWS_REGION"},
		{"missing table", func(c *Config) { c.TableName = "" }, "DYNAMODB"},
		{"missing encoder index", func(c *Config) { c.EncoderContentIDIndex = "" }, "DYNAMODB_ENCODER_CONTENT_ID_INDEX"},
		{"cors star", func(c *Config) { c.CORSAllowedOrigins = []string{"*"} }, "must not contain '*'"},
		{"cors missing scheme", func(c *Config) { c.CORSAllowedOrigins = []string{"example.com"} }, "CORS_ALLOWED_ORIGINS"},
		{"cors invalid scheme", func(c *Config) { c.CORSAllowedOrigins = []string{"ftp://example.com"} }, "CORS_ALLOWED_ORIGINS"},
		{"prod requires https", func(c *Config) {
			c.Env = "prod"
			c.CORSAllowedOrigins = []string{"http://example.com"}
		}, "https://"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cfg := baseConfig()
			tc.mut(&cfg)
			err := cfg.Validate()
			if err == nil {
				t.Fatalf("expected error containing %q", tc.want)
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("err %q missing %q", err.Error(), tc.want)
			}
		})
	}
}

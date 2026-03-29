package app

import (
	awsinfra "github.com/xanderbilla/bi8s-go/internal/aws"
	"github.com/xanderbilla/bi8s-go/internal/service"
)

// AWSConfig holds the credentials and region needed to connect to AWS.
// These are typically loaded from environment variables at startup.
type AWSConfig struct {
	AccessKey       string
	SecretAccessKey string
	Region          string
}

// Config is the top-level configuration for the entire application.
// It holds the server address, the runtime environment name, and AWS settings.
type Config struct {
	Addr      string // e.g. ":8080"
	Env       string // e.g. "prod", "dev"
	TableName string // DynamoDB table name, e.g. "bi8s-dev"
	S3Bucket  string // S3 bucket for movie posters
	S3Prefix  string // S3 key prefix for posters, e.g. "movies"
	AWS       AWSConfig
}

// Application is the central dependency container constructed natively at startup.
// It acts solely as an aggregation registry for wiring explicit dependencies into specific handlers and services.
type Application struct {
	Config       Config
	Clients      *awsinfra.Clients
	MovieService *service.MovieService
}

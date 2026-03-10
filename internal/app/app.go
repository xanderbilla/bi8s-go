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
	AWS       AWSConfig
}

// Application is the central dependency container.
// Handlers receive a pointer to this so they can access config, AWS clients, and services.
type Application struct {
	Config       Config
	Clients      *awsinfra.Clients
	MovieService *service.MovieService
}

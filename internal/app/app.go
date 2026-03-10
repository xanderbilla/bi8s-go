package app

import "github.com/aws/aws-sdk-go-v2/service/dynamodb"

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
	Addr string    // e.g. ":8080"
	Env  string    // e.g. "prod", "dev"
	AWS  AWSConfig
}

// Application is the central dependency container.
// Handlers and services receive a pointer to this so they can access
// shared resources like config and the database client without globals.
type Application struct {
	Config Config
	DB     *dynamodb.Client
}

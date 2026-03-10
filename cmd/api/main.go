package main

import (
	"log"
	"net/http"
	"time"

	"github.com/xanderbilla/bi8s-go/internal/app"
	"github.com/xanderbilla/bi8s-go/internal/aws"
	"github.com/xanderbilla/bi8s-go/internal/env"
	transport "github.com/xanderbilla/bi8s-go/internal/http"
	"github.com/xanderbilla/bi8s-go/internal/repository"
	"github.com/xanderbilla/bi8s-go/internal/service"
)

func main() {

	// TODO: implement graceful shutdown using channel and goroutine
	if err := run(); err != nil {
		log.Fatal(err)
	}
}

// run contains all startup logic and returns an error if anything goes wrong.
// Keeping it separate from main() makes it easy to test and reason about.
func run() error {
	// Build the app config from environment variables.
	// If a variable isn't set, we fall back to safe defaults.
	cfg := app.Config{
		Addr:      ":8080",
		Env:       env.GetString("ENV", "prod"),
		TableName: env.GetString("DYNAMODB_TABLE", "bi8s-dev"),
		AWS: app.AWSConfig{
			AccessKey:       env.GetString("AWS_ACCESS_KEY_ID", ""),
			SecretAccessKey: env.GetString("AWS_SECRET_ACCESS_KEY", ""),
			Region:          env.GetString("AWS_REGION", "us-east-1"),
		},
	}

	// Connect to AWS using the credentials from config.
	// If static keys are empty, the SDK falls back to the default credential chain
	// (IAM role, ~/.aws/credentials, etc.).
	awsCfg, err := aws.AWSConfig(cfg.AWS.Region, cfg.AWS.AccessKey, cfg.AWS.SecretAccessKey)
	if err != nil {
		return err
	}

	// Create all AWS clients (DynamoDB, and S3 when needed) from the AWS config.
	dynamoClient := aws.NewClients(awsCfg)

	// Build the repository and service layers.
	// The repo talks directly to DynamoDB; the service sits on top and owns business logic.
	movieRepo := repository.NewMovieRepository(dynamoClient.Dynamo, cfg.TableName)
	movieService := service.NewMovieService(movieRepo)

	// Wire everything together into a single Application struct.
	// This gets passed around to handlers so they have access to config, AWS clients, and services.
	application := &app.Application{
		Config:       cfg,
		Clients:      dynamoClient,
		MovieService: movieService,
	}

	// Build the HTTP router with all routes and middleware attached.
	mux := transport.Mount(application)

	// Configure the HTTP server with sensible timeouts to avoid slow/stuck connections.
	srv := http.Server{
		Addr:         application.Config.Addr,
		Handler:      mux,
		WriteTimeout: 30 * time.Second, // max time to write a full response
		ReadTimeout:  10 * time.Second, // max time to read the full request
		IdleTimeout:  time.Minute,      // how long to keep idle keep-alive connections open
	}

	log.Printf("server started on %s", application.Config.Addr)

	return srv.ListenAndServe()
}

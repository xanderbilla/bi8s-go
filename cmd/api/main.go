package main

import (
	"log"
	"net/http"
	"time"

	"github.com/xanderbilla/bi8s-go/internal/app"
	"github.com/xanderbilla/bi8s-go/internal/aws"
	"github.com/xanderbilla/bi8s-go/internal/env"
	httptransport "github.com/xanderbilla/bi8s-go/internal/http"
	"github.com/xanderbilla/bi8s-go/internal/repository"
)

func main() {
	// Build the app config from environment variables.
	// If a variable isn't set, we fall back to safe defaults.
	cfg := app.Config{
		Addr: ":8080",
		Env:  env.GetString("ENV", "prod"),
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
		log.Fatal(err)
	}

	// Create a DynamoDB client using the AWS config we just built.
	dynamoClient := aws.NewDynamoClient(awsCfg)

	// Wire everything together into a single Application struct.
	// This gets passed around to handlers so they have access to config and DB.
	app := &app.Application{
		Config: cfg,
		DB:     dynamoClient,
	}

	// Set up the movie repository, pointing it at the "bi8s-dev" DynamoDB table.
	movieRepo := repository.NewMovieRepository(app.DB, "bi8s-dev")

	// Build the HTTP router with all routes and middleware attached.
	mux := httptransport.Mount(app, movieRepo)

	// Configure the HTTP server with sensible timeouts to avoid slow/stuck connections.
	srv := http.Server{
		Addr:         app.Config.Addr,
		Handler:      mux,
		WriteTimeout: 30 * time.Second, // max time to write a full response
		ReadTimeout:  10 * time.Second, // max time to read the full request
		IdleTimeout:  time.Minute,      // how long to keep idle keep-alive connections open
	}

	log.Printf("server started on %s", app.Config.Addr)

	// ListenAndServe blocks forever. log.Fatal will print the error and exit if it crashes.
	log.Fatal(srv.ListenAndServe())
}

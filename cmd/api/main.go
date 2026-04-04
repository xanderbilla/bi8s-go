package main

import (
	"errors"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/xanderbilla/bi8s-go/internal/app"
	"github.com/xanderbilla/bi8s-go/internal/aws"
	"github.com/xanderbilla/bi8s-go/internal/env"
	transport "github.com/xanderbilla/bi8s-go/internal/http"
	"github.com/xanderbilla/bi8s-go/internal/repository"
	"github.com/xanderbilla/bi8s-go/internal/service"
	"github.com/xanderbilla/bi8s-go/internal/storage"
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
	defaultCORSOrigins := "http://localhost:3000,http://localhost:8443,https://localhost:8443,http://127.0.0.1:8443,https://127.0.0.1:8443,http://13.217.109.221:8443,https://13.217.109.221:8443"

	// Build the app config from environment variables.
	// If a variable isn't set, we fall back to safe defaults.
	cfg := app.Config{
		Addr:                    ":8080",
		Env:                     env.GetString("APP_ENV", "prod"),
		TableName:               env.GetString("DYNAMODB_MOVIE_TABLE", "bi8s-dev"),
		PersonTableName:         env.GetString("DYNAMODB_PERSON_TABLE", "bi8s-person-dev"),
		AttributeTableName:      env.GetString("DYNAMODB_ATTRIBUTE_TABLE", "bi8s-attribute-dev"),
		EncoderTableName:        env.GetString("DYNAMODB_ENCODER_TABLE", "bi8s-video-dev"),
		S3Bucket:                env.GetString("S3_BUCKET", ""),
		CORSAllowedOrigins:      parseCommaSeparated(env.GetString("CORS_ALLOWED_ORIGINS", defaultCORSOrigins)),
		CORSAllowPrivateNetwork: strings.EqualFold(env.GetString("CORS_ALLOW_PRIVATE_NETWORK", "true"), "true"),
		AWS: app.AWSConfig{
			AccessKey:       env.GetString("AWS_ACCESS_KEY_ID", ""),
			SecretAccessKey: env.GetString("AWS_SECRET_ACCESS_KEY", ""),
			Region:          env.GetString("AWS_REGION", "us-east-1"),
		},
	}

	if cfg.S3Bucket == "" {
		return errors.New("S3_BUCKET is required for poster uploads")
	}

	// Connect to AWS using the credentials from config.
	// If static keys are empty, the SDK falls back to the default credential chain
	// (IAM role, ~/.aws/credentials, etc.).
	awsCfg, err := aws.AWSConfig(cfg.AWS.Region, cfg.AWS.AccessKey, cfg.AWS.SecretAccessKey)
	if err != nil {
		return err
	}

	// Create all AWS clients (DynamoDB, and S3 when needed) from the AWS config.
	awsClient := aws.NewClients(awsCfg)

	// Build the repository and service layers.
	// The repo talks directly to DynamoDB; the service sits on top and owns business logic.
	// Use generic file uploader implementation backed by S3.
	fileUploader := storage.NewS3FileUploader(awsClient.S3, cfg.S3Bucket)

	attributeRepo := repository.NewAttributeDynamoRepository(awsClient.Dynamo, cfg.AttributeTableName)
	attributeService := service.NewAttributeService(attributeRepo)

	personRepo := repository.NewPersonDynamoRepository(awsClient.Dynamo, cfg.PersonTableName)
	personService := service.NewPersonService(personRepo, attributeRepo, fileUploader)

	movieRepo := repository.NewMovieRepository(awsClient.Dynamo, cfg.TableName)
	encoderRepo := repository.NewEncoderRepository(awsClient.Dynamo, cfg.EncoderTableName)
	movieService := service.NewMovieService(movieRepo, personRepo, attributeRepo, encoderRepo, fileUploader)
	encoderService := service.NewEncoderService(encoderRepo, fileUploader)

	// Wire everything together into a single Application structure acting as a central registry natively.
	// It is passed only into the router Mount to orchestrate specific dependency injection bindings.
	application := &app.Application{
		Config:           cfg,
		Clients:          awsClient,
		MovieService:     movieService,
		PersonService:    personService,
		AttributeService: attributeService,
		EncoderService:   encoderService,
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

func parseCommaSeparated(val string) []string {
	parts := strings.Split(val, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		trimmed := strings.TrimSpace(p)
		if trimmed != "" {
			out = append(out, trimmed)
		}
	}
	return out
}

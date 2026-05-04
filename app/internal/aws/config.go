package aws

import (
	"context"
	"errors"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"go.opentelemetry.io/contrib/instrumentation/github.com/aws/aws-sdk-go-v2/otelaws"
)

func LoadConfig(ctx context.Context, region, accessKey, secretKey string) (aws.Config, error) {
	if region == "" {
		return aws.Config{}, errors.New("aws region is required")
	}
	if ctx == nil {
		ctx = context.Background()
	}

	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	loadOpts := []func(*config.LoadOptions) error{
		config.WithRegion(region),
		config.WithRetryMode(aws.RetryModeAdaptive),
		config.WithRetryMaxAttempts(5),
	}
	if accessKey != "" && secretKey != "" {
		loadOpts = append(loadOpts,
			config.WithCredentialsProvider(
				credentials.NewStaticCredentialsProvider(accessKey, secretKey, ""),
			),
		)
	}

	cfg, err := config.LoadDefaultConfig(ctx, loadOpts...)
	if err != nil {
		return aws.Config{}, err
	}

	// Instrument all AWS SDK calls so each operation produces an OTel span
	// linked to the parent HTTP request. Telemetry is exported via the OTel
	// collector configured by internal/observability.
	otelaws.AppendMiddlewares(&cfg.APIOptions)

	return cfg, nil
}

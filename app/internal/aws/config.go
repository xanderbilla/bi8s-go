package aws

import (
	"context"
	"errors"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
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

	if accessKey != "" && secretKey != "" {
		return config.LoadDefaultConfig(
			ctx,
			config.WithRegion(region),
			config.WithRetryMode(aws.RetryModeAdaptive),
			config.WithRetryMaxAttempts(5),
			config.WithCredentialsProvider(
				credentials.NewStaticCredentialsProvider(accessKey, secretKey, ""),
			),
		)
	}

	return config.LoadDefaultConfig(
		ctx,
		config.WithRegion(region),
		config.WithRetryMode(aws.RetryModeAdaptive),
		config.WithRetryMaxAttempts(5),
	)
}

package aws

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
)

// AWSConfig builds an aws.Config that the rest of the app uses to talk to AWS services.
//
// Two modes:
//   - If accessKey and secretKey are provided, it uses them directly (good for local dev).
//   - If they're empty, it falls back to the default AWS credential chain:
//     IAM instance role → ~/.aws/credentials → environment variables, etc.
func AWSConfig(region, accessKey, secretKey string) (aws.Config, error) {

	// Explicit credentials take priority — useful when running locally without an IAM role.
	if accessKey != "" && secretKey != "" {

		return config.LoadDefaultConfig(
			context.Background(),
			config.WithRegion(region),
			config.WithCredentialsProvider(
				credentials.NewStaticCredentialsProvider(accessKey, secretKey, ""),
			),
		)
	}

	// No explicit credentials — let the SDK figure it out from the environment.
	return config.LoadDefaultConfig(
		context.Background(),
		config.WithRegion(region),
	)
}

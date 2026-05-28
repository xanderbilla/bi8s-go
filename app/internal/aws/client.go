package aws

import (
	"context"
	"errors"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awshttp "github.com/aws/aws-sdk-go-v2/aws/transport/http"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

type Clients struct {
	Dynamo *dynamodb.Client
	S3     *s3.Client
}

func NewClients(cfg aws.Config) *Clients {
	dynamoHTTP := awshttp.NewBuildableClient().WithTransportOptions(func(t *http.Transport) {
		t.MaxIdleConns = 100
		t.MaxIdleConnsPerHost = 100
		t.IdleConnTimeout = 90 * time.Second
	})
	return &Clients{
		Dynamo: dynamodb.NewFromConfig(cfg, func(o *dynamodb.Options) {
			o.HTTPClient = dynamoHTTP
		}),
		S3:     s3.NewFromConfig(cfg),
	}
}

// NewB2S3Client creates an S3-compatible client pointing at Backblaze B2.
// keyID is the B2 application key ID, appKey is the application key,
// endpoint is the B2 S3-compatible endpoint (e.g. https://s3.us-east-005.backblazeb2.com).
func NewB2S3Client(keyID, appKey, endpoint string) (*s3.Client, error) {
	if keyID == "" || appKey == "" || endpoint == "" {
		return nil, errors.New("B2_KEY_ID, B2_APPLICATION_KEY, and B2_ENDPOINT are all required")
	}

	// Extract the region from the B2 endpoint hostname.
	// e.g. https://s3.us-east-005.backblazeb2.com → us-east-005
	region := b2RegionFromEndpoint(endpoint)

	cfg, err := config.LoadDefaultConfig(context.Background(),
		config.WithRegion(region),
		config.WithCredentialsProvider(
			credentials.NewStaticCredentialsProvider(keyID, appKey, ""),
		),
	)
	if err != nil {
		return nil, err
	}
	return s3.NewFromConfig(cfg, func(o *s3.Options) {
		o.BaseEndpoint = aws.String(endpoint)
		o.UsePathStyle = true
	}), nil
}

// b2RegionFromEndpoint parses the region segment out of a B2 S3 endpoint URL.
// "https://s3.us-east-005.backblazeb2.com" → "us-east-005"
// Falls back to "us-east-005" (most common B2 region) if parsing fails.
func b2RegionFromEndpoint(endpoint string) string {
	u, err := url.Parse(endpoint)
	if err != nil {
		return "us-east-005"
	}
	// hostname: s3.<region>.backblazeb2.com
	host := strings.TrimSuffix(u.Hostname(), ".backblazeb2.com")
	if after, ok := strings.CutPrefix(host, "s3."); ok && after != "" {
		return after
	}
	return "us-east-005"
}

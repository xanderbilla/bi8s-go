// Package aws manages all Amazon Web Services configurations and client bindings securely.
package aws

import (
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

// Clients holds all AWS service clients in one place.
// Add new clients here (e.g. S3) as the app grows — avoids passing multiple clients separately.
type Clients struct {
	Dynamo *dynamodb.Client
	S3     *s3.Client
}

// NewClients initialises all AWS service clients from the given config.
// Call this once at startup and share the result — all clients are safe for concurrent use.
func NewClients(cfg aws.Config) *Clients {
	return &Clients{
		Dynamo: dynamodb.NewFromConfig(cfg),
		S3:     s3.NewFromConfig(cfg),
	}
}

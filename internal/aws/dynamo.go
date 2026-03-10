package aws

import (
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
)

// NewDynamoClient creates a DynamoDB client from the given AWS config.
// Call this once at startup and share the client — it's safe for concurrent use.
func NewDynamoClient(cfg aws.Config) *dynamodb.Client {
	return dynamodb.NewFromConfig(cfg)
}

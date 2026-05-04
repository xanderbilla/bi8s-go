package aws

import (
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

type Clients struct {
	Dynamo *dynamodb.Client
	S3     *s3.Client
}

func NewClients(cfg aws.Config) *Clients {
	return &Clients{
		Dynamo: dynamodb.NewFromConfig(cfg),
		S3:     s3.NewFromConfig(cfg),
	}
}

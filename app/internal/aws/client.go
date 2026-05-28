package aws

import (
	"net/http"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awshttp "github.com/aws/aws-sdk-go-v2/aws/transport/http"
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

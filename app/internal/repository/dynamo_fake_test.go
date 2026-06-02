package repository

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
)

// fakeDynamo is a test double for DynamoAPI used across repository tests.
type fakeDynamo struct {
	queryOuts []*dynamodb.QueryOutput
	queryErrs []error
	queryCall int
	queryIns  []*dynamodb.QueryInput

	scanOuts []*dynamodb.ScanOutput
	scanErrs []error
	scanCall int

	getOuts []*dynamodb.GetItemOutput
	getErrs []error
	getCall int

	putOuts []*dynamodb.PutItemOutput
	putErrs []error
	putCall int

	deleteOuts []*dynamodb.DeleteItemOutput
	deleteErrs []error
	deleteCall int
}

func (f *fakeDynamo) Query(_ context.Context, in *dynamodb.QueryInput, _ ...func(*dynamodb.Options)) (*dynamodb.QueryOutput, error) {
	i := f.queryCall
	f.queryCall++
	f.queryIns = append(f.queryIns, in)
	if i < len(f.queryErrs) && f.queryErrs[i] != nil {
		return nil, f.queryErrs[i]
	}
	if i < len(f.queryOuts) {
		return f.queryOuts[i], nil
	}
	return &dynamodb.QueryOutput{}, nil
}

func (f *fakeDynamo) Scan(_ context.Context, _ *dynamodb.ScanInput, _ ...func(*dynamodb.Options)) (*dynamodb.ScanOutput, error) {
	i := f.scanCall
	f.scanCall++
	if i < len(f.scanErrs) && f.scanErrs[i] != nil {
		return nil, f.scanErrs[i]
	}
	if i < len(f.scanOuts) {
		return f.scanOuts[i], nil
	}
	return &dynamodb.ScanOutput{}, nil
}

func (f *fakeDynamo) GetItem(_ context.Context, _ *dynamodb.GetItemInput, _ ...func(*dynamodb.Options)) (*dynamodb.GetItemOutput, error) {
	i := f.getCall
	f.getCall++
	if i < len(f.getErrs) && f.getErrs[i] != nil {
		return nil, f.getErrs[i]
	}
	if i < len(f.getOuts) {
		return f.getOuts[i], nil
	}
	return &dynamodb.GetItemOutput{}, nil
}

func (f *fakeDynamo) PutItem(_ context.Context, _ *dynamodb.PutItemInput, _ ...func(*dynamodb.Options)) (*dynamodb.PutItemOutput, error) {
	i := f.putCall
	f.putCall++
	if i < len(f.putErrs) && f.putErrs[i] != nil {
		return nil, f.putErrs[i]
	}
	if i < len(f.putOuts) {
		return f.putOuts[i], nil
	}
	return &dynamodb.PutItemOutput{}, nil
}

func (f *fakeDynamo) DeleteItem(_ context.Context, _ *dynamodb.DeleteItemInput, _ ...func(*dynamodb.Options)) (*dynamodb.DeleteItemOutput, error) {
	i := f.deleteCall
	f.deleteCall++
	if i < len(f.deleteErrs) && f.deleteErrs[i] != nil {
		return nil, f.deleteErrs[i]
	}
	if i < len(f.deleteOuts) {
		return f.deleteOuts[i], nil
	}
	return &dynamodb.DeleteItemOutput{}, nil
}

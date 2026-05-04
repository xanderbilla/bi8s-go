package repository

import (
	"context"
	"errors"
	"testing"

	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
)

func attrItem(id, name string) map[string]types.AttributeValue {
	return map[string]types.AttributeValue{
		"id":   &types.AttributeValueMemberS{Value: id},
		"name": &types.AttributeValueMemberS{Value: name},
	}
}

func TestAttributeRepository_GetByName_GSIHit(t *testing.T) {
	t.Parallel()

	fake := &fakeDynamo{
		queryOuts: []*dynamodb.QueryOutput{
			{Items: []map[string]types.AttributeValue{attrItem("a1", "Action")}},
		},
	}
	repo := &AttributeDynamoRepository{
		BaseRepository: NewBaseRepository(fake, "attributes"),
		nameIndex:      "name-index",
	}

	got, err := repo.GetByName(context.Background(), "Action")
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if got == nil || got.ID != "a1" || got.Name != "Action" {
		t.Fatalf("unexpected attribute: %+v", got)
	}
	if fake.queryCall != 1 {
		t.Fatalf("expected 1 query call, got %d", fake.queryCall)
	}
	if fake.scanCall != 0 {
		t.Fatalf("expected 0 scan calls, got %d", fake.scanCall)
	}
	if fake.queryIns[0].IndexName == nil || *fake.queryIns[0].IndexName != "name-index" {
		t.Fatalf("expected IndexName=name-index, got %v", fake.queryIns[0].IndexName)
	}
}

func TestAttributeRepository_GetByName_GSIErrorFallsBackToScan(t *testing.T) {
	t.Parallel()

	fake := &fakeDynamo{
		queryErrs: []error{errors.New("transient gsi error")},
		scanOuts: []*dynamodb.ScanOutput{
			{Items: []map[string]types.AttributeValue{attrItem("a2", "Drama")}},
		},
	}
	repo := &AttributeDynamoRepository{
		BaseRepository: NewBaseRepository(fake, "attributes"),
		nameIndex:      "name-index",
	}

	got, err := repo.GetByName(context.Background(), "Drama")
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if got == nil || got.ID != "a2" {
		t.Fatalf("expected fallback to return a2, got %+v", got)
	}
	if fake.queryCall != 1 || fake.scanCall != 1 {
		t.Fatalf("expected 1 query + 1 scan, got query=%d scan=%d", fake.queryCall, fake.scanCall)
	}
}

func TestAttributeRepository_GetByName_NoIndexUsesScan(t *testing.T) {
	t.Parallel()

	fake := &fakeDynamo{
		scanOuts: []*dynamodb.ScanOutput{
			{Items: []map[string]types.AttributeValue{attrItem("a3", "Comedy")}},
		},
	}
	repo := &AttributeDynamoRepository{
		BaseRepository: NewBaseRepository(fake, "attributes"),
		nameIndex:      "",
	}

	got, err := repo.GetByName(context.Background(), "Comedy")
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if got == nil || got.ID != "a3" {
		t.Fatalf("expected a3, got %+v", got)
	}
	if fake.queryCall != 0 {
		t.Fatalf("expected no query calls, got %d", fake.queryCall)
	}
	if fake.scanCall != 1 {
		t.Fatalf("expected 1 scan call, got %d", fake.scanCall)
	}
}

func TestAttributeRepository_GetByName_NotFoundReturnsNil(t *testing.T) {
	t.Parallel()

	fake := &fakeDynamo{
		queryOuts: []*dynamodb.QueryOutput{{Items: nil}},
		scanOuts:  []*dynamodb.ScanOutput{{Items: nil}},
	}
	repo := &AttributeDynamoRepository{
		BaseRepository: NewBaseRepository(fake, "attributes"),
		nameIndex:      "name-index",
	}

	got, err := repo.GetByName(context.Background(), "missing")
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if got != nil {
		t.Fatalf("expected nil, got %+v", got)
	}
	// queryByName returned (nil, nil) so scanByName should also be invoked.
	if fake.queryCall != 1 || fake.scanCall != 1 {
		t.Fatalf("expected 1 query + 1 scan, got query=%d scan=%d", fake.queryCall, fake.scanCall)
	}
}

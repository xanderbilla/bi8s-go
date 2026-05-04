package repository

import (
	"context"
	"errors"
	"testing"

	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/xanderbilla/bi8s-go/internal/model"
)

func encoderJobFixture(jobID, contentID string) model.EncoderJob {
	return model.EncoderJob{
		JobID:     jobID,
		ContentID: contentID,
	}
}

type fakeDynamo struct {
	queryOuts []*dynamodb.QueryOutput
	queryErrs []error
	queryCall int
	queryIns  []*dynamodb.QueryInput

	scanOuts []*dynamodb.ScanOutput
	scanErrs []error
	scanCall int
	scanIns  []*dynamodb.ScanInput

	getOut *dynamodb.GetItemOutput
	getErr error
	putErr error
	delErr error
}

func (f *fakeDynamo) Query(_ context.Context, in *dynamodb.QueryInput, _ ...func(*dynamodb.Options)) (*dynamodb.QueryOutput, error) {

	cp := *in
	f.queryIns = append(f.queryIns, &cp)
	i := f.queryCall
	f.queryCall++
	if i < len(f.queryErrs) && f.queryErrs[i] != nil {
		return nil, f.queryErrs[i]
	}
	if i < len(f.queryOuts) {
		return f.queryOuts[i], nil
	}
	return &dynamodb.QueryOutput{}, nil
}

func (f *fakeDynamo) Scan(_ context.Context, in *dynamodb.ScanInput, _ ...func(*dynamodb.Options)) (*dynamodb.ScanOutput, error) {
	cp := *in
	f.scanIns = append(f.scanIns, &cp)
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
	if f.getErr != nil {
		return nil, f.getErr
	}
	if f.getOut != nil {
		return f.getOut, nil
	}
	return &dynamodb.GetItemOutput{}, nil
}

func (f *fakeDynamo) PutItem(_ context.Context, _ *dynamodb.PutItemInput, _ ...func(*dynamodb.Options)) (*dynamodb.PutItemOutput, error) {
	if f.putErr != nil {
		return nil, f.putErr
	}
	return &dynamodb.PutItemOutput{}, nil
}

func (f *fakeDynamo) DeleteItem(_ context.Context, _ *dynamodb.DeleteItemInput, _ ...func(*dynamodb.Options)) (*dynamodb.DeleteItemOutput, error) {
	if f.delErr != nil {
		return nil, f.delErr
	}
	return &dynamodb.DeleteItemOutput{}, nil
}

func jobItem(id, contentID string) map[string]types.AttributeValue {
	return map[string]types.AttributeValue{
		"id":        &types.AttributeValueMemberS{Value: id},
		"contentId": &types.AttributeValueMemberS{Value: contentID},
	}
}

func TestEncoderRepository_GetByContentId_QueryHappyPath(t *testing.T) {
	t.Parallel()

	fake := &fakeDynamo{
		queryOuts: []*dynamodb.QueryOutput{
			{Items: []map[string]types.AttributeValue{jobItem("j1", "c1"), jobItem("j2", "c1")}},
		},
	}
	repo := &DynamoEncoderRepository{
		BaseRepository: NewBaseRepository(fake, "encoder"),
		contentIDIndex: "contentId-index",
	}

	jobs, err := repo.GetByContentId(context.Background(), "c1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(jobs) != 2 {
		t.Fatalf("expected 2 jobs, got %d", len(jobs))
	}
	if fake.scanCall != 0 {
		t.Errorf("Scan must not be called when index is configured (got %d calls)", fake.scanCall)
	}
	if fake.queryCall != 1 {
		t.Errorf("expected exactly 1 Query call, got %d", fake.queryCall)
	}
	in := fake.queryIns[0]
	if in.IndexName == nil || *in.IndexName != "contentId-index" {
		t.Errorf("expected IndexName=contentId-index, got %v", in.IndexName)
	}
	if in.KeyConditionExpression == nil || *in.KeyConditionExpression != "contentId = :contentId" {
		t.Errorf("unexpected KeyConditionExpression: %v", in.KeyConditionExpression)
	}
}

func TestEncoderRepository_GetByContentId_QueryPaginates(t *testing.T) {
	t.Parallel()

	page1 := &dynamodb.QueryOutput{
		Items:            []map[string]types.AttributeValue{jobItem("j1", "c1")},
		LastEvaluatedKey: map[string]types.AttributeValue{"id": &types.AttributeValueMemberS{Value: "j1"}},
	}
	page2 := &dynamodb.QueryOutput{
		Items: []map[string]types.AttributeValue{jobItem("j2", "c1"), jobItem("j3", "c1")},
	}
	fake := &fakeDynamo{queryOuts: []*dynamodb.QueryOutput{page1, page2}}
	repo := &DynamoEncoderRepository{
		BaseRepository: NewBaseRepository(fake, "encoder"),
		contentIDIndex: "contentId-index",
	}

	jobs, err := repo.GetByContentId(context.Background(), "c1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(jobs) != 3 {
		t.Fatalf("expected 3 jobs across 2 pages, got %d", len(jobs))
	}
	if fake.queryCall != 2 {
		t.Errorf("expected 2 Query calls (one per page), got %d", fake.queryCall)
	}
	if fake.queryIns[1].ExclusiveStartKey == nil {
		t.Errorf("second Query call must carry ExclusiveStartKey from page 1")
	}
}

func TestEncoderRepository_GetByContentId_QueryError(t *testing.T) {
	t.Parallel()

	wantErr := errors.New("dynamo down")
	fake := &fakeDynamo{queryErrs: []error{wantErr}}
	repo := &DynamoEncoderRepository{
		BaseRepository: NewBaseRepository(fake, "encoder"),
		contentIDIndex: "contentId-index",
	}

	_, err := repo.GetByContentId(context.Background(), "c1")
	if err == nil || !errors.Is(err, wantErr) {
		t.Fatalf("expected wrapped %v, got %v", wantErr, err)
	}
}

func TestEncoderRepository_GetByContentId_EmptyResultReturnsEmptySlice(t *testing.T) {
	t.Parallel()

	fake := &fakeDynamo{queryOuts: []*dynamodb.QueryOutput{{Items: nil}}}
	repo := &DynamoEncoderRepository{
		BaseRepository: NewBaseRepository(fake, "encoder"),
		contentIDIndex: "contentId-index",
	}

	jobs, err := repo.GetByContentId(context.Background(), "c1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if jobs == nil {
		t.Fatalf("expected non-nil empty slice, got nil")
	}
	if len(jobs) != 0 {
		t.Fatalf("expected empty slice, got %d items", len(jobs))
	}
}

func TestEncoderRepository_GetByContentId_ScanFallback(t *testing.T) {
	t.Parallel()

	fake := &fakeDynamo{
		scanOuts: []*dynamodb.ScanOutput{
			{Items: []map[string]types.AttributeValue{jobItem("j1", "c1")}},
		},
	}

	repo := &DynamoEncoderRepository{
		BaseRepository: NewBaseRepository(fake, "encoder"),
		contentIDIndex: "",
	}

	jobs, err := repo.GetByContentId(context.Background(), "c1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(jobs) != 1 {
		t.Fatalf("expected 1 job, got %d", len(jobs))
	}
	if fake.queryCall != 0 {
		t.Errorf("Query must not be called in fallback mode (got %d)", fake.queryCall)
	}
	if fake.scanCall != 1 {
		t.Errorf("expected exactly 1 Scan call, got %d", fake.scanCall)
	}
	in := fake.scanIns[0]
	if in.FilterExpression == nil || *in.FilterExpression != "contentId = :contentId" {
		t.Errorf("unexpected FilterExpression: %v", in.FilterExpression)
	}
}

func TestEncoderRepository_Get_ReturnsNilWhenItemMissing(t *testing.T) {
	t.Parallel()

	fake := &fakeDynamo{getOut: &dynamodb.GetItemOutput{Item: nil}}
	repo := &DynamoEncoderRepository{
		BaseRepository: NewBaseRepository(fake, "encoder"),
	}

	job, err := repo.Get(context.Background(), "missing")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if job != nil {
		t.Fatalf("expected nil job for missing key, got %+v", job)
	}
}

func TestEncoderRepository_Create_PropagatesPutError(t *testing.T) {
	t.Parallel()

	wantErr := errors.New("conditional check failed")
	fake := &fakeDynamo{putErr: wantErr}
	repo := &DynamoEncoderRepository{
		BaseRepository: NewBaseRepository(fake, "encoder"),
	}

	err := repo.Create(context.Background(), encoderJobFixture("j1", "c1"))
	if err == nil || !errors.Is(err, wantErr) {
		t.Fatalf("expected wrapped %v, got %v", wantErr, err)
	}
}

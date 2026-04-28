package service

import (
	"context"
	"io"
	"sync"
	"testing"
	"time"

	"github.com/xanderbilla/bi8s-go/internal/model"
)

func TestEncoderService_ConcurrencySafety(t *testing.T) {
	repo := newMockEncoderRepository()
	uploader := newMockFileUploader()
	service := NewEncoderService(repo, uploader)

	numJobs := 5
	var wg sync.WaitGroup
	errors := make(chan error, numJobs)

	for i := 0; i < numJobs; i++ {
		wg.Add(1)
		go func(index int) {
			defer wg.Done()

			ctx := context.Background()
			videoInput := &model.FileUploadInput{
				FileName:    "test.mp4",
				ContentType: "video/mp4",
				Data:        []byte("fake video data"),
			}

			_, err := service.CreateEncodingJob(
				ctx,
				"content123",
				model.ContentTypeMovie,
				videoInput,
			)
			if err != nil {
				errors <- err
			}
		}(i)
	}

	wg.Wait()
	close(errors)

	for err := range errors {
		t.Errorf("Job creation failed: %v", err)
	}

	service.Wait(context.Background())

	if len(repo.jobs) != numJobs {
		t.Errorf("Expected %d jobs, got %d", numJobs, len(repo.jobs))
	}
}

func TestEncoderService_ContextTimeout(t *testing.T) {
	repo := newMockEncoderRepository()
	uploader := newMockFileUploader()
	service := NewEncoderService(repo, uploader)

	ctx := context.Background()
	videoInput := &model.FileUploadInput{
		FileName:    "test.mp4",
		ContentType: "video/mp4",
		Data:        []byte("fake video data"),
	}

	job, err := service.CreateEncodingJob(
		ctx,
		"content123",
		model.ContentTypeMovie,
		videoInput,
	)
	if err != nil {
		t.Fatalf("Failed to create job: %v", err)
	}

	time.Sleep(100 * time.Millisecond)

	if job.JobID == "" {
		t.Error("Job ID should not be empty")
	}

	service.Wait(context.Background())

	updatedJob, err := repo.Get(context.Background(), job.JobID)
	if err != nil {
		t.Fatalf("Failed to get job: %v", err)
	}

	if updatedJob == nil {
		t.Fatal("Job should exist in repository")
	}

	if updatedJob.Status == model.EncoderStatusProcessing {
		t.Error("Job should not be stuck in PROCESSING state")
	}
}

func TestEncoderService_PanicRecovery(t *testing.T) {
	repo := newMockEncoderRepository()
	uploader := &panicMockFileUploader{}
	service := NewEncoderService(repo, uploader)

	ctx := context.Background()
	videoInput := &model.FileUploadInput{
		FileName:    "test.mp4",
		ContentType: "video/mp4",
		Data:        []byte("fake video data"),
	}

	_, err := service.CreateEncodingJob(
		ctx,
		"content123",
		model.ContentTypeMovie,
		videoInput,
	)

	if err == nil {
		t.Fatalf("Job creation should fail when uploader panics")
	}

	service.Wait(context.Background())

	service.fileUploader = newMockFileUploader()
	_, err = service.CreateEncodingJob(
		ctx,
		"content456",
		model.ContentTypeMovie,
		videoInput,
	)

	if err != nil {
		t.Errorf("Service should still work after panic recovery: %v", err)
	}
}

func TestEncoderService_GracefulShutdown(t *testing.T) {
	repo := newMockEncoderRepository()
	uploader := newMockFileUploader()
	service := NewEncoderService(repo, uploader)

	numJobs := 3
	for i := 0; i < numJobs; i++ {
		ctx := context.Background()
		videoInput := &model.FileUploadInput{
			FileName:    "test.mp4",
			ContentType: "video/mp4",
			Data:        []byte("fake video data"),
		}

		_, err := service.CreateEncodingJob(
			ctx,
			"content123",
			model.ContentTypeMovie,
			videoInput,
		)
		if err != nil {
			t.Fatalf("Failed to create job: %v", err)
		}
	}

	done := make(chan bool)
	go func() {
		service.Wait(context.Background())
		done <- true
	}()

	select {
	case <-done:

	case <-time.After(5 * time.Second):
		t.Error("Wait() timed out - jobs did not complete")
	}
}

type panicMockFileUploader struct{}

func (m *panicMockFileUploader) UploadFile(ctx context.Context, prefix, resourceID, purpose, fileName, contentType string, data []byte) (string, error) {
	panic("simulated panic in file upload")
}

func (m *panicMockFileUploader) UploadFileStream(ctx context.Context, prefix, resourceID, purpose, fileName, contentType string, body io.Reader, size int64) (string, error) {
	panic("simulated panic in file upload")
}

func (m *panicMockFileUploader) Delete(ctx context.Context, key string) error {
	return nil
}

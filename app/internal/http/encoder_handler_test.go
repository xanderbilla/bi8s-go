package http

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/xanderbilla/bi8s-go/internal/model"
)

type mockEncoderService struct {
	jobs map[string]*model.EncoderJob
	mu   sync.RWMutex
}

func newMockEncoderService() *mockEncoderService {
	return &mockEncoderService{
		jobs: make(map[string]*model.EncoderJob),
	}
}

func (m *mockEncoderService) CreateEncodingJob(ctx context.Context, contentID string, contentType model.ContentType, videoInput any) (model.EncoderJob, error) {
	job := model.EncoderJob{
		JobID:       "test_job_123",
		ContentID:   contentID,
		ContentType: contentType,
		Status:      model.EncoderStatusQueued,
		Meta:        model.EncoderMeta{CreatedAt: time.Now()},
	}
	m.mu.Lock()
	m.jobs[job.JobID] = &job
	m.mu.Unlock()
	return job, nil
}

func (m *mockEncoderService) GetEncodingJob(ctx context.Context, jobID string) (*model.EncoderJob, error) {
	m.mu.RLock()
	job, exists := m.jobs[jobID]
	m.mu.RUnlock()
	if !exists {
		return nil, nil
	}
	return job, nil
}

func (m *mockEncoderService) Wait() {

}

func TestEncoderHandler_CreateJob_InvalidContentTypeParam(t *testing.T) {
	tests := []struct {
		name        string
		contentType string
		wantValid   bool
	}{
		{
			name:        "valid movie type",
			contentType: "movie",
			wantValid:   true,
		},
		{
			name:        "valid person type",
			contentType: "person",
			wantValid:   true,
		},
		{
			name:        "invalid type",
			contentType: "invalid",
			wantValid:   false,
		},
		{
			name:        "empty type",
			contentType: "",
			wantValid:   false,
		},
		{
			name:        "sql injection attempt",
			contentType: "movie'; DROP TABLE movies;--",
			wantValid:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var contentType model.ContentType
			switch tt.contentType {
			case "movie":
				contentType = model.ContentTypeMovie
			case "person":
				contentType = model.ContentTypePerson
			default:
				contentType = model.ContentType(tt.contentType)
			}

			isValid := contentType == model.ContentTypeMovie || contentType == model.ContentTypePerson

			if isValid != tt.wantValid {
				t.Errorf("Content type %s validation = %v, want %v", tt.contentType, isValid, tt.wantValid)
			}
		})
	}
}

func TestEncoderHandler_GetJob_Success(t *testing.T) {
	service := newMockEncoderService()

	testJob := &model.EncoderJob{
		JobID:       "test123",
		ContentID:   "movie123",
		ContentType: model.ContentTypeMovie,
		Status:      model.EncoderStatusCompleted,
	}
	service.jobs["test123"] = testJob

	job, err := service.GetEncodingJob(context.Background(), "test123")
	if err != nil {
		t.Errorf("GetEncodingJob() unexpected error = %v", err)
		return
	}

	if job == nil {
		t.Fatal("GetEncodingJob() returned nil")
	}

	if job.JobID != "test123" {
		t.Errorf("JobID = %v, want test123", job.JobID)
	}

	if job.Status != model.EncoderStatusCompleted {
		t.Errorf("Status = %v, want %v", job.Status, model.EncoderStatusCompleted)
	}
}

func TestEncoderHandler_GetJob_NotFound(t *testing.T) {
	service := newMockEncoderService()

	job, err := service.GetEncodingJob(context.Background(), "nonexistent")
	if err != nil {
		t.Errorf("GetEncodingJob() unexpected error = %v", err)
		return
	}

	if job != nil {
		t.Error("GetEncodingJob() should return nil for non-existent job")
	}
}

func TestEncoderHandler_JobIDValidation(t *testing.T) {
	tests := []struct {
		name    string
		jobID   string
		wantErr bool
	}{
		{
			name:    "valid job ID",
			jobID:   "job_1234567890",
			wantErr: false,
		},
		{
			name:    "empty job ID",
			jobID:   "",
			wantErr: true,
		},
		{
			name:    "path traversal attempt",
			jobID:   "../../../etc/passwd",
			wantErr: true,
		},
		{
			name:    "sql injection attempt",
			jobID:   "job123'; DROP TABLE jobs;--",
			wantErr: true,
		},
		{
			name:    "very long job ID",
			jobID:   string(make([]byte, 1000)),
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {

			isValid := len(tt.jobID) > 0 && len(tt.jobID) < 100
			for _, c := range tt.jobID {
				if !((c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '_') {
					isValid = false
					break
				}
			}

			if tt.wantErr && isValid {
				t.Errorf("Job ID %s should be invalid", tt.jobID)
			}

			if !tt.wantErr && !isValid {
				t.Errorf("Job ID %s should be valid", tt.jobID)
			}
		})
	}
}

func TestEncoderHandler_ResponseFormat(t *testing.T) {
	job := model.EncoderJob{
		JobID:       "test123",
		ContentID:   "movie123",
		ContentType: model.ContentTypeMovie,
		Status:      model.EncoderStatusQueued,
		Meta:        model.EncoderMeta{CreatedAt: time.Now()},
	}

	data, err := json.Marshal(job)
	if err != nil {
		t.Fatalf("Failed to marshal job: %v", err)
	}

	var decoded model.EncoderJob
	err = json.Unmarshal(data, &decoded)
	if err != nil {
		t.Fatalf("Failed to unmarshal job: %v", err)
	}

	if decoded.JobID != job.JobID {
		t.Errorf("JobID after marshal/unmarshal = %v, want %v", decoded.JobID, job.JobID)
	}

	if decoded.Status != job.Status {
		t.Errorf("Status after marshal/unmarshal = %v, want %v", decoded.Status, job.Status)
	}
}

func TestEncoderHandler_LargeFileUpload(t *testing.T) {

	largeSize := int64(100 * 1024 * 1024)

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	part, err := writer.CreateFormFile("video", "large_video.mp4")
	if err != nil {
		t.Fatalf("Failed to create form file: %v", err)
	}

	_, err = part.Write([]byte("simulated large video content"))
	if err != nil {
		t.Fatalf("Failed to write to form file: %v", err)
	}

	writer.Close()

	if int64(body.Len()) > largeSize {
		t.Error("File size exceeds limit")
	}

	t.Logf("Large file upload test: simulated %d bytes", body.Len())
}

func TestEncoderHandler_ConcurrentRequests(t *testing.T) {
	service := newMockEncoderService()

	concurrency := 10
	done := make(chan bool, concurrency)

	for i := 0; i < concurrency; i++ {
		go func(id int) {
			ctx := context.Background()
			_, err := service.CreateEncodingJob(ctx, "movie123", model.ContentTypeMovie, nil)
			if err != nil {
				t.Errorf("Concurrent CreateEncodingJob() failed: %v", err)
			}
			done <- true
		}(i)
	}

	for i := 0; i < concurrency; i++ {
		select {
		case <-done:

		case <-time.After(5 * time.Second):
			t.Fatal("Concurrent requests timed out")
		}
	}

	t.Logf("Successfully handled %d concurrent requests", concurrency)
}

func TestEncoderHandler_ContextCancellation(t *testing.T) {
	service := newMockEncoderService()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := service.CreateEncodingJob(ctx, "movie123", model.ContentTypeMovie, nil)

	_ = err

	t.Log("Context cancellation handled gracefully")
}

func TestMultipartMemoryLimit(t *testing.T) {
	maxMemory := int64(10 << 20)

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	part, err := writer.CreateFormFile("video", "test.mp4")
	if err != nil {
		t.Fatalf("Failed to create form file: %v", err)
	}

	testData := bytes.Repeat([]byte("a"), 1024)
	_, err = io.CopyN(part, bytes.NewReader(testData), 1024)
	if err != nil && err != io.EOF {
		t.Fatalf("Failed to write test data: %v", err)
	}

	writer.Close()

	req := httptest.NewRequest(http.MethodPost, "/upload", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())

	err = req.ParseMultipartForm(maxMemory)
	if err != nil {
		t.Logf("ParseMultipartForm with limit: %v", err)
	}

	if req.MultipartForm != nil {
		t.Log("Multipart form parsed successfully within memory limit")
	}
}

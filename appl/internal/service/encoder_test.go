package service

import (
	"context"
	"errors"
	"io"
	"sync"
	"testing"
	"time"

	"github.com/xanderbilla/bi8s-go/internal/errs"
	"github.com/xanderbilla/bi8s-go/internal/model"
)

type mockEncoderRepository struct {
	mu   sync.RWMutex
	jobs map[string]model.EncoderJob
}

func newMockEncoderRepository() *mockEncoderRepository {
	return &mockEncoderRepository{
		jobs: make(map[string]model.EncoderJob),
	}
}

func (m *mockEncoderRepository) Create(ctx context.Context, job model.EncoderJob) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.jobs[job.JobID] = job
	return nil
}

func (m *mockEncoderRepository) Get(ctx context.Context, jobID string) (*model.EncoderJob, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	job, exists := m.jobs[jobID]
	if !exists {
		return nil, nil
	}
	return &job, nil
}

func (m *mockEncoderRepository) Update(ctx context.Context, job *model.EncoderJob) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.jobs[job.JobID] = *job
	return nil
}

func (m *mockEncoderRepository) GetByContentId(ctx context.Context, contentID string) ([]model.EncoderJob, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	var jobs []model.EncoderJob
	for _, job := range m.jobs {
		if job.ContentID == contentID {
			jobs = append(jobs, job)
		}
	}
	return jobs, nil
}

type mockFileUploader struct {
	mu            sync.RWMutex
	uploadedFiles map[string][]byte
}

func newMockFileUploader() *mockFileUploader {
	return &mockFileUploader{
		uploadedFiles: make(map[string][]byte),
	}
}

func (m *mockFileUploader) UploadFile(ctx context.Context, contentType, contentID, path, filename, mimeType string, data []byte) (string, error) {
	key := contentType + "/" + contentID + "/" + path
	m.mu.Lock()
	defer m.mu.Unlock()
	m.uploadedFiles[key] = data
	return key, nil
}

func (m *mockFileUploader) UploadFileStream(ctx context.Context, contentType, contentID, path, filename, mimeType string, body io.Reader, size int64) (string, error) {
	data, err := io.ReadAll(body)
	if err != nil {
		return "", err
	}
	return m.UploadFile(ctx, contentType, contentID, path, filename, mimeType, data)
}

func (m *mockFileUploader) GetFileURL(ctx context.Context, key string) (string, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return "https://example.com/" + key, nil
}

func (m *mockFileUploader) DeleteFile(ctx context.Context, key string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.uploadedFiles, key)
	return nil
}

func (m *mockFileUploader) Delete(ctx context.Context, key string) error {
	return m.DeleteFile(ctx, key)
}

func TestNewEncoderService(t *testing.T) {
	t.Setenv("ENCODER_MAX_CONCURRENT", "3")
	repo := newMockEncoderRepository()
	uploader := newMockFileUploader()

	service := NewEncoderService(repo, uploader)

	if service == nil {
		t.Fatal("NewEncoderService() returned nil")
	}

	if service.repo == nil {
		t.Error("EncoderService.repo is nil")
	}

	if service.fileUploader == nil {
		t.Error("EncoderService.fileUploader is nil")
	}

	if service.sem == nil {
		t.Error("EncoderService.sem (semaphore) is nil")
	}

	if cap(service.sem) != defaultMaxConcurrentJobs {
		t.Errorf("EncoderService.sem capacity = %d, want %d", cap(service.sem), defaultMaxConcurrentJobs)
	}
}

func TestCreateEncodingJob_Validation(t *testing.T) {
	repo := newMockEncoderRepository()
	uploader := newMockFileUploader()
	service := NewEncoderService(repo, uploader)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	tests := []struct {
		name        string
		contentID   string
		contentType model.ContentType
		videoInput  *model.FileUploadInput
		wantErr     bool
	}{
		{
			name:        "valid movie upload",
			contentID:   "movie123",
			contentType: model.ContentTypeMovie,
			videoInput: &model.FileUploadInput{
				FileName:    "test.mp4",
				ContentType: "video/mp4",
				Data:        []byte("fake video data"),
			},
			wantErr: false,
		},
		{
			name:        "valid person upload",
			contentID:   "person456",
			contentType: model.ContentTypePerson,
			videoInput: &model.FileUploadInput{
				FileName:    "profile.mp4",
				ContentType: "video/mp4",
				Data:        []byte("fake video data"),
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			job, err := service.CreateEncodingJob(ctx, tt.contentID, tt.contentType, tt.videoInput)

			if tt.wantErr {
				if err == nil {
					t.Error("CreateEncodingJob() expected error but got none")
				}
				return
			}

			if err != nil {
				t.Errorf("CreateEncodingJob() unexpected error = %v", err)
				return
			}

			if job.JobID == "" {
				t.Error("CreateEncodingJob() returned job with empty JobID")
			}

			if job.ContentID != tt.contentID {
				t.Errorf("CreateEncodingJob() ContentID = %v, want %v", job.ContentID, tt.contentID)
			}

			if job.ContentType != tt.contentType {
				t.Errorf("CreateEncodingJob() ContentType = %v, want %v", job.ContentType, tt.contentType)
			}

			if job.Status != model.EncoderStatusQueued {
				t.Errorf("CreateEncodingJob() Status = %v, want %v", job.Status, model.EncoderStatusQueued)
			}

			if job.Input.FileName != tt.videoInput.FileName {
				t.Errorf("CreateEncodingJob() Input.FileName = %v, want %v", job.Input.FileName, tt.videoInput.FileName)
			}

			if len(uploader.uploadedFiles) == 0 {
				t.Error("CreateEncodingJob() did not upload input file")
			}
		})
	}
}

func TestGetEncodingJob(t *testing.T) {
	repo := newMockEncoderRepository()
	uploader := newMockFileUploader()
	service := NewEncoderService(repo, uploader)

	ctx := context.Background()

	testJob := model.EncoderJob{
		JobID:       "test123",
		ContentID:   "movie123",
		ContentType: model.ContentTypeMovie,
		Status:      model.EncoderStatusCompleted,
	}
	if err := repo.Create(ctx, testJob); err != nil {
		t.Fatalf("repo.Create() unexpected error = %v", err)
	}

	job, err := service.GetEncodingJob(ctx, "test123")
	if err != nil {
		t.Errorf("GetEncodingJob() unexpected error = %v", err)
		return
	}

	if job == nil {
		t.Fatal("GetEncodingJob() returned nil job")
	}

	if job.JobID != "test123" {
		t.Errorf("GetEncodingJob() JobID = %v, want test123", job.JobID)
	}

	job, err = service.GetEncodingJob(ctx, "nonexistent")
	if !errors.Is(err, errs.ErrContentNotFound) {
		t.Errorf("GetEncodingJob() expected ErrContentNotFound for non-existent job, got = %v", err)
	}

	if job != nil {
		t.Error("GetEncodingJob() should return nil for non-existent job")
	}
}

func TestGetQualityConfig(t *testing.T) {
	tests := []struct {
		name        string
		quality     string
		wantQuality string
	}{
		{name: "720p config", quality: "720p", wantQuality: "720p"},
		{name: "1080p config", quality: "1080p", wantQuality: "1080p"},
		{name: "360p config", quality: "360p", wantQuality: "360p"},
		{name: "unknown quality returns zero value", quality: "42p", wantQuality: ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := getQualityConfig(tt.quality)

			if config.Quality != tt.wantQuality {
				t.Errorf("getQualityConfig() Quality = %v, want %v", config.Quality, tt.wantQuality)
			}

			if tt.wantQuality == "" {
				return
			}

			if config.Resolution == "" {
				t.Error("getQualityConfig() Resolution is empty")
			}

			if config.VideoBitrate == "" {
				t.Error("getQualityConfig() VideoBitrate is empty")
			}
		})
	}
}

func TestContentTypeToPath(t *testing.T) {
	tests := []struct {
		name        string
		contentType model.ContentType
		want        string
	}{
		{"movie", model.ContentTypeMovie, "movies"},
		{"person", model.ContentTypePerson, "persons"},
		{"tv", model.ContentTypeTV, "tv"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.contentType.ToPath(); got != tt.want {
				t.Errorf("ToPath() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestEncoderService_Wait(t *testing.T) {
	repo := newMockEncoderRepository()
	uploader := newMockFileUploader()
	service := NewEncoderService(repo, uploader)

	service.wg.Add(1)
	go func() {
		defer service.wg.Done()
		time.Sleep(100 * time.Millisecond)
	}()

	done := make(chan struct{})
	go func() {
		if err := service.Wait(context.Background()); err != nil {
			t.Logf("Wait() returned: %v", err)
		}
		close(done)
	}()

	select {
	case <-done:

	case <-time.After(1 * time.Second):
		t.Error("Wait() did not return within timeout")
	}
}

func TestEncoderService_Semaphore(t *testing.T) {
	t.Setenv("ENCODER_MAX_CONCURRENT", "3")
	repo := newMockEncoderRepository()
	uploader := newMockFileUploader()
	service := NewEncoderService(repo, uploader)

	if cap(service.sem) != defaultMaxConcurrentJobs {
		t.Errorf("Semaphore capacity = %d, want %d", cap(service.sem), defaultMaxConcurrentJobs)
	}

	for i := 0; i < defaultMaxConcurrentJobs; i++ {
		select {
		case service.sem <- struct{}{}:

		case <-time.After(100 * time.Millisecond):
			t.Errorf("Failed to acquire semaphore slot %d", i)
		}
	}

	select {
	case service.sem <- struct{}{}:
		t.Error("Semaphore should be full but accepted another slot")
	case <-time.After(100 * time.Millisecond):

	}

	<-service.sem

	select {
	case service.sem <- struct{}{}:

	case <-time.After(100 * time.Millisecond):
		t.Error("Failed to acquire semaphore slot after release")
	}

	for i := 0; i < defaultMaxConcurrentJobs; i++ {
		select {
		case <-service.sem:
		default:
		}
	}
}

func TestBuildPlaybackResponse(t *testing.T) {
	repo := newMockEncoderRepository()
	uploader := newMockFileUploader()
	service := NewEncoderService(repo, uploader)

	masterPlaylist := "/movies/movie123/output/master.m3u8"
	previewFile := "/movies/movie123/preview/30sec.mp4"
	previewDuration := 30.0
	spriteImage := "/movies/movie123/preview/sprite.jpg"
	spriteVTT := "/movies/movie123/preview/sprite.vtt"

	job := &model.EncoderJob{
		JobID:       "job123",
		ContentID:   "movie123",
		ContentType: model.ContentTypeMovie,
		Status:      model.EncoderStatusCompleted,
		Input: model.EncoderInput{
			DurationSec: 120.5,
		},
		Output: model.EncoderOutput{
			MasterPlaylists: model.MasterPlaylists{
				CurrentMasterPlaylist: &masterPlaylist,
			},
			Video: model.VideoOutput{
				Qualities: []model.VideoQuality{
					{Quality: "720p", Resolution: "1280x720"},
					{Quality: "1080p", Resolution: "1920x1080"},
				},
			},
			Audio: model.AudioOutput{
				Tracks: []model.AudioTrack{
					{Bitrate: "256k", Label: "Default", Language: "und", Default: true},
				},
			},
			Preview: model.PreviewOutput{
				File:        &previewFile,
				DurationSec: &previewDuration,
			},
			Thumbnails: model.ThumbnailsOutput{
				Items: []string{"/thumb1.jpg", "/thumb2.jpg"},
			},
			Sprite: model.SpriteOutput{
				Image: &spriteImage,
				VTT:   &spriteVTT,
			},
		},
	}

	response := service.buildPlaybackResponse(job)

	if response == nil {
		t.Fatal("buildPlaybackResponse() returned nil")
	}

	if response.DurationSec != 120.5 {
		t.Errorf("DurationSec = %v, want 120.5", response.DurationSec)
	}

	if response.Streaming.MasterPlaylist != masterPlaylist {
		t.Errorf("MasterPlaylist = %v, want %v", response.Streaming.MasterPlaylist, masterPlaylist)
	}

	if len(response.Video.Qualities) != 2 {
		t.Errorf("Video.Qualities length = %d, want 2", len(response.Video.Qualities))
	}

	if len(response.Audio.Tracks) != 1 {
		t.Errorf("Audio.Tracks length = %d, want 1", len(response.Audio.Tracks))
	}

	if response.Preview.Video != previewFile {
		t.Errorf("Preview.Video = %v, want %v", response.Preview.Video, previewFile)
	}

	if len(response.Thumbnails.Items) != 2 {
		t.Errorf("Thumbnails.Items length = %d, want 2", len(response.Thumbnails.Items))
	}

	if response.Sprite.Image != spriteImage {
		t.Errorf("Sprite.Image = %v, want %v", response.Sprite.Image, spriteImage)
	}
}

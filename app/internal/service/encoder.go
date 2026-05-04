package service

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/xanderbilla/bi8s-go/internal/env"
	"github.com/xanderbilla/bi8s-go/internal/errs"
	"github.com/xanderbilla/bi8s-go/internal/logger"
	"github.com/xanderbilla/bi8s-go/internal/model"
	"github.com/xanderbilla/bi8s-go/internal/repository"
	"github.com/xanderbilla/bi8s-go/internal/storage"
	"github.com/xanderbilla/bi8s-go/internal/utils"
)

const defaultMaxConcurrentJobs = 3

type EncoderService struct {
	repo         repository.EncoderRepository
	fileUploader storage.FileUploader

	wg           sync.WaitGroup
	sem          chan struct{}
	ffmpegPerJob int
	shutdownCh   chan struct{}
	shutdownMu   sync.Mutex
	shuttingDown bool
}

func NewEncoderService(repo repository.EncoderRepository, fileUploader storage.FileUploader) *EncoderService {
	maxConcurrent := env.GetInt("ENCODER_MAX_CONCURRENT", defaultMaxConcurrentJobs)
	ffmpegPerJob := env.GetInt("ENCODER_FFMPEG_PARALLELISM", runtime.GOMAXPROCS(0))
	if ffmpegPerJob < 1 {
		ffmpegPerJob = 1
	}
	return &EncoderService{
		repo:         repo,
		fileUploader: fileUploader,
		sem:          make(chan struct{}, maxConcurrent),
		ffmpegPerJob: ffmpegPerJob,
		shutdownCh:   make(chan struct{}),
	}
}

func (s *EncoderService) Shutdown() {
	s.shutdownMu.Lock()
	defer s.shutdownMu.Unlock()
	if s.shuttingDown {
		return
	}
	s.shuttingDown = true
	close(s.shutdownCh)
}

func (s *EncoderService) Wait(ctx context.Context) error {
	done := make(chan struct{})
	go func() {
		s.wg.Wait()
		close(done)
	}()
	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (s *EncoderService) CreateEncodingJob(
	ctx context.Context,
	contentID string,
	contentType model.ContentType,
	videoInput *model.FileUploadInput,
) (model.EncoderJob, error) {
	if videoInput == nil {
		return model.EncoderJob{}, fmt.Errorf("video input is required")
	}

	jobID := fmt.Sprintf("job_%s", utils.GenerateID())
	contentTypePath := contentType.ToPath()

	logger.InfoContext(ctx, "creating encoding job",
		"job_id", jobID,
		"content_id", contentID,
		"content_type", contentType,
		"file_name", videoInput.FileName,
		"file_size", len(videoInput.Data),
	)

	tempFile, err := s.ensureTempFile(ctx, jobID, videoInput)
	if err != nil {
		return model.EncoderJob{}, err
	}

	inputS3Key := fmt.Sprintf("%s/%s/input/%s%s", contentTypePath, contentID, jobID, filepath.Ext(videoInput.FileName))

	fileHandle, err := os.Open(tempFile)
	if err != nil {
		if rmErr := os.Remove(tempFile); rmErr != nil && !os.IsNotExist(rmErr) {
			logger.WarnContext(ctx, "failed to remove temp file", "job_id", jobID, "temp_file", tempFile, "error", rmErr.Error())
		}
		logger.ErrorContext(ctx, "failed to open temporary file for upload",
			"job_id", jobID,
			"temp_file", tempFile,
			"error", err.Error(),
		)
		return model.EncoderJob{}, fmt.Errorf("open temporary file: %w", err)
	}
	defer func() {
		if err := fileHandle.Close(); err != nil {
			logger.WarnContext(ctx, "failed to close file handle", "temp_file", tempFile, "error", err.Error())
		}
	}()

	func() {
		defer func() {
			if r := recover(); r != nil {
				err = fmt.Errorf("upload input video panic: %v", r)
			}
		}()
		_, err = s.fileUploader.UploadFileStream(
			ctx,
			"",
			"",
			inputS3Key,
			videoInput.FileName,
			videoInput.ContentType,
			fileHandle,
			videoInput.Size,
		)
	}()
	if err != nil {
		if rmErr := os.Remove(tempFile); rmErr != nil && !os.IsNotExist(rmErr) {
			logger.WarnContext(ctx, "failed to remove temp file", "job_id", jobID, "temp_file", tempFile, "error", rmErr.Error())
		}
		logger.ErrorContext(ctx, "failed to upload input video",
			"job_id", jobID,
			"content_id", contentID,
			"error", err.Error(),
		)
		return model.EncoderJob{}, fmt.Errorf("upload input video: %w", err)
	}

	job := model.EncoderJob{
		JobID:       jobID,
		ContentType: contentType,
		ContentID:   contentID,
		Status:      model.EncoderStatusQueued,
		Input: model.EncoderInput{
			FileName:        videoInput.FileName,
			SourcePath:      "/" + inputS3Key,
			SourceExtension: strings.TrimPrefix(filepath.Ext(videoInput.FileName), "."),
		},
		Output: model.EncoderOutput{
			BaseOutputDir: fmt.Sprintf("/%s/%s/output/%s", contentTypePath, contentID, jobID),
			Video:         model.VideoOutput{Qualities: []model.VideoQuality{}},
			Audio:         model.AudioOutput{Tracks: []model.AudioTrack{}},
			Subtitles:     model.SubtitlesOutput{Dir: fmt.Sprintf("/%s/%s/output/%s/video/subtitle/", contentTypePath, contentID, jobID), Tracks: []model.SubtitleOutput{}},
			Thumbnails:    model.ThumbnailsOutput{Dir: fmt.Sprintf("/%s/%s/thumbnails/", contentTypePath, contentID), Count: 0, Items: []string{}},
		},
		Errors:   []model.EncoderError{},
		Warnings: []model.EncoderWarning{},
		Meta:     model.EncoderMeta{CreatedAt: time.Now()},
	}

	if err := s.repo.Create(ctx, job); err != nil {
		if rmErr := os.Remove(tempFile); rmErr != nil && !os.IsNotExist(rmErr) {
			logger.WarnContext(ctx, "failed to remove temp file", "job_id", jobID, "temp_file", tempFile, "error", rmErr.Error())
		}
		logger.ErrorContext(ctx, "failed to create job record",
			"job_id", jobID,
			"content_id", contentID,
			"error", err.Error(),
		)
		return model.EncoderJob{}, fmt.Errorf("create job record: %w", err)
	}

	logger.InfoContext(ctx, "encoding job created and queued",
		"job_id", jobID,
		"content_id", contentID,
		"status", job.Status,
	)

	jobTimeout := time.Duration(env.GetInt("ENCODER_JOB_TIMEOUT_SECONDS", 1800)) * time.Second
	jobCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), jobTimeout)

	s.wg.Add(1)

	go func(jobPtr *model.EncoderJob, tempFilePath string, ctx context.Context, cancelFn context.CancelFunc) {
		defer s.wg.Done()
		defer cancelFn()
		defer func() {
			if rmErr := os.Remove(tempFilePath); rmErr != nil && !os.IsNotExist(rmErr) {
				logger.WarnContext(ctx, "failed to remove temp file", "job_id", jobPtr.JobID, "temp_file", tempFilePath, "error", rmErr.Error())
			}
		}()

		select {
		case s.sem <- struct{}{}:
		case <-s.shutdownCh:
			s.failJob(ctx, jobPtr,
				"SHUTDOWN", "service shutting down before slot acquired",
				"queue", "", nil,
			)
			return
		case <-ctx.Done():
			s.failJob(ctx, jobPtr,
				"TIMEOUT", "job timed out before slot acquired",
				"queue", "", ctx.Err(),
			)
			return
		}

		defer func() {
			<-s.sem
		}()

		defer func() {
			if r := recover(); r != nil {
				logger.ErrorContext(ctx, "encoder panic recovered",
					"job_id", jobPtr.JobID,
					"content_id", jobPtr.ContentID,
					"panic", fmt.Sprintf("%v", r),
				)
				s.failJob(ctx, jobPtr,
					"PANIC", fmt.Sprintf("unexpected panic: %v", r),
					"runtime", "", nil,
				)
			}
		}()

		s.processEncodingJobFromFile(ctx, jobPtr, tempFilePath)
	}(&job, tempFile, jobCtx, cancel)

	return job, nil
}

func (s *EncoderService) ensureTempFile(ctx context.Context, jobID string, videoInput *model.FileUploadInput) (string, error) {
	if videoInput.TempFilePath != "" {
		return videoInput.TempFilePath, nil
	}

	pattern := fmt.Sprintf("bi8s-job-%s-*%s", jobID, filepath.Ext(videoInput.FileName))
	tmp, err := os.CreateTemp(utils.TmpDir(), pattern)
	if err != nil {
		logger.ErrorContext(ctx, "failed to create temporary file",
			"job_id", jobID,
			"error", err.Error(),
		)
		return "", fmt.Errorf("create temporary file: %w", err)
	}
	tempFile := tmp.Name()

	if _, err := tmp.Write(videoInput.Data); err != nil {
		if closeErr := tmp.Close(); closeErr != nil {
			logger.WarnContext(ctx, "failed to close temp file after write error", "job_id", jobID, "temp_file", tempFile, "error", closeErr.Error())
		}
		if rmErr := os.Remove(tempFile); rmErr != nil && !os.IsNotExist(rmErr) {
			logger.WarnContext(ctx, "failed to remove temp file", "job_id", jobID, "temp_file", tempFile, "error", rmErr.Error())
		}
		logger.ErrorContext(ctx, "failed to write temporary file",
			"job_id", jobID,
			"temp_file", tempFile,
			"error", err.Error(),
		)
		return "", fmt.Errorf("write temporary file: %w", err)
	}
	if err := tmp.Close(); err != nil {
		if rmErr := os.Remove(tempFile); rmErr != nil && !os.IsNotExist(rmErr) {
			logger.WarnContext(ctx, "failed to remove temp file", "job_id", jobID, "temp_file", tempFile, "error", rmErr.Error())
		}
		return "", fmt.Errorf("close temporary file: %w", err)
	}
	if err := os.Chmod(tempFile, 0o600); err != nil {
		if rmErr := os.Remove(tempFile); rmErr != nil && !os.IsNotExist(rmErr) {
			logger.WarnContext(ctx, "failed to remove temp file", "job_id", jobID, "temp_file", tempFile, "error", rmErr.Error())
		}
		return "", fmt.Errorf("chmod temporary file: %w", err)
	}

	videoInput.Size = int64(len(videoInput.Data))
	videoInput.Data = nil
	return tempFile, nil
}

func (s *EncoderService) GetEncodingJob(ctx context.Context, jobID string) (*model.EncoderJob, error) {
	j, err := s.repo.Get(ctx, jobID)
	if err != nil {
		return nil, err
	}
	if j == nil {
		return nil, errs.ErrContentNotFound
	}
	return j, nil
}

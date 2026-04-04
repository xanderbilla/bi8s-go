package service

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/xanderbilla/bi8s-go/internal/domain"
	"github.com/xanderbilla/bi8s-go/internal/model"
	"github.com/xanderbilla/bi8s-go/internal/repository"
	"github.com/xanderbilla/bi8s-go/internal/storage"
	"github.com/xanderbilla/bi8s-go/internal/utils"
)

// EncoderService handles video encoding operations
type EncoderService struct {
	repo         repository.EncoderRepository
	fileUploader storage.FileUploader
}

// NewEncoderService creates a new encoder service
func NewEncoderService(repo repository.EncoderRepository, fileUploader storage.FileUploader) *EncoderService {
	return &EncoderService{
		repo:         repo,
		fileUploader: fileUploader,
	}
}

// CreateEncodingJob creates a new encoding job and starts processing
func (s *EncoderService) CreateEncodingJob(ctx context.Context, contentID string, contentType model.ContentType, videoInput *domain.FileUploadInput) (model.EncoderJob, error) {
	// Generate job ID using timestamp
	jobID := fmt.Sprintf("job_%d", time.Now().UnixMilli())
	
	// Determine content type path
	contentTypePath := getContentTypePath(contentType)
	
	// Upload input video to S3
	inputPath := fmt.Sprintf("%s/%s/input/%s%s", contentTypePath, contentID, jobID, filepath.Ext(videoInput.FileName))
	_, err := s.fileUploader.UploadFile(
		ctx,
		contentTypePath,
		contentID,
		fmt.Sprintf("input/%s", filepath.Base(inputPath)),
		videoInput.FileName,
		videoInput.ContentType,
		videoInput.Data,
	)
	if err != nil {
		return model.EncoderJob{}, fmt.Errorf("failed to upload input video: %w", err)
	}

	// Create initial job record
	job := model.EncoderJob{
		JobID:       jobID,
		ContentType: contentType,
		ContentID:   contentID,
		Status:      model.EncoderStatusQueued,
		Input: model.EncoderInput{
			FileName:        videoInput.FileName,
			SourcePath:      "/" + inputPath,
			SourceExtension: strings.TrimPrefix(filepath.Ext(videoInput.FileName), "."),
		},
		Output: model.EncoderOutput{
			BaseOutputDir: fmt.Sprintf("/%s/%s/output/%s", contentTypePath, contentID, jobID),
			Video:         model.VideoOutput{Qualities: []model.VideoQuality{}},
			Audio:         model.AudioOutput{Tracks: []model.AudioTrack{}},
			Subtitles:     model.SubtitlesOutput{Dir: fmt.Sprintf("/%s/%s/output/%s/video/subtitle/", contentTypePath, contentID, jobID), Tracks: []any{}},
			Thumbnails:    model.ThumbnailsOutput{Dir: fmt.Sprintf("/%s/%s/thumbnails/", contentTypePath, contentID), Count: 0, Items: []string{}},
		},
		Errors:   []model.EncoderError{},
		Warnings: []model.EncoderWarning{},
		Meta: model.EncoderMeta{
			CreatedAt: time.Now(),
		},
	}

	// Save initial job to DynamoDB
	if err := s.repo.Create(ctx, job); err != nil {
		return model.EncoderJob{}, fmt.Errorf("failed to create job record: %w", err)
	}

	// Start encoding in background with the video data
	go s.processEncodingJob(context.Background(), job, videoInput.Data)

	return job, nil
}

// GetEncodingJob retrieves an encoding job by ID
func (s *EncoderService) GetEncodingJob(ctx context.Context, jobID string) (*model.EncoderJob, error) {
	return s.repo.Get(ctx, jobID)
}

// processEncodingJob processes the encoding job in background
func (s *EncoderService) processEncodingJob(ctx context.Context, job model.EncoderJob, videoData []byte) {
	// Update status to processing
	job.Status = model.EncoderStatusProcessing
	startTime := time.Now()
	job.Meta.StartedAt = &startTime
	s.repo.Update(ctx, job)

	// Save uploaded video to temporary file for processing
	tempFile := fmt.Sprintf("/tmp/%s%s", job.JobID, filepath.Ext(job.Input.FileName))
	if err := os.WriteFile(tempFile, videoData, 0644); err != nil {
		s.failJob(ctx, &job, "TEMP_FILE_WRITE_FAILED", "Failed to write temporary file for processing", "initialization", tempFile, err)
		return
	}
	defer os.Remove(tempFile)
	
	// Extract real video metadata using FFprobe
	job = s.extractVideoMetadata(job, tempFile)
	if len(job.Errors) > 0 {
		s.failJob(ctx, &job, "", "", "", "", nil)
		return
	}
	
	// Update job with metadata
	s.repo.Update(ctx, job)
	
	// Create temporary working directory
	workDir := fmt.Sprintf("/tmp/%s_work", job.JobID)
	if err := os.MkdirAll(workDir, 0755); err != nil {
		s.failJob(ctx, &job, "WORK_DIR_CREATION_FAILED", "Failed to create working directory", "initialization", workDir, err)
		return
	}
	defer os.RemoveAll(workDir)
	
	// Process video with parallel operations
	job = s.processVideoParallel(ctx, job, tempFile, workDir)
	
	// Update final status
	s.repo.Update(ctx, job)
}

// extractVideoMetadata extracts real metadata from the video file using FFprobe
func (s *EncoderService) extractVideoMetadata(job model.EncoderJob, videoPath string) model.EncoderJob {
	metadata, err := utils.GetVideoMetadata(videoPath)
	if err != nil {
		// If FFprobe fails, add error and return
		job.Errors = append(job.Errors, model.EncoderError{
			Code:    "METADATA_EXTRACTION_FAILED",
			Message: "Failed to extract video metadata",
			Stage:   "metadata_extraction",
			Path:    videoPath,
			Details: err.Error(),
		})
		return job
	}

	// Update input with real metadata
	job.Input.Resolution = metadata.GetResolutionString()
	job.Input.DurationSec = metadata.Duration
	job.Input.VideoCodec = metadata.VideoCodec
	job.Input.AudioCodec = metadata.AudioCodec
	job.Input.HasEmbeddedAudio = metadata.HasAudio
	job.Input.HasExternalAudio = false
	job.Input.HasSubtitles = false

	// Determine appropriate quality levels based on source resolution
	qualities := metadata.DetermineQualities()
	job.Input.GeneratedQualities = []model.QualityConfig{}
	
	for _, quality := range qualities {
		config := getQualityConfig(quality, metadata.Width, metadata.Height)
		job.Input.GeneratedQualities = append(job.Input.GeneratedQualities, config)
	}

	return job
}

// getQualityConfig returns the configuration for a specific quality level
func getQualityConfig(quality string, sourceWidth, sourceHeight int) model.QualityConfig {
	if spec, ok := model.GetQualitySpec(quality); ok {
		return model.QualityConfig{
			Quality:      spec.Quality,
			Resolution:   spec.Resolution,
			VideoBitrate: spec.VideoBitrate,
		}
	}
	return model.QualityConfig{}
}

// generateMockOutput generates mock output structure (TODO: replace with real FFmpeg processing)
func (s *EncoderService) generateMockOutput(job model.EncoderJob) model.EncoderJob {
	contentTypePath := getContentTypePath(job.ContentType)
	
	// Calculate chunk count based on actual video duration (6 second chunks for HLS)
	chunkCount := int(job.Input.DurationSec / 6.0)
	if chunkCount == 0 {
		chunkCount = 1
	}

	// Generate video output based on extracted qualities
	baseOutputDir := fmt.Sprintf("/%s/%s/output/%s", contentTypePath, job.ContentID, job.JobID)
	job.Output.BaseOutputDir = baseOutputDir
	
	recordPlaylist := fmt.Sprintf("%s/%s_master.m3u8", baseOutputDir, job.JobID)
	currentPlaylist := fmt.Sprintf("/%s/%s/output/master.m3u8", contentTypePath, job.ContentID)
	job.Output.MasterPlaylists = model.MasterPlaylists{
		RecordMasterPlaylist:  &recordPlaylist,
		CurrentMasterPlaylist: &currentPlaylist,
	}

	// Generate video qualities based on extracted metadata
	job.Output.Video.Qualities = []model.VideoQuality{}
	for _, qualityConfig := range job.Input.GeneratedQualities {
		job.Output.Video.Qualities = append(job.Output.Video.Qualities, model.VideoQuality{
			Quality:    qualityConfig.Quality,
			Resolution: qualityConfig.Resolution,
			Playlist:   fmt.Sprintf("%s/video/%s/index.m3u8", baseOutputDir, strings.TrimSuffix(qualityConfig.Quality, "p")),
			ChunksDir:  fmt.Sprintf("%s/video/%s/", baseOutputDir, strings.TrimSuffix(qualityConfig.Quality, "p")),
			ChunkCount: chunkCount,
		})
	}

	// Generate audio output only if video has embedded audio
	if job.Input.HasEmbeddedAudio {
		job.Output.Audio.Tracks = []model.AudioTrack{
			{
				Bitrate:    "256k",
				Playlist:   fmt.Sprintf("%s/audio/256k/index.m3u8", baseOutputDir),
				ChunksDir:  fmt.Sprintf("%s/audio/256k/", baseOutputDir),
				ChunkCount: chunkCount,
				Language:   "und",
				Label:      "Default Audio",
				Default:    true,
			},
			{
				Bitrate:    "128k",
				Playlist:   fmt.Sprintf("%s/audio/128k/index.m3u8", baseOutputDir),
				ChunksDir:  fmt.Sprintf("%s/audio/128k/", baseOutputDir),
				ChunkCount: chunkCount,
				Language:   "und",
				Label:      "Default Audio",
				Default:    false,
			},
		}
	} else {
		job.Output.Audio.Tracks = []model.AudioTrack{}
	}

	// Generate thumbnails (5 thumbnails evenly distributed)
	thumbnailsDir := fmt.Sprintf("/%s/%s/thumbnails/", contentTypePath, job.ContentID)
	job.Output.Thumbnails = model.ThumbnailsOutput{
		Dir:   thumbnailsDir,
		Count: 5,
		Items: []string{
			fmt.Sprintf("%sthumbnail_1.jpg", thumbnailsDir),
			fmt.Sprintf("%sthumbnail_2.jpg", thumbnailsDir),
			fmt.Sprintf("%sthumbnail_3.jpg", thumbnailsDir),
			fmt.Sprintf("%sthumbnail_4.jpg", thumbnailsDir),
			fmt.Sprintf("%sthumbnail_5.jpg", thumbnailsDir),
		},
	}

	// Generate preview (30 seconds or full duration if video is shorter)
	previewFile := fmt.Sprintf("/%s/%s/preview/30sec.mp4", contentTypePath, job.ContentID)
	previewDuration := 30.0
	if job.Input.DurationSec < 30.0 {
		previewDuration = job.Input.DurationSec
	}
	job.Output.Preview = model.PreviewOutput{
		File:        &previewFile,
		DurationSec: &previewDuration,
	}

	// Generate sprite
	spriteImage := fmt.Sprintf("/%s/%s/preview/sprite.jpg", contentTypePath, job.ContentID)
	spriteVTT := fmt.Sprintf("/%s/%s/preview/sprite.vtt", contentTypePath, job.ContentID)
	job.Output.Sprite = model.SpriteOutput{
		Image: &spriteImage,
		VTT:   &spriteVTT,
	}

	// Update status and timing
	job.Status = model.EncoderStatusCompleted
	completedTime := time.Now()
	job.Meta.CompletedAt = &completedTime
	
	if job.Meta.StartedAt != nil {
		processingTime := completedTime.Sub(*job.Meta.StartedAt).Seconds()
		job.Meta.ProcessingTimeSec = &processingTime
	}

	return job
}

// getContentTypePath converts ContentType to path string (deprecated: use ContentType.ToPath())
func getContentTypePath(contentType model.ContentType) string {
	return contentType.ToPath()
}


// processVideoParallel processes video transcoding, thumbnails, preview, and sprite in parallel
func (s *EncoderService) processVideoParallel(ctx context.Context, job model.EncoderJob, inputFile, workDir string) model.EncoderJob {
	contentTypePath := getContentTypePath(job.ContentType)
	
	// Channels for parallel processing results
	type videoResult struct {
		quality model.VideoQuality
		err     error
	}
	type audioResult struct {
		track model.AudioTrack
		err   error
	}
	type thumbnailResult struct {
		path string
		err  error
	}
	
	videoResults := make(chan videoResult, len(job.Input.GeneratedQualities))
	audioResults := make(chan audioResult, 2)
	thumbnailResults := make(chan thumbnailResult, 5)
	previewDone := make(chan error, 1)
	spriteDone := make(chan error, 1)
	
	// Start video quality transcoding in parallel
	for _, qualityConfig := range job.Input.GeneratedQualities {
		go func(qc model.QualityConfig) {
			quality := strings.TrimSuffix(qc.Quality, "p")
			localDir := filepath.Join(workDir, "video", quality)
			
			err := utils.TranscodeToHLS(inputFile, localDir, qc.Quality, qc.Resolution, qc.VideoBitrate)
			if err != nil {
				videoResults <- videoResult{err: fmt.Errorf("quality %s: %w", qc.Quality, err)}
				return
			}
			
			// Upload to S3
			s3Dir := fmt.Sprintf("%s/%s/output/%s/video/%s", contentTypePath, job.ContentID, job.JobID, quality)
			if err := s.uploadDirectory(ctx, localDir, s3Dir); err != nil {
				videoResults <- videoResult{err: fmt.Errorf("upload %s: %w", qc.Quality, err)}
				return
			}
			
			chunkCount := utils.CountSegments(localDir)
			videoResults <- videoResult{
				quality: model.VideoQuality{
					Quality:    qc.Quality,
					Resolution: qc.Resolution,
					Playlist:   fmt.Sprintf("/%s/%s/output/%s/video/%s/index.m3u8", contentTypePath, job.ContentID, job.JobID, quality),
					ChunksDir:  fmt.Sprintf("/%s/%s/output/%s/video/%s/", contentTypePath, job.ContentID, job.JobID, quality),
					ChunkCount: chunkCount,
				},
			}
		}(qualityConfig)
	}
	
	// Start audio transcoding in parallel (if video has audio)
	if job.Input.HasEmbeddedAudio {
		// 256k audio
		go func() {
			localDir := filepath.Join(workDir, "audio", "256k")
			err := utils.TranscodeAudioToHLS(inputFile, localDir, "256k")
			if err != nil {
				audioResults <- audioResult{err: fmt.Errorf("audio 256k: %w", err)}
				return
			}
			
			s3Dir := fmt.Sprintf("%s/%s/output/%s/audio/256k", contentTypePath, job.ContentID, job.JobID)
			if err := s.uploadDirectory(ctx, localDir, s3Dir); err != nil {
				audioResults <- audioResult{err: fmt.Errorf("upload audio 256k: %w", err)}
				return
			}
			
			chunkCount := utils.CountSegments(localDir)
			audioResults <- audioResult{
				track: model.AudioTrack{
					Bitrate:    "256k",
					Playlist:   fmt.Sprintf("/%s/%s/output/%s/audio/256k/index.m3u8", contentTypePath, job.ContentID, job.JobID),
					ChunksDir:  fmt.Sprintf("/%s/%s/output/%s/audio/256k/", contentTypePath, job.ContentID, job.JobID),
					ChunkCount: chunkCount,
					Language:   "und",
					Label:      "Default Audio",
					Default:    true,
				},
			}
		}()
		
		// 128k audio
		go func() {
			localDir := filepath.Join(workDir, "audio", "128k")
			err := utils.TranscodeAudioToHLS(inputFile, localDir, "128k")
			if err != nil {
				audioResults <- audioResult{err: fmt.Errorf("audio 128k: %w", err)}
				return
			}
			
			s3Dir := fmt.Sprintf("%s/%s/output/%s/audio/128k", contentTypePath, job.ContentID, job.JobID)
			if err := s.uploadDirectory(ctx, localDir, s3Dir); err != nil {
				audioResults <- audioResult{err: fmt.Errorf("upload audio 128k: %w", err)}
				return
			}
			
			chunkCount := utils.CountSegments(localDir)
			audioResults <- audioResult{
				track: model.AudioTrack{
					Bitrate:    "128k",
					Playlist:   fmt.Sprintf("/%s/%s/output/%s/audio/128k/index.m3u8", contentTypePath, job.ContentID, job.JobID),
					ChunksDir:  fmt.Sprintf("/%s/%s/output/%s/audio/128k/", contentTypePath, job.ContentID, job.JobID),
					ChunkCount: chunkCount,
					Language:   "und",
					Label:      "Default Audio",
					Default:    false,
				},
			}
		}()
	}
	
	// Start thumbnail generation in parallel
	for i := 1; i <= 5; i++ {
		go func(index int) {
			timestamp := (job.Input.DurationSec / 6.0) * float64(index)
			localPath := filepath.Join(workDir, "thumbnails", fmt.Sprintf("thumbnail_%d.jpg", index))
			
			err := utils.GenerateThumbnail(inputFile, localPath, timestamp)
			if err != nil {
				thumbnailResults <- thumbnailResult{err: fmt.Errorf("thumbnail %d: %w", index, err)}
				return
			}
			
			s3Path := fmt.Sprintf("%s/%s/thumbnails/thumbnail_%d.jpg", contentTypePath, job.ContentID, index)
			if err := s.uploadFile(ctx, localPath, s3Path); err != nil {
				thumbnailResults <- thumbnailResult{err: fmt.Errorf("upload thumbnail %d: %w", index, err)}
				return
			}
			
			thumbnailResults <- thumbnailResult{path: fmt.Sprintf("/%s", s3Path)}
		}(i)
	}
	
	// Start preview generation
	go func() {
		previewDuration := 30.0
		if job.Input.DurationSec < 30.0 {
			previewDuration = job.Input.DurationSec
		}
		
		localPath := filepath.Join(workDir, "preview", "30sec.mp4")
		err := utils.GeneratePreview(inputFile, localPath, previewDuration)
		if err != nil {
			previewDone <- fmt.Errorf("preview: %w", err)
			return
		}
		
		s3Path := fmt.Sprintf("%s/%s/preview/30sec.mp4", contentTypePath, job.ContentID)
		if err := s.uploadFile(ctx, localPath, s3Path); err != nil {
			previewDone <- fmt.Errorf("upload preview: %w", err)
			return
		}
		
		previewDone <- nil
	}()
	
	// Start sprite generation
	go func() {
		localImagePath := filepath.Join(workDir, "preview", "sprite.jpg")
		localVTTPath := filepath.Join(workDir, "preview", "sprite.vtt")
		
		err := utils.GenerateSprite(inputFile, localImagePath, localVTTPath, job.Input.DurationSec)
		if err != nil {
			spriteDone <- fmt.Errorf("sprite: %w", err)
			return
		}
		
		s3ImagePath := fmt.Sprintf("%s/%s/preview/sprite.jpg", contentTypePath, job.ContentID)
		s3VTTPath := fmt.Sprintf("%s/%s/preview/sprite.vtt", contentTypePath, job.ContentID)
		
		if err := s.uploadFile(ctx, localImagePath, s3ImagePath); err != nil {
			spriteDone <- fmt.Errorf("upload sprite image: %w", err)
			return
		}
		
		if err := s.uploadFile(ctx, localVTTPath, s3VTTPath); err != nil {
			spriteDone <- fmt.Errorf("upload sprite vtt: %w", err)
			return
		}
		
		spriteDone <- nil
	}()
	
	// Collect video results
	var videoQualities []model.VideoQuality
	for i := 0; i < len(job.Input.GeneratedQualities); i++ {
		result := <-videoResults
		if result.err != nil {
			quality := ""
			if len(job.Input.GeneratedQualities) > i {
				quality = job.Input.GeneratedQualities[i].Quality
			}
			job.Errors = append(job.Errors, model.EncoderError{
				Code:    "VIDEO_TRANSCODE_FAILED",
				Message: "Video transcoding failed",
				Stage:   "video_transcoding",
				Quality: &quality,
				Path:    "",
				Details: result.err.Error(),
			})
		} else {
			videoQualities = append(videoQualities, result.quality)
		}
	}
	job.Output.Video.Qualities = videoQualities
	
	// Collect audio results
	var audioTracks []model.AudioTrack
	if job.Input.HasEmbeddedAudio {
		for i := 0; i < 2; i++ {
			result := <-audioResults
			if result.err != nil {
				job.Warnings = append(job.Warnings, model.EncoderWarning{
					Code:    "AUDIO_TRANSCODE_FAILED",
					Message: "Audio transcoding failed",
					Stage:   "audio_transcoding",
					Details: result.err.Error(),
				})
			} else {
				audioTracks = append(audioTracks, result.track)
			}
		}
	}
	job.Output.Audio.Tracks = audioTracks
	
	// Collect thumbnail results
	var thumbnailPaths []string
	for i := 0; i < 5; i++ {
		result := <-thumbnailResults
		if result.err != nil {
			job.Warnings = append(job.Warnings, model.EncoderWarning{
				Code:    "THUMBNAIL_GENERATION_FAILED",
				Message: "Thumbnail generation failed",
				Stage:   "thumbnail_generation",
				Details: result.err.Error(),
			})
		} else {
			thumbnailPaths = append(thumbnailPaths, result.path)
		}
	}
	job.Output.Thumbnails = model.ThumbnailsOutput{
		Dir:   fmt.Sprintf("/%s/%s/thumbnails/", contentTypePath, job.ContentID),
		Count: len(thumbnailPaths),
		Items: thumbnailPaths,
	}
	
	// Collect preview result
	if err := <-previewDone; err != nil {
		job.Warnings = append(job.Warnings, model.EncoderWarning{
			Code:    "PREVIEW_GENERATION_FAILED",
			Message: "Preview generation failed",
			Stage:   "preview_generation",
			Details: err.Error(),
		})
		job.Output.Preview = model.PreviewOutput{}
	} else {
		previewDuration := 30.0
		if job.Input.DurationSec < 30.0 {
			previewDuration = job.Input.DurationSec
		}
		previewFile := fmt.Sprintf("/%s/%s/preview/30sec.mp4", contentTypePath, job.ContentID)
		job.Output.Preview = model.PreviewOutput{
			File:        &previewFile,
			DurationSec: &previewDuration,
		}
	}
	
	// Collect sprite result
	if err := <-spriteDone; err != nil {
		job.Warnings = append(job.Warnings, model.EncoderWarning{
			Code:    "SPRITE_GENERATION_FAILED",
			Message: "Sprite generation failed",
			Stage:   "sprite_generation",
			Details: err.Error(),
		})
		job.Output.Sprite = model.SpriteOutput{}
	} else {
		spriteImage := fmt.Sprintf("/%s/%s/preview/sprite.jpg", contentTypePath, job.ContentID)
		spriteVTT := fmt.Sprintf("/%s/%s/preview/sprite.vtt", contentTypePath, job.ContentID)
		job.Output.Sprite = model.SpriteOutput{
			Image: &spriteImage,
			VTT:   &spriteVTT,
		}
	}
	
	// Generate master playlists if we have video qualities
	if len(videoQualities) > 0 {
		baseOutputDir := fmt.Sprintf("/%s/%s/output/%s", contentTypePath, job.ContentID, job.JobID)
		job.Output.BaseOutputDir = baseOutputDir
		
		recordPlaylist := fmt.Sprintf("%s/%s_master.m3u8", baseOutputDir, job.JobID)
		currentPlaylist := fmt.Sprintf("/%s/%s/output/master.m3u8", contentTypePath, job.ContentID)
		
		// Generate master playlist for record (with relative paths from job directory)
		localRecordMasterPath := filepath.Join(workDir, "record_master.m3u8")
		recordQualities := []utils.QualityPlaylist{}
		for _, q := range videoQualities {
			recordQualities = append(recordQualities, utils.QualityPlaylist{
				Quality:      q.Quality,
				Resolution:   q.Resolution,
				Bandwidth:    model.GetBandwidth(q.Quality),
				RelativePath: fmt.Sprintf("video/%s/index.m3u8", strings.TrimSuffix(q.Quality, "p")),
			})
		}
		
		// Generate master playlist for current (with absolute paths including job ID)
		localCurrentMasterPath := filepath.Join(workDir, "current_master.m3u8")
		currentQualities := []utils.QualityPlaylist{}
		for _, q := range videoQualities {
			currentQualities = append(currentQualities, utils.QualityPlaylist{
				Quality:      q.Quality,
				Resolution:   q.Resolution,
				Bandwidth:    model.GetBandwidth(q.Quality),
				RelativePath: fmt.Sprintf("%s/video/%s/index.m3u8", job.JobID, strings.TrimSuffix(q.Quality, "p")),
			})
		}
		
		// Generate both playlists
		if err := utils.GenerateMasterPlaylist(localRecordMasterPath, recordQualities, []utils.AudioPlaylist{}); err != nil {
			job.Errors = append(job.Errors, model.EncoderError{
				Code:    "MASTER_PLAYLIST_GENERATION_FAILED",
				Message: "Failed to generate record master playlist",
				Stage:   "master_playlist_generation",
				Details: err.Error(),
			})
		} else if err := utils.GenerateMasterPlaylist(localCurrentMasterPath, currentQualities, []utils.AudioPlaylist{}); err != nil {
			job.Errors = append(job.Errors, model.EncoderError{
				Code:    "MASTER_PLAYLIST_GENERATION_FAILED",
				Message: "Failed to generate current master playlist",
				Stage:   "master_playlist_generation",
				Details: err.Error(),
			})
		} else {
			// Upload both master playlists
			s3RecordPath := fmt.Sprintf("%s/%s/output/%s/%s_master.m3u8", contentTypePath, job.ContentID, job.JobID, job.JobID)
			s3CurrentPath := fmt.Sprintf("%s/%s/output/master.m3u8", contentTypePath, job.ContentID)
			
			if err := s.uploadFile(ctx, localRecordMasterPath, s3RecordPath); err == nil {
				if err := s.uploadFile(ctx, localCurrentMasterPath, s3CurrentPath); err == nil {
					job.Output.MasterPlaylists = model.MasterPlaylists{
						RecordMasterPlaylist:  &recordPlaylist,
						CurrentMasterPlaylist: &currentPlaylist,
					}
				}
			}
		}
	}
	
	// Determine final status
	if len(job.Errors) > 0 {
		job.Status = model.EncoderStatusFailed
		failedTime := time.Now()
		job.Meta.FailedAt = &failedTime
	} else if len(job.Warnings) > 0 {
		job.Status = model.EncoderStatusCompletedWithWarnings
		completedTime := time.Now()
		job.Meta.CompletedAt = &completedTime
	} else {
		job.Status = model.EncoderStatusCompleted
		completedTime := time.Now()
		job.Meta.CompletedAt = &completedTime
	}
	
	if job.Meta.StartedAt != nil {
		endTime := time.Now()
		processingTime := endTime.Sub(*job.Meta.StartedAt).Seconds()
		job.Meta.ProcessingTimeSec = &processingTime
	}
	
	// Build playback object if encoding completed successfully
	if job.Status == model.EncoderStatusCompleted || job.Status == model.EncoderStatusCompletedWithWarnings {
		job.Playback = s.buildPlaybackResponse(&job)
	}
	
	return job
}

// uploadFile uploads a single file to S3
func (s *EncoderService) uploadFile(ctx context.Context, localPath, s3Path string) error {
	data, err := os.ReadFile(localPath)
	if err != nil {
		return fmt.Errorf("failed to read file: %w", err)
	}
	
	// Determine content type
	contentType := "application/octet-stream"
	ext := filepath.Ext(localPath)
	switch ext {
	case ".m3u8":
		contentType = "application/vnd.apple.mpegurl"
	case ".ts":
		contentType = "video/MP2T"
	case ".mp4":
		contentType = "video/mp4"
	case ".jpg", ".jpeg":
		contentType = "image/jpeg"
	case ".vtt":
		contentType = "text/vtt"
	}
	
	_, err = s.fileUploader.UploadFile(ctx, "", "", s3Path, filepath.Base(localPath), contentType, data)
	return err
}

// uploadDirectory uploads all files in a directory to S3
func (s *EncoderService) uploadDirectory(ctx context.Context, localDir, s3Dir string) error {
	return filepath.Walk(localDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		
		relPath, err := filepath.Rel(localDir, path)
		if err != nil {
			return err
		}
		
		s3Path := filepath.Join(s3Dir, relPath)
		return s.uploadFile(ctx, path, s3Path)
	})
}

// failJob marks a job as failed with an error
func (s *EncoderService) failJob(ctx context.Context, job *model.EncoderJob, code, message, stage, path string, err error) {
	if code != "" {
		details := ""
		if err != nil {
			details = err.Error()
		}
		job.Errors = append(job.Errors, model.EncoderError{
			Code:    code,
			Message: message,
			Stage:   stage,
			Path:    path,
			Details: details,
		})
	}
	
	job.Status = model.EncoderStatusFailed
	failedTime := time.Now()
	job.Meta.FailedAt = &failedTime
	
	if job.Meta.StartedAt != nil {
		processingTime := failedTime.Sub(*job.Meta.StartedAt).Seconds()
		job.Meta.ProcessingTimeSec = &processingTime
	}
	
	s.repo.Update(ctx, *job)
}

// buildPlaybackResponse builds a playback response from an encoder job
func (s *EncoderService) buildPlaybackResponse(job *model.EncoderJob) *model.PlaybackResponse {
	response := &model.PlaybackResponse{
		DurationSec: job.Input.DurationSec,
		Streaming: model.StreamingInfo{
			Type:           "HLS",
			MasterPlaylist: "",
		},
		Video: model.VideoInfo{
			Qualities:      []string{},
			DefaultQuality: "auto",
		},
		Audio: model.AudioInfo{
			Tracks:         []model.AudioTrackInfo{},
			DefaultTrackID: "",
		},
		Subtitles: model.SubtitlesInfo{
			Tracks:         []model.SubtitleTrackInfo{},
			DefaultTrackID: nil,
		},
		Preview: model.PreviewInfo{
			Video: "",
		},
		Thumbnails: model.ThumbnailsInfo{
			Poster: "",
			Items:  []string{},
		},
		Sprite: model.SpriteInfo{
			Image: "",
			VTT:   "",
		},
	}
	
	// Set master playlist
	if job.Output.MasterPlaylists.CurrentMasterPlaylist != nil {
		response.Streaming.MasterPlaylist = *job.Output.MasterPlaylists.CurrentMasterPlaylist
	}
	
	// Set video qualities
	for _, quality := range job.Output.Video.Qualities {
		response.Video.Qualities = append(response.Video.Qualities, quality.Quality)
	}
	
	// Set audio tracks
	for _, track := range job.Output.Audio.Tracks {
		audioTrack := model.AudioTrackInfo{
			ID:       fmt.Sprintf("audio_%s_%s", track.Bitrate, track.Language),
			Label:    track.Label,
			Language: track.Language,
			Bitrate:  track.Bitrate,
			Default:  track.Default,
		}
		response.Audio.Tracks = append(response.Audio.Tracks, audioTrack)
		
		if track.Default {
			response.Audio.DefaultTrackID = audioTrack.ID
		}
	}
	
	// Set subtitles (currently empty, will be populated when subtitle support is added)
	// Subtitles remain empty for now
	
	// Set preview
	if job.Output.Preview.File != nil {
		response.Preview.Video = *job.Output.Preview.File
	}
	
	// Set thumbnails
	if len(job.Output.Thumbnails.Items) > 0 {
		response.Thumbnails.Poster = job.Output.Thumbnails.Items[0]
		response.Thumbnails.Items = job.Output.Thumbnails.Items
	}
	
	// Set sprite
	if job.Output.Sprite.Image != nil {
		response.Sprite.Image = *job.Output.Sprite.Image
	}
	if job.Output.Sprite.VTT != nil {
		response.Sprite.VTT = *job.Output.Sprite.VTT
	}
	
	return response
}

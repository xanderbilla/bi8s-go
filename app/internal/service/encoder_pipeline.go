package service

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/xanderbilla/bi8s-go/internal/logger"
	"github.com/xanderbilla/bi8s-go/internal/model"
	"github.com/xanderbilla/bi8s-go/internal/utils"
)

func (s *EncoderService) processEncodingJobFromFile(ctx context.Context, jobPtr *model.EncoderJob, tempFile string) {
	job := *jobPtr
	logger.InfoContext(ctx, "starting encoding job processing",
		"job_id", job.JobID,
		"content_id", job.ContentID,
	)

	job.Status = model.EncoderStatusProcessing
	startTime := time.Now()
	job.Meta.StartedAt = &startTime
	if err := s.repo.Update(ctx, &job); err != nil {
		logger.ErrorContext(ctx, "failed to persist processing status",
			"job_id", job.JobID,
			"error", err.Error(),
		)
	}

	logger.InfoContext(ctx, "extracting video metadata",
		"job_id", job.JobID,
		"temp_file", tempFile,
	)

	job = s.extractVideoMetadata(ctx, job, tempFile)
	if len(job.Errors) > 0 {
		firstErr := job.Errors[0]
		logger.ErrorContext(ctx, "metadata extraction failed",
			"job_id", job.JobID,
			"error_count", len(job.Errors),
			"code", firstErr.Code,
			"stage", firstErr.Stage,
			"details", firstErr.Details,
		)
		s.failJob(ctx, &job, "", "", "", "", nil)
		return
	}

	logger.InfoContext(ctx, "video metadata extracted successfully",
		"job_id", job.JobID,
		"resolution", job.Input.Resolution,
		"duration", job.Input.DurationSec,
		"has_audio", job.Input.HasEmbeddedAudio,
	)

	if err := s.repo.Update(ctx, &job); err != nil {
		logger.ErrorContext(ctx, "failed to persist extracted metadata",
			"job_id", job.JobID,
			"error", err.Error(),
		)
	}

	workDir, err := os.MkdirTemp("/tmp", fmt.Sprintf("bi8s-work-%s-*", job.JobID))
	if err != nil {
		logger.ErrorContext(ctx, "failed to create working directory",
			"job_id", job.JobID,
			"error", err.Error(),
		)
		s.failJob(ctx, &job, "WORK_DIR_CREATION_FAILED", "Failed to create working directory", "initialization", workDir, err)
		return
	}
	defer os.RemoveAll(workDir)

	logger.InfoContext(ctx, "starting parallel video processing",
		"job_id", job.JobID,
		"qualities_count", len(job.Input.GeneratedQualities),
	)

	job = s.processVideoParallel(ctx, job, tempFile, workDir)

	logger.InfoContext(ctx, "encoding job completed",
		"job_id", job.JobID,
		"status", job.Status,
		"duration", time.Since(startTime).Seconds(),
		"error_count", len(job.Errors),
		"warning_count", len(job.Warnings),
	)

	*jobPtr = job
	if err := s.repo.Update(ctx, jobPtr); err != nil {
		logger.ErrorContext(ctx, "failed to persist final encoding status",
			"job_id", job.JobID,
			"status", job.Status,
			"error", err.Error(),
		)
	}
}

func (s *EncoderService) extractVideoMetadata(ctx context.Context, job model.EncoderJob, videoPath string) model.EncoderJob {
	metadata, err := utils.GetVideoMetadata(ctx, videoPath)
	if err != nil {

		job.Errors = append(job.Errors, model.EncoderError{
			Code:    "METADATA_EXTRACTION_FAILED",
			Message: "Failed to extract video metadata",
			Stage:   "metadata_extraction",
			Path:    videoPath,
			Details: err.Error(),
		})
		return job
	}

	job.Input.Resolution = metadata.GetResolutionString()
	job.Input.DurationSec = metadata.Duration
	job.Input.VideoCodec = metadata.VideoCodec
	job.Input.AudioCodec = metadata.AudioCodec
	job.Input.HasEmbeddedAudio = metadata.HasAudio
	job.Input.HasExternalAudio = false
	job.Input.HasSubtitles = false

	qualities := metadata.DetermineQualities()
	job.Input.GeneratedQualities = []model.QualityConfig{}

	for _, quality := range qualities {
		config := getQualityConfig(quality)
		job.Input.GeneratedQualities = append(job.Input.GeneratedQualities, config)
	}

	return job
}

func getQualityConfig(quality string) model.QualityConfig {
	if spec, ok := model.GetQualitySpec(quality); ok {
		return model.QualityConfig{
			Quality:      spec.Quality,
			Resolution:   spec.Resolution,
			VideoBitrate: spec.VideoBitrate,
		}
	}
	return model.QualityConfig{}
}

func (s *EncoderService) processVideoParallel(ctx context.Context, job model.EncoderJob, inputFile, workDir string) model.EncoderJob {
	contentTypePath := job.ContentType.ToPath()

	type videoResult struct {
		inputQuality string
		quality      model.VideoQuality
		err          error
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

	for _, qualityConfig := range job.Input.GeneratedQualities {
		go func(qc model.QualityConfig) {
			defer func() {
				if r := recover(); r != nil {
					videoResults <- videoResult{inputQuality: qc.Quality, err: fmt.Errorf("panic: %v", r)}
				}
			}()
			quality := strings.TrimSuffix(qc.Quality, "p")
			localDir := filepath.Join(workDir, "video", quality)

			err := utils.TranscodeToHLS(ctx, inputFile, localDir, qc.Quality, qc.Resolution, qc.VideoBitrate)
			if err != nil {
				videoResults <- videoResult{inputQuality: qc.Quality, err: fmt.Errorf("quality %s: %w", qc.Quality, err)}
				return
			}

			s3Dir := fmt.Sprintf("%s/%s/output/%s/video/%s", contentTypePath, job.ContentID, job.JobID, quality)
			if err := s.uploadDirectory(ctx, localDir, s3Dir); err != nil {
				videoResults <- videoResult{inputQuality: qc.Quality, err: fmt.Errorf("upload %s: %w", qc.Quality, err)}
				return
			}

			chunkCount := utils.CountSegments(localDir)
			videoResults <- videoResult{
				inputQuality: qc.Quality,
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

	if job.Input.HasEmbeddedAudio {

		go func() {
			defer func() {
				if r := recover(); r != nil {
					audioResults <- audioResult{err: fmt.Errorf("audio 256k panic: %v", r)}
				}
			}()
			localDir := filepath.Join(workDir, "audio", "256k")
			err := utils.TranscodeAudioToHLS(ctx, inputFile, localDir, "256k")
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

		go func() {
			defer func() {
				if r := recover(); r != nil {
					audioResults <- audioResult{err: fmt.Errorf("audio 128k panic: %v", r)}
				}
			}()
			localDir := filepath.Join(workDir, "audio", "128k")
			err := utils.TranscodeAudioToHLS(ctx, inputFile, localDir, "128k")
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

	for i := 1; i <= 5; i++ {
		go func(index int) {
			defer func() {
				if r := recover(); r != nil {
					thumbnailResults <- thumbnailResult{err: fmt.Errorf("thumbnail %d panic: %v", index, r)}
				}
			}()
			timestamp := (job.Input.DurationSec / 6.0) * float64(index)
			localPath := filepath.Join(workDir, "thumbnails", fmt.Sprintf("thumbnail_%d.jpg", index))

			err := utils.GenerateThumbnail(ctx, inputFile, localPath, timestamp)
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

	go func() {
		defer func() {
			if r := recover(); r != nil {
				previewDone <- fmt.Errorf("preview panic: %v", r)
			}
		}()
		previewDuration := 30.0
		if job.Input.DurationSec < 30.0 {
			previewDuration = job.Input.DurationSec
		}

		localPath := filepath.Join(workDir, "preview", "30sec.mp4")
		err := utils.GeneratePreview(ctx, inputFile, localPath, previewDuration)
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

	go func() {
		defer func() {
			if r := recover(); r != nil {
				spriteDone <- fmt.Errorf("sprite panic: %v", r)
			}
		}()
		localImagePath := filepath.Join(workDir, "preview", "sprite.jpg")
		localVTTPath := filepath.Join(workDir, "preview", "sprite.vtt")

		err := utils.GenerateSprite(ctx, inputFile, localImagePath, localVTTPath, job.Input.DurationSec)
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

	var videoQualities []model.VideoQuality
	var audioTracks []model.AudioTrack
	var thumbnailPaths []string
	var errors []model.EncoderError
	var warnings []model.EncoderWarning

	for i := 0; i < len(job.Input.GeneratedQualities); i++ {
		result := <-videoResults
		if result.err != nil {
			quality := result.inputQuality
			errors = append(errors, model.EncoderError{
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

	if job.Input.HasEmbeddedAudio {
		for i := 0; i < 2; i++ {
			result := <-audioResults
			if result.err != nil {
				warnings = append(warnings, model.EncoderWarning{
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

	for i := 0; i < 5; i++ {
		result := <-thumbnailResults
		if result.err != nil {
			warnings = append(warnings, model.EncoderWarning{
				Code:    "THUMBNAIL_GENERATION_FAILED",
				Message: "Thumbnail generation failed",
				Stage:   "thumbnail_generation",
				Details: result.err.Error(),
			})
		} else {
			thumbnailPaths = append(thumbnailPaths, result.path)
		}
	}

	if err := <-previewDone; err != nil {
		warnings = append(warnings, model.EncoderWarning{
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

	if err := <-spriteDone; err != nil {
		warnings = append(warnings, model.EncoderWarning{
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

	job.Output.Video.Qualities = videoQualities
	job.Output.Audio.Tracks = audioTracks
	job.Output.Thumbnails.Items = thumbnailPaths
	job.Output.Thumbnails.Count = len(thumbnailPaths)
	job.Errors = append(job.Errors, errors...)
	job.Warnings = append(job.Warnings, warnings...)

	if len(videoQualities) > 0 {
		baseOutputDir := fmt.Sprintf("/%s/%s/output/%s", contentTypePath, job.ContentID, job.JobID)
		job.Output.BaseOutputDir = baseOutputDir

		recordPlaylist := fmt.Sprintf("%s/%s_master.m3u8", baseOutputDir, job.JobID)
		currentPlaylist := fmt.Sprintf("/%s/%s/output/master.m3u8", contentTypePath, job.ContentID)

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

	errorCount := len(job.Errors)
	warningCount := len(job.Warnings)

	if errorCount > 0 {
		job.Status = model.EncoderStatusFailed
		failedTime := time.Now()
		job.Meta.FailedAt = &failedTime
	} else if warningCount > 0 {
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

	if job.Status == model.EncoderStatusCompleted || job.Status == model.EncoderStatusCompletedWithWarnings {
		job.Playback = s.buildPlaybackResponse(&job)
	}

	return job
}

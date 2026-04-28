package service

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/xanderbilla/bi8s-go/internal/logger"
	"github.com/xanderbilla/bi8s-go/internal/model"
)

func (s *EncoderService) uploadFile(ctx context.Context, localPath, s3Path string) error {
	f, err := os.Open(localPath)
	if err != nil {
		return fmt.Errorf("failed to open file: %w", err)
	}
	defer f.Close()

	stat, err := f.Stat()
	if err != nil {
		return fmt.Errorf("failed to stat file: %w", err)
	}

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

	_, err = s.fileUploader.UploadFileStream(ctx, "", "", s3Path, filepath.Base(localPath), contentType, f, stat.Size())
	return err
}

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

	if err := s.repo.Update(ctx, job); err != nil {
		logger.ErrorContext(ctx, "failed to persist failed job status",
			"job_id", job.JobID,
			"error", err.Error(),
		)
	}
}

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

	if job.Output.MasterPlaylists.CurrentMasterPlaylist != nil {
		response.Streaming.MasterPlaylist = *job.Output.MasterPlaylists.CurrentMasterPlaylist
	}

	for _, quality := range job.Output.Video.Qualities {
		response.Video.Qualities = append(response.Video.Qualities, quality.Quality)
	}

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

	if job.Output.Preview.File != nil {
		response.Preview.Video = *job.Output.Preview.File
	}

	if len(job.Output.Thumbnails.Items) > 0 {
		response.Thumbnails.Poster = job.Output.Thumbnails.Items[0]
		response.Thumbnails.Items = job.Output.Thumbnails.Items
	}

	if job.Output.Sprite.Image != nil {
		response.Sprite.Image = *job.Output.Sprite.Image
	}
	if job.Output.Sprite.VTT != nil {
		response.Sprite.VTT = *job.Output.Sprite.VTT
	}

	return response
}

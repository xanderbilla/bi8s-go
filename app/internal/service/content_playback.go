package service

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/xanderbilla/bi8s-go/internal/errs"
	"github.com/xanderbilla/bi8s-go/internal/logger"
	"github.com/xanderbilla/bi8s-go/internal/model"
)

func (s *ContentService) GetPlaybackInfo(ctx context.Context, contentID string) (*model.PlaybackInfo, error) {
	content, err := s.repo.Get(ctx, contentID)
	if err != nil {
		return nil, err
	}
	if content == nil {
		return nil, errs.ErrContentNotFound
	}

	latestJob, err := s.findLatestCompletedJob(ctx, contentID)
	if err != nil {
		return nil, fmt.Errorf("get encoder jobs: %w", err)
	}
	if latestJob == nil {
		return nil, errs.ErrNoCompletedEncoding
	}
	if latestJob.Playback == nil {
		return nil, errs.ErrPlaybackNotAvailable
	}

	playback := *latestJob.Playback
	if signer, ok := s.fileUploader.(interface {
		GeneratePresignedGetURL(ctx context.Context, key string, expiry time.Duration) (string, error)
	}); ok {
		master := strings.TrimSpace(playback.Streaming.MasterPlaylist)
		if master != "" && !strings.HasPrefix(strings.ToLower(master), "http://") && !strings.HasPrefix(strings.ToLower(master), "https://") {
			signed, signErr := signer.GeneratePresignedGetURL(ctx, strings.TrimPrefix(master, "/"), s.playbackURLTTL)
			if signErr != nil {
				logger.WarnContext(ctx, "failed to presign playback URL", "contentId", contentID, "error", signErr)
			} else {
				playback.Streaming.MasterPlaylist = signed
			}
		}
	}

	return &model.PlaybackInfo{
		ContentID:   contentID,
		ContentType: content.ContentType,
		Info: model.PlaybackMeta{
			Title:    content.Title,
			Overview: content.Overview,
			Casts:    content.Casts,
		},
		Playback: &playback,
	}, nil
}

func (s *ContentService) findLatestCompletedJob(ctx context.Context, contentID string) (*model.EncoderJob, error) {
	jobs, err := s.encoderRepo.GetByContentId(ctx, contentID)
	if err != nil {
		return nil, err
	}
	var latest *model.EncoderJob
	for i := range jobs {
		j := &jobs[i]
		if j.Status != model.EncoderStatusCompleted && j.Status != model.EncoderStatusCompletedWithWarnings {
			continue
		}
		if latest == nil {
			latest = j
			continue
		}
		if j.Meta.CompletedAt != nil && latest.Meta.CompletedAt != nil &&
			j.Meta.CompletedAt.After(*latest.Meta.CompletedAt) {
			latest = j
		}
	}
	return latest, nil
}

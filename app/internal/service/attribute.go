package service

import (
	"context"
	"sync"
	"time"

	"github.com/xanderbilla/bi8s-go/internal/errs"
	"github.com/xanderbilla/bi8s-go/internal/model"
	"github.com/xanderbilla/bi8s-go/internal/repository"
	"github.com/xanderbilla/bi8s-go/internal/utils"
)

const attributeLocalTTL = 10 * time.Minute

type AttributeService struct {
	repo        repository.AttributeRepository
	allCache    []model.Attribute
	allCacheExp time.Time
	allCacheMu  sync.RWMutex
}

func NewAttributeService(repo repository.AttributeRepository) *AttributeService {
	return &AttributeService{
		repo: repo,
	}
}

func (s *AttributeService) GetAll(ctx context.Context) ([]model.Attribute, error) {
	s.allCacheMu.RLock()
	if s.allCache != nil && time.Now().Before(s.allCacheExp) {
		result := s.allCache
		s.allCacheMu.RUnlock()
		return result, nil
	}
	s.allCacheMu.RUnlock()

	s.allCacheMu.Lock()
	defer s.allCacheMu.Unlock()

	if s.allCache != nil && time.Now().Before(s.allCacheExp) {
		return s.allCache, nil
	}
	attrs, err := s.repo.GetAll(ctx)
	if err != nil {
		return nil, err
	}
	s.allCache = attrs
	s.allCacheExp = time.Now().Add(attributeLocalTTL)
	return attrs, nil
}

// InvalidateCache clears the in-process attribute cache.
// Call after Create or Delete to keep the cache consistent.
func (s *AttributeService) InvalidateCache() {
	s.allCacheMu.Lock()
	s.allCache = nil
	s.allCacheMu.Unlock()
}

func (s *AttributeService) Get(ctx context.Context, id string) (*model.Attribute, error) {
	attrs, err := s.GetAll(ctx)
	if err != nil {
		return nil, err
	}
	for i := range attrs {
		if attrs[i].ID == id {
			return &attrs[i], nil
		}
	}
	return nil, errs.ErrContentNotFound
}

func (s *AttributeService) Create(ctx context.Context, attribute model.Attribute) (model.Attribute, error) {

	if attribute.ID == "" {
		attribute.ID = utils.GenerateID()
	}

	existing, err := s.repo.GetByName(ctx, attribute.Name)
	if err != nil {
		return model.Attribute{}, err
	}
	if existing != nil {
		return model.Attribute{}, errs.ErrAttributeNameTaken
	}

	attribute.ContentType = model.ContentTypeAttribute

	now := time.Now()
	attribute.Audit = model.Audit{
		CreatedAt: now,
		Version:   1,
	}

	if err := s.repo.Create(ctx, attribute); err != nil {
		return model.Attribute{}, err
	}
	s.InvalidateCache()
	return attribute, nil
}

func (s *AttributeService) Delete(ctx context.Context, id string) error {
	if err := s.repo.Delete(ctx, id); err != nil {
		if errs.IsConditionalCheckFailed(err) {
			return errs.NewNotFound("attribute")
		}
		return err
	}
	s.InvalidateCache()
	return nil
}

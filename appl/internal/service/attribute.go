package service

import (
	"context"
	"time"

	"github.com/xanderbilla/bi8s-go/internal/errs"
	"github.com/xanderbilla/bi8s-go/internal/model"
	"github.com/xanderbilla/bi8s-go/internal/repository"
	"github.com/xanderbilla/bi8s-go/internal/utils"
)

type AttributeService struct {
	repo repository.AttributeRepository
}

func NewAttributeService(repo repository.AttributeRepository) *AttributeService {
	return &AttributeService{
		repo: repo,
	}
}

func (s *AttributeService) GetAll(ctx context.Context) ([]model.Attribute, error) {
	return s.repo.GetAll(ctx)
}

func (s *AttributeService) Get(ctx context.Context, id string) (*model.Attribute, error) {
	a, err := s.repo.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	if a == nil {
		return nil, errs.ErrContentNotFound
	}
	return a, nil
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
	attribute.Active = true

	now := time.Now()
	attribute.Audit = model.Audit{
		CreatedAt: now,
		Version:   1,
	}

	if err := s.repo.Create(ctx, attribute); err != nil {
		return model.Attribute{}, err
	}

	return attribute, nil
}

func (s *AttributeService) Delete(ctx context.Context, id string) error {
	return s.repo.Delete(ctx, id)
}

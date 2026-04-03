package service

import (
	"context"
	"errors"
	"time"

	"github.com/xanderbilla/bi8s-go/internal/model"
	"github.com/xanderbilla/bi8s-go/internal/repository"
	"github.com/xanderbilla/bi8s-go/internal/utils"
)

// AttributeService handles business logic for attribute operations.
type AttributeService struct {
	repo repository.AttributeRepository
}

// NewAttributeService creates a new AttributeService.
func NewAttributeService(repo repository.AttributeRepository) *AttributeService {
	return &AttributeService{
		repo: repo,
	}
}

// GetAll returns all attributes from the repository.
func (s *AttributeService) GetAll(ctx context.Context) ([]model.Attribute, error) {
	return s.repo.GetAll(ctx)
}

// Get returns a single attribute by ID.
func (s *AttributeService) Get(ctx context.Context, id string) (*model.Attribute, error) {
	return s.repo.Get(ctx, id)
}

// Create saves a new attribute after validating uniqueness by name.
func (s *AttributeService) Create(ctx context.Context, attribute model.Attribute) (model.Attribute, error) {
	// Generate ID using 6-digit formula
	if attribute.ID == "" {
		attribute.ID = utils.GenerateNumericID()
	}

	// Check if attribute with same name already exists
	existing, err := s.repo.GetByName(ctx, attribute.Name)
	if err != nil {
		return model.Attribute{}, err
	}
	if existing != nil {
		return model.Attribute{}, errors.New("attribute with this name already exists")
	}

	// Set default values
	attribute.ContentType = model.ContentTypeAttribute
	attribute.Active = true // Always set to true by default

	// Set audit timestamps
	now := time.Now()
	attribute.Audit = model.Audit{
		CreatedAt: now,
		IsDeleted: false,
		Version:   1,
	}

	if err := s.repo.Create(ctx, attribute); err != nil {
		return model.Attribute{}, err
	}

	return attribute, nil
}

// Delete removes an attribute from the repository.
func (s *AttributeService) Delete(ctx context.Context, id string) error {
	return s.repo.Delete(ctx, id)
}

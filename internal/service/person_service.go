package service

import (
	"context"
	"time"

	"github.com/xanderbilla/bi8s-go/internal/domain"
	"github.com/xanderbilla/bi8s-go/internal/errs"
	"github.com/xanderbilla/bi8s-go/internal/model"
	"github.com/xanderbilla/bi8s-go/internal/repository"
	"github.com/xanderbilla/bi8s-go/internal/storage"
	"github.com/xanderbilla/bi8s-go/internal/utils"
	"github.com/xanderbilla/bi8s-go/internal/validation"
)

// PersonService handles business logic for person operations.
type PersonService struct {
	repo          repository.PersonRepository
	attributeRepo repository.AttributeRepository
	fileUploader  storage.FileUploader
}

// NewPersonService creates a new PersonService.
func NewPersonService(repo repository.PersonRepository, attributeRepo repository.AttributeRepository, fileUploader storage.FileUploader) *PersonService {
	return &PersonService{
		repo:          repo,
		attributeRepo: attributeRepo,
		fileUploader:  fileUploader,
	}
}

// GetAll returns all persons from the repository.
func (s *PersonService) GetAll(ctx context.Context) ([]model.Person, error) {
	return s.repo.GetAll(ctx)
}

// Get returns a single person by ID.
func (s *PersonService) Get(ctx context.Context, id string) (*model.Person, error) {
	return s.repo.Get(ctx, id)
}

// Create saves a new person with optional profile and backdrop uploads.
func (s *PersonService) Create(ctx context.Context, person model.Person, profileInput, backdropInput *domain.FileUploadInput) (model.Person, error) {
	// Generate ID using 6-digit formula
	if person.ID == "" {
		person.ID = utils.GenerateNumericID()
	}

	// Set default values
	person.ContentType = model.ContentTypePerson
	person.Verified = false

	// If stageName is not provided, use name
	if person.StageName == "" {
		person.StageName = person.Name
	}

	// Validate and populate tags (must have TAG attribute type)
	validatedTags, err := validation.ValidateAndPopulateAttributes(ctx, person.Tags, model.AttributeTypeTag, s.attributeRepo)
	if err != nil {
		return model.Person{}, err
	}
	person.Tags = validatedTags

	// Validate and populate categories (must have CATEGORY attribute type)
	validatedCategories, err := validation.ValidateAndPopulateAttributes(ctx, person.Categories, model.AttributeTypeCategory, s.attributeRepo)
	if err != nil {
		return model.Person{}, err
	}
	person.Categories = validatedCategories

	// Validate and populate specialties (must have SPECIALITY attribute type)
	validatedSpecialties, err := validation.ValidateAndPopulateAttributes(ctx, person.Specialties, model.AttributeTypeSpeciality, s.attributeRepo)
	if err != nil {
		return model.Person{}, err
	}
	person.Specialties = validatedSpecialties

	// Initialize stats with zero values
	person.Stats = model.Stats{
		TotalProductions: 0,
		TotalViews:       0,
		SubscriberCount:  0,
		FollowersCount:   0,
		AverageRating:    0.0,
	}

	// Set audit timestamps
	now := time.Now()
	person.Audit = model.Audit{
		CreatedAt: now,
		IsDeleted: false,
		Version:   1,
	}

	// Upload profile image
	if profileInput != nil {
		profileKey, err := s.uploadFileToStorage(ctx, person.ID, "profile", profileInput)
		if err != nil {
			return model.Person{}, err
		}
		person.ProfilePath = profileKey
	}

	// Upload backdrop image
	if backdropInput != nil {
		backdropKey, err := s.uploadFileToStorage(ctx, person.ID, "cover", backdropInput)
		if err != nil {
			return model.Person{}, err
		}
		person.BackdropPath = backdropKey
	}

	if err := s.repo.Create(ctx, person); err != nil {
		return model.Person{}, err
	}

	return person, nil
}

// uploadFileToStorage handles file uploads to S3 for person images.
func (s *PersonService) uploadFileToStorage(ctx context.Context, personID, purpose string, input *domain.FileUploadInput) (string, error) {
	if s.fileUploader == nil {
		return "", errs.ErrFileUploaderNotConfigured
	}

	return s.fileUploader.UploadFile(
		ctx,
		"person",
		personID,
		purpose,
		input.FileName,
		input.ContentType,
		input.Data,
	)
}

// Delete removes a person from the repository.
func (s *PersonService) Delete(ctx context.Context, id string) error {
	return s.repo.Delete(ctx, id)
}

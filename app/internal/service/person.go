package service

import (
	"context"
	"time"

	"github.com/xanderbilla/bi8s-go/internal/errs"
	"github.com/xanderbilla/bi8s-go/internal/model"
	"github.com/xanderbilla/bi8s-go/internal/repository"
	"github.com/xanderbilla/bi8s-go/internal/storage"
	"github.com/xanderbilla/bi8s-go/internal/utils"
	"github.com/xanderbilla/bi8s-go/internal/validation"
)

type PersonService struct {
	repo          repository.PersonRepository
	attributeRepo repository.AttributeRepository
	fileUploader  storage.FileUploader
}

func NewPersonService(repo repository.PersonRepository, attributeRepo repository.AttributeRepository, fileUploader storage.FileUploader) *PersonService {
	return &PersonService{
		repo:          repo,
		attributeRepo: attributeRepo,
		fileUploader:  fileUploader,
	}
}

func (s *PersonService) GetAll(ctx context.Context) ([]model.Person, error) {
	return s.repo.GetAll(ctx)
}

func (s *PersonService) Get(ctx context.Context, id string) (*model.Person, error) {
	p, err := s.repo.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	if p == nil {
		return nil, errs.ErrContentNotFound
	}
	return p, nil
}

func (s *PersonService) Create(ctx context.Context, person model.Person, profileInput, backdropInput *model.FileUploadInput) (model.Person, error) {

	if person.ID == "" {
		person.ID = utils.GenerateNumericID()
	}

	person.ContentType = model.ContentTypePerson
	person.Verified = false

	if person.StageName == "" {
		person.StageName = person.Name
	}

	if err := validation.ValidateAndPopulateAttributeGroups(ctx, s.attributeRepo,
		validation.AttributeGroup{Refs: person.Tags, ExpectedType: model.AttributeTypeTag, Assign: func(v []model.EntityRef) { person.Tags = v }},
		validation.AttributeGroup{Refs: person.Categories, ExpectedType: model.AttributeTypeCategory, Assign: func(v []model.EntityRef) { person.Categories = v }},
		validation.AttributeGroup{Refs: person.Specialties, ExpectedType: model.AttributeTypeSpeciality, Assign: func(v []model.EntityRef) { person.Specialties = v }},
	); err != nil {
		return model.Person{}, err
	}

	person.Stats = model.Stats{
		TotalProductions: 0,
		TotalViews:       0,
		SubscriberCount:  0,
		FollowersCount:   0,
		AverageRating:    0.0,
	}

	now := time.Now()
	person.Audit = model.Audit{
		CreatedAt: now,
		Version:   1,
	}

	var uploadedKeys []string

	if profileInput != nil {
		profileKey, err := s.uploadFileToStorage(ctx, person.ID, "profile", profileInput)
		if err != nil {
			s.cleanupUploadedKeys(ctx, uploadedKeys)
			return model.Person{}, err
		}
		person.ProfilePath = profileKey
		uploadedKeys = append(uploadedKeys, profileKey)
	}

	if backdropInput != nil {
		backdropKey, err := s.uploadFileToStorage(ctx, person.ID, "cover", backdropInput)
		if err != nil {
			s.cleanupUploadedKeys(ctx, uploadedKeys)
			return model.Person{}, err
		}
		person.BackdropPath = backdropKey
		uploadedKeys = append(uploadedKeys, backdropKey)
	}

	if err := s.repo.Create(ctx, person); err != nil {
		s.cleanupUploadedKeys(ctx, uploadedKeys)
		return model.Person{}, err
	}

	return person, nil
}

func (s *PersonService) cleanupUploadedKeys(ctx context.Context, keys []string) {
	cleanupUploadedKeys(ctx, s.fileUploader, keys)
}

func (s *PersonService) uploadFileToStorage(ctx context.Context, personID, purpose string, input *model.FileUploadInput) (string, error) {
	return uploadInputToStorage(ctx, s.fileUploader, "person", personID, purpose, input)
}

func (s *PersonService) Delete(ctx context.Context, id string) error {
	return s.repo.Delete(ctx, id)
}

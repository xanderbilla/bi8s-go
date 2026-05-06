package service

import (
	"context"
	"time"

	goredis "github.com/redis/go-redis/v9"

	"github.com/xanderbilla/bi8s-go/internal/errs"
	"github.com/xanderbilla/bi8s-go/internal/logger"
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
	redisClient   *goredis.Client
	searchService *SearchService
}

const personRedisTTL = 60 * time.Second

func NewPersonService(repo repository.PersonRepository, attributeRepo repository.AttributeRepository, fileUploader storage.FileUploader) *PersonService {
	return &PersonService{
		repo:          repo,
		attributeRepo: attributeRepo,
		fileUploader:  fileUploader,
		searchService: NewSearchService(nil, false),
	}
}

func (s *PersonService) SetSearchService(searchService *SearchService) {
	if searchService == nil {
		s.searchService = NewSearchService(nil, false)
		return
	}
	s.searchService = searchService
}

func (s *PersonService) SetRedisClient(client *goredis.Client) {
	s.redisClient = client
}

func (s *PersonService) GetAll(ctx context.Context) ([]model.Person, error) {
	return s.repo.GetAll(ctx)
}

func (s *PersonService) Get(ctx context.Context, id string) (*model.Person, error) {
	if p, ok := cacheGetJSON[model.Person](ctx, s.redisClient, personCacheKey(id)); ok {
		return p, nil
	}

	p, err := s.repo.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	if p == nil {
		return nil, errs.ErrContentNotFound
	}

	cacheSetJSON(ctx, s.redisClient, personCacheKey(p.ID), p, personRedisTTL, "person", "personId", p.ID)
	return p, nil
}

func (s *PersonService) Create(ctx context.Context, person model.Person, profileInput, backdropInput *model.FileUploadInput) (model.Person, error) {

	if person.ID == "" {
		person.ID = utils.GenerateID()
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
	cacheSetJSON(ctx, s.redisClient, personCacheKey(person.ID), &person, personRedisTTL, "person", "personId", person.ID)
	if err := s.searchService.IndexPerson(ctx, person); err != nil {
		logger.WarnContext(ctx, "search person indexing failed", "personId", person.ID, "error", err.Error())
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
	if err := s.repo.Delete(ctx, id); err != nil {
		return err
	}
	cacheDel(ctx, s.redisClient, personCacheKey(id), "person", "personId", id)
	if err := s.searchService.DeletePerson(ctx, id); err != nil {
		logger.WarnContext(ctx, "search person delete indexing failed", "personId", id, "error", err.Error())
	}
	return nil
}

func personCacheKey(id string) string {
	return "bi8s:person:" + id
}

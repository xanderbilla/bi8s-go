package service

import (
	"context"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
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

const personRedisTTL = 10 * time.Minute

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

func (s *PersonService) GetAll(ctx context.Context, limit int32, startKey map[string]types.AttributeValue) ([]model.Person, map[string]types.AttributeValue, error) {
	return s.repo.GetAll(ctx, limit, startKey)
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

	if err := s.validateAndPopulateSocialPresence(ctx, person.SocialPresence); err != nil {
		return model.Person{}, err
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

func (s *PersonService) UpdateCore(ctx context.Context, id string, patch model.Person) (*model.Person, error) {
	existing, err := s.repo.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	if existing == nil {
		return nil, errs.NewNotFound("person")
	}

	existing.Name = patch.Name
	existing.LegalName = patch.LegalName
	existing.Roles = patch.Roles
	existing.StageName = patch.StageName
	existing.Bio = patch.Bio
	existing.BirthDate = patch.BirthDate
	existing.BirthPlace = patch.BirthPlace
	existing.Nationality = patch.Nationality
	existing.Gender = patch.Gender
	existing.Height = patch.Height
	existing.Weight = patch.Weight
	existing.Verified = patch.Verified
	existing.Active = patch.Active
	existing.DebutYear = patch.DebutYear
	existing.CareerStatus = patch.CareerStatus
	existing.Aliases = patch.Aliases
	existing.Measurements = patch.Measurements
	existing.Career = patch.Career
	existing.SourceMetadata = patch.SourceMetadata

	if err := s.repo.Update(ctx, *existing); err != nil {
		return nil, err
	}
	cacheSetJSON(ctx, s.redisClient, personCacheKey(existing.ID), existing, personRedisTTL, "person", "personId", existing.ID)
	if err := s.searchService.IndexPerson(ctx, *existing); err != nil {
		logger.WarnContext(ctx, "search person indexing failed after person update", "personId", existing.ID, "error", err.Error())
	}
	return existing, nil
}

func (s *PersonService) UpdateProfileImage(ctx context.Context, id string, profileInput *model.FileUploadInput) (*model.Person, error) {
	return s.updatePersonImage(ctx, id, profileInput, "profile")
}

func (s *PersonService) UpdateBackdropImage(ctx context.Context, id string, backdropInput *model.FileUploadInput) (*model.Person, error) {
	return s.updatePersonImage(ctx, id, backdropInput, "backdrop")
}

func (s *PersonService) updatePersonImage(ctx context.Context, id string, input *model.FileUploadInput, purpose string) (*model.Person, error) {
	if input == nil {
		return nil, errs.NewBadRequest("image file is required")
	}

	existing, err := s.repo.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	if existing == nil {
		return nil, errs.NewNotFound("person")
	}

	var oldKey string
	if purpose == "profile" {
		oldKey = strings.TrimSpace(existing.ProfilePath)
	} else {
		oldKey = strings.TrimSpace(existing.BackdropPath)
	}
	if oldKey != "" {
		_ = s.fileUploader.Delete(ctx, strings.TrimPrefix(oldKey, "/"))
	}

	newKey, err := s.uploadFileToStorage(ctx, existing.ID, purpose, input)
	if err != nil {
		return nil, err
	}

	if purpose == "profile" {
		existing.ProfilePath = newKey
	} else {
		existing.BackdropPath = newKey
	}

	if err := s.repo.Update(ctx, *existing); err != nil {
		_ = s.fileUploader.Delete(ctx, newKey)
		return nil, err
	}
	cacheSetJSON(ctx, s.redisClient, personCacheKey(existing.ID), existing, personRedisTTL, "person", "personId", existing.ID)
	if err := s.searchService.IndexPerson(ctx, *existing); err != nil {
		logger.WarnContext(ctx, "search person indexing failed after image update", "personId", existing.ID, "error", err.Error())
	}
	return existing, nil
}

func (s *PersonService) AddAttribute(ctx context.Context, personID string, attributeID string) (*model.Person, error) {
	return s.mutateAttribute(ctx, personID, attributeID, true)
}

func (s *PersonService) RemoveAttribute(ctx context.Context, personID string, attributeID string) (*model.Person, error) {
	return s.mutateAttribute(ctx, personID, attributeID, false)
}

func (s *PersonService) mutateAttribute(ctx context.Context, personID string, attributeID string, add bool) (*model.Person, error) {
	existing, err := s.repo.Get(ctx, personID)
	if err != nil {
		return nil, err
	}
	if existing == nil {
		return nil, errs.NewNotFound("person")
	}

	attr, err := s.attributeRepo.Get(ctx, attributeID)
	if err != nil || attr == nil {
		return nil, errs.NewNotFound("attribute")
	}

	ref := model.EntityRef{ID: attr.ID, Name: attr.Name}
	handled := false
	for _, t := range attr.AttributeType {
		switch t {
		case model.AttributeTypeTag:
			handled = true
			if add {
				if !containsPersonEntity(existing.Tags, attr.ID) {
					existing.Tags = append(existing.Tags, ref)
				}
			} else {
				existing.Tags = removePersonEntity(existing.Tags, attr.ID)
			}
		case model.AttributeTypeCategory:
			handled = true
			if add {
				if !containsPersonEntity(existing.Categories, attr.ID) {
					existing.Categories = append(existing.Categories, ref)
				}
			} else {
				existing.Categories = removePersonEntity(existing.Categories, attr.ID)
			}
		case model.AttributeTypeSpeciality:
			handled = true
			if add {
				if !containsPersonEntity(existing.Specialties, attr.ID) {
					existing.Specialties = append(existing.Specialties, ref)
				}
			} else {
				existing.Specialties = removePersonEntity(existing.Specialties, attr.ID)
			}
		case model.AttributeTypePlatform, model.AttributeTypeSocial:
			handled = true
			if add {
				found := false
				for _, sp := range existing.SocialPresence {
					if sp.PlatformID == attr.ID {
						found = true
						break
					}
				}
				if !found {
					existing.SocialPresence = append(existing.SocialPresence, model.SocialPresenceEntry{PlatformID: attr.ID, Platform: strings.ToLower(attr.Name), Available: true})
				}
			} else {
				filtered := make([]model.SocialPresenceEntry, 0, len(existing.SocialPresence))
				for _, sp := range existing.SocialPresence {
					if sp.PlatformID != attr.ID {
						filtered = append(filtered, sp)
					}
				}
				existing.SocialPresence = filtered
			}
		}
	}

	if !handled {
		return nil, errs.NewBadRequest("attribute type is not assignable to person")
	}

	if err := s.repo.Update(ctx, *existing); err != nil {
		return nil, err
	}
	cacheSetJSON(ctx, s.redisClient, personCacheKey(existing.ID), existing, personRedisTTL, "person", "personId", existing.ID)
	if err := s.searchService.IndexPerson(ctx, *existing); err != nil {
		logger.WarnContext(ctx, "search person indexing failed after attribute mutation", "personId", existing.ID, "error", err.Error())
	}
	return existing, nil
}

func containsPersonEntity(items []model.EntityRef, id string) bool {
	for _, it := range items {
		if it.ID == id {
			return true
		}
	}
	return false
}

func removePersonEntity(items []model.EntityRef, id string) []model.EntityRef {
	filtered := make([]model.EntityRef, 0, len(items))
	for _, it := range items {
		if it.ID != id {
			filtered = append(filtered, it)
		}
	}
	return filtered
}

func (s *PersonService) Delete(ctx context.Context, id string) error {
	if err := s.repo.Delete(ctx, id); err != nil {
		if errs.IsConditionalCheckFailed(err) {
			// Record no longer in DB — still purge any orphaned search document.
			if serr := s.searchService.DeletePerson(ctx, id); serr != nil {
				logger.WarnContext(ctx, "search person delete indexing failed", "personId", id, "error", serr.Error())
			}
			return errs.NewNotFound("person")
		}
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

// validateAndPopulateSocialPresence ensures each social presence entry references a valid
// PLATFORM attribute and populates the Platform name field from the attribute record.
func (s *PersonService) validateAndPopulateSocialPresence(ctx context.Context, entries []model.SocialPresenceEntry) error {
	for i := range entries {
		attr, err := s.attributeRepo.Get(ctx, entries[i].PlatformID)
		if err != nil || attr == nil {
			return errs.NewNotFound("attribute not found: " + entries[i].PlatformID)
		}
		hasPlatformType := false
		for _, t := range attr.AttributeType {
			if t == model.AttributeTypePlatform || t == model.AttributeTypeSocial {
				hasPlatformType = true
				break
			}
		}
		if !hasPlatformType {
			return errs.NewBadRequest("attribute " + entries[i].PlatformID + " is not of type PLATFORM or SOCIAL")
		}
		entries[i].Platform = strings.ToLower(attr.Name)
	}
	return nil
}

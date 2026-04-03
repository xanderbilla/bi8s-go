package http

import (
	"errors"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/xanderbilla/bi8s-go/internal/errs"
	"github.com/xanderbilla/bi8s-go/internal/model"
	"github.com/xanderbilla/bi8s-go/internal/service"
)

// PersonHandler handles person-related HTTP routes.
type PersonHandler struct {
	personService *service.PersonService
}

// NewPersonHandler creates a new PersonHandler.
func NewPersonHandler(personService *service.PersonService) *PersonHandler {
	return &PersonHandler{
		personService: personService,
	}
}

// GetAllPeople handles GET /v1/people and GET /v1/persons and returns all people.
func (h *PersonHandler) GetAllPeople(w http.ResponseWriter, r *http.Request) {
	persons, err := h.personService.GetAll(r.Context())
	if err != nil {
		errs.InternalServerError(w, r, err)
		return
	}

	Success(w, http.StatusOK, "people fetched", persons)
}

// GetPerson handles GET /v1/a/people/{peopleId} and GET /v1/c/people/{peopleId}.
func (h *PersonHandler) GetPerson(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "peopleId")

	person, err := h.personService.Get(r.Context(), id)
	if err != nil {
		errs.InternalServerError(w, r, err)
		return
	}

	if person == nil {
		errs.NotFoundError(w, r, errors.New("person not found"))
		return
	}

	// Convert to public detail format (exclude stats and audit)
	publicPerson := model.PersonPublicDetail{
		ID:           person.ID,
		ContentType:  person.ContentType,
		Name:         person.Name,
		Roles:        person.Roles,
		StageName:    person.StageName,
		Bio:          person.Bio,
		BirthDate:    person.BirthDate,
		BirthPlace:   person.BirthPlace,
		Nationality:  person.Nationality,
		Gender:       person.Gender,
		Height:       person.Height,
		Verified:     person.Verified,
		Active:       person.Active,
		DebutYear:    person.DebutYear,
		CareerStatus: person.CareerStatus,
		ProfilePath:  person.ProfilePath,
		BackdropPath: person.BackdropPath,
		Measurements: person.Measurements,
		Tags:         person.Tags,
		Categories:   person.Categories,
		Specialties:  person.Specialties,
	}

	Success(w, http.StatusOK, "person fetched", publicPerson)
}

// CreatePerson handles POST /v1/persons.
func (h *PersonHandler) CreatePerson(w http.ResponseWriter, r *http.Request) {
	formValues, err := ParseMultipartForm(r)
	if err != nil {
		errs.BadRequestError(w, r, err)
		return
	}

	person, err := ParsePersonFromForm(formValues)
	if err != nil {
		errs.BadRequestError(w, r, err)
		return
	}

	profileInput, err := ExtractFile(r, "profile")
	if err != nil {
		errs.BadRequestError(w, r, err)
		return
	}

	backdropInput, err := ExtractFile(r, "backdrop")
	if err != nil {
		errs.BadRequestError(w, r, err)
		return
	}

	newPerson, err := h.personService.Create(r.Context(), person, profileInput, backdropInput)
	if err != nil {
		if errors.Is(err, errs.ErrFileUploaderNotConfigured) {
			errs.InternalServerError(w, r, err)
			return
		}

		// Check if it's an attribute validation error
		if strings.Contains(err.Error(), "attribute with id") {
			errs.BadRequestError(w, r, err)
			return
		}

		if isConditionalCheckFailed(err) {
			errs.ConflictError(w, r, err)
			return
		}

		errs.InternalServerError(w, r, err)
		return
	}

	Success(w, http.StatusCreated, "person created", newPerson)
}

// DeletePerson handles DELETE /v1/a/people/{peopleId}.
func (h *PersonHandler) DeletePerson(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "peopleId")

	if err := h.personService.Delete(r.Context(), id); err != nil {
		if isConditionalCheckFailed(err) {
			errs.NotFoundError(w, r, err)
			return
		}

		errs.InternalServerError(w, r, err)
		return
	}

	Success(w, http.StatusOK, "person deleted", nil)
}

package http

import (
	"net/http"
	"strings"

	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/go-chi/chi/v5"
	"github.com/xanderbilla/bi8s-go/internal/errs"
	"github.com/xanderbilla/bi8s-go/internal/model"
	"github.com/xanderbilla/bi8s-go/internal/repository"
	"github.com/xanderbilla/bi8s-go/internal/response"
	"github.com/xanderbilla/bi8s-go/internal/service"
)

type PersonHandler struct {
	personService *service.PersonService
}

type personUpdateRequest struct {
	Name         string                      `json:"name"`
	LegalName    string                      `json:"legalName"`
	Roles        []model.EntityType          `json:"roles"`
	StageName    string                      `json:"stageName"`
	Bio          string                      `json:"bio"`
	BirthDate    string                      `json:"birthDate"`
	BirthPlace   string                      `json:"birthPlace"`
	Nationality  string                      `json:"nationality"`
	Gender       model.Gender                `json:"gender"`
	Height       int                         `json:"height"`
	Weight       int                         `json:"weight"`
	Verified     bool                        `json:"verified"`
	Active       bool                        `json:"active"`
	DebutYear    int                         `json:"debutYear"`
	CareerStatus model.CareerStatus          `json:"careerStatus"`
	Aliases      []string                    `json:"aliases"`
	Measurements model.Measurements          `json:"measurements"`
	Career       *model.PersonCareer         `json:"career"`
	SourceMeta   *model.PersonSourceMetadata `json:"sourceMetadata"`
}

type personAttributeMutationRequest struct {
	PersonID string `json:"personId"`
}

func NewPersonHandler(personService *service.PersonService) *PersonHandler {
	return &PersonHandler{
		personService: personService,
	}
}

func (h *PersonHandler) GetAllPeople(w http.ResponseWriter, r *http.Request) {
	query := strings.TrimSpace(r.URL.Query().Get("q"))
	sortMode := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("sort")))
	if query != "" || sortMode != "" {
		if sortMode == "" {
			sortMode = "recent"
		}
		if !isAdminSortValid(sortMode) {
			errs.BadRequestError(w, r, errs.NewBadRequest("sort must be one of: recent, latest, alpha_asc, alpha_desc"))
			return
		}

		limit, err := parseLimitParam(r.URL.Query().Get("limit"))
		if err != nil {
			errs.BadRequestError(w, r, err)
			return
		}
		offset, err := parseOffsetCursor(r.URL.Query().Get("cursor"))
		if err != nil {
			errs.BadRequestError(w, r, err)
			return
		}

		items := make([]model.Person, 0)
		var startKey map[string]types.AttributeValue
		for {
			pageItems, nextKey, err := h.personService.GetAll(r.Context(), maxPageLimit, startKey)
			if err != nil {
				errs.Write(w, r, err)
				return
			}
			items = append(items, pageItems...)
			if len(nextKey) == 0 {
				break
			}
			startKey = nextKey
		}

		items = filterAdminPeople(items, query)
		sortAdminPeople(items, sortMode)
		page, next := paginatePeople(items, offset, int(limit))

		writeOK(w, r, http.StatusOK, "people fetched", response.PagedData[model.Person]{
			Items:      page,
			NextCursor: next,
			Count:      len(page),
		})
		return
	}

	limit, startKey, err := parsePaginationParams(r)
	if err != nil {
		errs.BadRequestError(w, r, err)
		return
	}
	persons, nextKey, err := h.personService.GetAll(r.Context(), limit, startKey)
	if err != nil {
		errs.Write(w, r, err)
		return
	}
	cursor, _ := repository.EncodeCursor(nextKey)
	writeOK(w, r, http.StatusOK, "people fetched", response.PagedData[model.Person]{
		Items:      persons,
		NextCursor: cursor,
		Count:      len(persons),
	})
}

func (h *PersonHandler) GetPerson(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "peopleId")

	person, err := h.personService.Get(r.Context(), id)
	if err != nil {
		errs.Write(w, r, err)
		return
	}

	writeOK(w, r, http.StatusOK, "person fetched", toPersonPublicDetail(person))
}

func (h *PersonHandler) GetPersonAdmin(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "peopleId")

	person, err := h.personService.Get(r.Context(), id)
	if err != nil {
		errs.Write(w, r, err)
		return
	}

	writeOK(w, r, http.StatusOK, "person fetched", person)
}

func (h *PersonHandler) CreatePerson(w http.ResponseWriter, r *http.Request) {
	formValues, files, err := ParseFormAndFiles(w, r, []string{"profile", "backdrop"})
	if err != nil {
		errs.BadRequestError(w, r, err)
		return
	}

	person, err := ParsePersonFromForm(formValues)
	if err != nil {
		errs.BadRequestError(w, r, err)
		return
	}

	newPerson, err := h.personService.Create(r.Context(), person, files["profile"], files["backdrop"])
	if err != nil {
		errs.Write(w, r, err)
		return
	}

	writeOK(w, r, http.StatusCreated, "person created", newPerson)
}

func (h *PersonHandler) DeletePerson(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "peopleId")

	if err := h.personService.Delete(r.Context(), id); err != nil {
		if errs.IsConditionalCheckFailed(err) {
			errs.NotFoundError(w, r, err)
			return
		}
		errs.Write(w, r, err)
		return
	}

	writeOK(w, r, http.StatusOK, "person deleted", nil)
}

func (h *PersonHandler) UpdatePerson(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "peopleId")

	var req personUpdateRequest
	if err := Decode(w, r, &req); err != nil {
		errs.BadRequestError(w, r, errs.NewBadRequest(err.Error()))
		return
	}

	patch := model.Person{
		Name:           strings.TrimSpace(req.Name),
		LegalName:      strings.TrimSpace(req.LegalName),
		Roles:          req.Roles,
		StageName:      strings.TrimSpace(req.StageName),
		Bio:            clampMaxChars(req.Bio, 150),
		BirthDate:      strings.TrimSpace(req.BirthDate),
		BirthPlace:     strings.TrimSpace(req.BirthPlace),
		Nationality:    strings.TrimSpace(req.Nationality),
		Gender:         req.Gender,
		Height:         req.Height,
		Weight:         req.Weight,
		Verified:       req.Verified,
		Active:         req.Active,
		DebutYear:      req.DebutYear,
		CareerStatus:   req.CareerStatus,
		Aliases:        req.Aliases,
		Measurements:   req.Measurements,
		Career:         req.Career,
		SourceMetadata: req.SourceMeta,
	}

	updated, err := h.personService.UpdateCore(r.Context(), id, patch)
	if err != nil {
		errs.Write(w, r, err)
		return
	}

	writeOK(w, r, http.StatusOK, "person updated", updated)
}

func (h *PersonHandler) UpdatePersonProfile(w http.ResponseWriter, r *http.Request) {
	h.updatePersonImageByPurpose(w, r, "profile")
}

func (h *PersonHandler) UpdatePersonBackdrop(w http.ResponseWriter, r *http.Request) {
	h.updatePersonImageByPurpose(w, r, "backdrop")
}

func (h *PersonHandler) updatePersonImageByPurpose(w http.ResponseWriter, r *http.Request, purpose string) {
	id := chi.URLParam(r, "peopleId")

	if _, err := ParseMultipartForm(r, w); err != nil {
		errs.BadRequestError(w, r, err)
		return
	}

	input, err := ExtractFile(r, purpose)
	if err != nil {
		errs.BadRequestError(w, r, err)
		return
	}
	if input == nil {
		input, err = ExtractFile(r, "image")
		if err != nil {
			errs.BadRequestError(w, r, err)
			return
		}
	}
	if input == nil {
		errs.BadRequestError(w, r, errs.NewBadRequest("image file is required"))
		return
	}

	var updated *model.Person
	if purpose == "profile" {
		updated, err = h.personService.UpdateProfileImage(r.Context(), id, input)
	} else {
		updated, err = h.personService.UpdateBackdropImage(r.Context(), id, input)
	}
	if err != nil {
		errs.Write(w, r, err)
		return
	}

	writeOK(w, r, http.StatusOK, "person image updated", updated)
}

func (h *PersonHandler) AddPersonAttribute(w http.ResponseWriter, r *http.Request) {
	h.mutatePersonAttribute(w, r, true)
}

func (h *PersonHandler) RemovePersonAttribute(w http.ResponseWriter, r *http.Request) {
	h.mutatePersonAttribute(w, r, false)
}

func (h *PersonHandler) mutatePersonAttribute(w http.ResponseWriter, r *http.Request, add bool) {
	attributeID := chi.URLParam(r, "attributeId")

	var req personAttributeMutationRequest
	if err := Decode(w, r, &req); err != nil {
		errs.BadRequestError(w, r, errs.NewBadRequest(err.Error()))
		return
	}
	if strings.TrimSpace(req.PersonID) == "" {
		errs.BadRequestError(w, r, errs.NewBadRequest("personId is required"))
		return
	}

	var (
		updated *model.Person
		err     error
	)
	if add {
		updated, err = h.personService.AddAttribute(r.Context(), req.PersonID, attributeID)
	} else {
		updated, err = h.personService.RemoveAttribute(r.Context(), req.PersonID, attributeID)
	}
	if err != nil {
		errs.Write(w, r, err)
		return
	}

	msg := "person attribute removed"
	if add {
		msg = "person attribute added"
	}
	writeOK(w, r, http.StatusOK, msg, updated)
}

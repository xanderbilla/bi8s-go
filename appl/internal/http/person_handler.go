package http

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/xanderbilla/bi8s-go/internal/errs"
	"github.com/xanderbilla/bi8s-go/internal/logger"
	"github.com/xanderbilla/bi8s-go/internal/response"
	"github.com/xanderbilla/bi8s-go/internal/service"
)

type PersonHandler struct {
	personService *service.PersonService
}

func NewPersonHandler(personService *service.PersonService) *PersonHandler {
	return &PersonHandler{
		personService: personService,
	}
}

func (h *PersonHandler) GetAllPeople(w http.ResponseWriter, r *http.Request) {
	persons, err := h.personService.GetAll(r.Context())
	if err != nil {
		errs.Write(w, r, err)
		return
	}

	if err := response.Success(w, r, http.StatusOK, "people fetched", persons); err != nil {
		logger.ErrorContext(r.Context(), "failed to write response", "error", err)
	}
}

func (h *PersonHandler) GetPerson(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "peopleId")

	person, err := h.personService.Get(r.Context(), id)
	if err != nil {
		errs.Write(w, r, err)
		return
	}

	if err := response.Success(w, r, http.StatusOK, "person fetched", toPersonPublicDetail(person)); err != nil {
		logger.ErrorContext(r.Context(), "failed to write response", "error", err)
	}
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

	if err := response.Success(w, r, http.StatusCreated, "person created", newPerson); err != nil {
		logger.ErrorContext(r.Context(), "failed to write response", "error", err)
	}
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

	if err := response.Success(w, r, http.StatusOK, "person deleted", nil); err != nil {
		logger.ErrorContext(r.Context(), "failed to write response", "error", err)
	}
}

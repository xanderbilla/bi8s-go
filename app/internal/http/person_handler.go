package http

import (
	"net/http"

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

func NewPersonHandler(personService *service.PersonService) *PersonHandler {
	return &PersonHandler{
		personService: personService,
	}
}

func (h *PersonHandler) GetAllPeople(w http.ResponseWriter, r *http.Request) {
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

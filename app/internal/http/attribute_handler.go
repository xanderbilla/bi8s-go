package http

import (
	"errors"
	"net/http"
	"sort"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/xanderbilla/bi8s-go/internal/errs"
	"github.com/xanderbilla/bi8s-go/internal/model"
	"github.com/xanderbilla/bi8s-go/internal/search"
	"github.com/xanderbilla/bi8s-go/internal/service"
)

type AttributeHandler struct {
	attributeService *service.AttributeService
}

func NewAttributeHandler(attributeService *service.AttributeService) *AttributeHandler {
	return &AttributeHandler{
		attributeService: attributeService,
	}
}

func (h *AttributeHandler) GetAllAttributes(w http.ResponseWriter, r *http.Request) {
	attributes, err := h.attributeService.GetAll(r.Context())
	if err != nil {
		errs.Write(w, r, err)
		return
	}

	publicAttributes := make([]model.AttributePublicDetail, len(attributes))
	for i, attr := range attributes {
		publicAttributes[i] = toAttributePublicDetail(&attr)
	}

	writeOK(w, r, http.StatusOK, "attributes fetched", publicAttributes)
}

func (h *AttributeHandler) GetConsumerAttributes(w http.ResponseWriter, r *http.Request) {
	attributes, err := h.attributeService.GetAll(r.Context())
	if err != nil {
		errs.Write(w, r, err)
		return
	}

	typeFilter, ok := parseAttributeTypeFilter(r.URL.Query().Get("type"))
	if !ok {
		errs.Write(w, r, errs.NewBadRequest("type must be one of: GENRE, TAG, MOOD, STUDIO, CATEGORY, SPECIALITY"))
		return
	}
	sortMode, err := parseAlphaSort(r.URL.Query().Get("sort"))
	if err != nil {
		errs.Write(w, r, errs.NewBadRequest(err.Error()))
		return
	}

	publicAttributes := make([]model.AttributePublicDetail, 0, len(attributes))
	for _, attr := range attributes {
		if !attr.Active {
			continue
		}
		if typeFilter != "" && !attributeHasType(attr, typeFilter) {
			continue
		}
		publicAttributes = append(publicAttributes, toAttributePublicDetail(&attr))
	}
	if sortMode != "" {
		sortAttributeDetails(publicAttributes, sortMode == search.SortAlphaDesc)
	}

	writeOK(w, r, http.StatusOK, "attributes fetched", publicAttributes)
}

func parseAlphaSort(raw string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "":
		return "", nil
	case search.SortAlphaAsc:
		return search.SortAlphaAsc, nil
	case search.SortAlphaDesc:
		return search.SortAlphaDesc, nil
	default:
		return "", errors.New("sort must be one of: alpha_asc, alpha_desc")
	}
}

func sortAttributeDetails(items []model.AttributePublicDetail, descending bool) {
	sort.SliceStable(items, func(i, j int) bool {
		left := strings.ToLower(strings.TrimSpace(items[i].Name))
		right := strings.ToLower(strings.TrimSpace(items[j].Name))
		if left == right {
			if descending {
				return items[i].ID > items[j].ID
			}
			return items[i].ID < items[j].ID
		}
		if descending {
			return left > right
		}
		return left < right
	})
}

func attributeHasType(attr model.Attribute, filter model.AttributeType) bool {
	for _, t := range attr.AttributeType {
		if t == filter {
			return true
		}
	}
	return false
}

func parseAttributeTypeFilter(raw string) (model.AttributeType, bool) {
	s := strings.ToUpper(strings.TrimSpace(raw))
	if s == "" {
		return "", true
	}
	s = strings.TrimSuffix(s, "S")
	if s == "CATEGORIE" {
		s = "CATEGORY"
	}
	if s == "SPECIALITIE" {
		s = "SPECIALITY"
	}

	switch s {
	case string(model.AttributeTypeGenre):
		return model.AttributeTypeGenre, true
	case string(model.AttributeTypeTag):
		return model.AttributeTypeTag, true
	case string(model.AttributeTypeMood):
		return model.AttributeTypeMood, true
	case string(model.AttributeTypeStudio):
		return model.AttributeTypeStudio, true
	case string(model.AttributeTypeCategory):
		return model.AttributeTypeCategory, true
	case string(model.AttributeTypeSpeciality):
		return model.AttributeTypeSpeciality, true
	default:
		return "", false
	}
}

func (h *AttributeHandler) GetAttribute(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "attributeId")

	attribute, err := h.attributeService.Get(r.Context(), id)
	if err != nil {
		errs.Write(w, r, err)
		return
	}

	writeOK(w, r, http.StatusOK, "attribute fetched", toAttributePublicDetail(attribute))
}

func (h *AttributeHandler) CreateAttribute(w http.ResponseWriter, r *http.Request) {
	formValues, err := ParseMultipartForm(r, w)
	if err != nil {
		errs.BadRequestError(w, r, err)
		return
	}

	attribute, err := ParseAttributeFromForm(formValues)
	if err != nil {
		errs.BadRequestError(w, r, err)
		return
	}

	newAttribute, err := h.attributeService.Create(r.Context(), attribute)
	if err != nil {
		errs.Write(w, r, err)
		return
	}

	writeOK(w, r, http.StatusCreated, "attribute created", newAttribute)
}

func (h *AttributeHandler) DeleteAttribute(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "attributeId")

	if err := h.attributeService.Delete(r.Context(), id); err != nil {
		if errs.IsConditionalCheckFailed(err) {
			errs.NotFoundError(w, r, err)
			return
		}
		errs.Write(w, r, err)
		return
	}

	writeOK(w, r, http.StatusOK, "attribute deleted", nil)
}

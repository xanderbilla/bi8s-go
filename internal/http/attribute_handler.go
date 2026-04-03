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

// AttributeHandler handles attribute-related HTTP routes.
type AttributeHandler struct {
	attributeService *service.AttributeService
}

// NewAttributeHandler creates a new AttributeHandler.
func NewAttributeHandler(attributeService *service.AttributeService) *AttributeHandler {
	return &AttributeHandler{
		attributeService: attributeService,
	}
}

// GetAllAttributes handles GET /v1/attributes and returns all attributes.
func (h *AttributeHandler) GetAllAttributes(w http.ResponseWriter, r *http.Request) {
	attributes, err := h.attributeService.GetAll(r.Context())
	if err != nil {
		errs.InternalServerError(w, r, err)
		return
	}

	// Convert to public list format (exclude audit)
	publicAttributes := make([]model.AttributePublicDetail, len(attributes))
	for i, attr := range attributes {
		publicAttributes[i] = model.AttributePublicDetail{
			ID:            attr.ID,
			Name:          attr.Name,
			AttributeType: attr.AttributeType,
			ContentType:   attr.ContentType,
			Active:        attr.Active,
		}
	}

	Success(w, http.StatusOK, "attributes fetched", publicAttributes)
}

// GetAttribute handles GET /v1/attributes/{attributeId}.
func (h *AttributeHandler) GetAttribute(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "attributeId")

	attribute, err := h.attributeService.Get(r.Context(), id)
	if err != nil {
		errs.InternalServerError(w, r, err)
		return
	}

	if attribute == nil {
		errs.NotFoundError(w, r, errors.New("attribute not found"))
		return
	}

	// Convert to public detail format (exclude audit)
	publicAttribute := model.AttributePublicDetail{
		ID:            attribute.ID,
		Name:          attribute.Name,
		AttributeType: attribute.AttributeType,
		ContentType:   attribute.ContentType,
		Active:        attribute.Active,
	}

	Success(w, http.StatusOK, "attribute fetched", publicAttribute)
}

// CreateAttribute handles POST /v1/attributes.
func (h *AttributeHandler) CreateAttribute(w http.ResponseWriter, r *http.Request) {
	formValues, err := ParseMultipartForm(r)
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
		// Check if it's a duplicate name error
		if strings.Contains(err.Error(), "attribute with this name already exists") {
			errs.ConflictError(w, r, err)
			return
		}

		if isConditionalCheckFailed(err) {
			errs.ConflictError(w, r, err)
			return
		}

		errs.InternalServerError(w, r, err)
		return
	}

	Success(w, http.StatusCreated, "attribute created", newAttribute)
}

// DeleteAttribute handles DELETE /v1/attributes/{attributeId}.
func (h *AttributeHandler) DeleteAttribute(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "attributeId")

	if err := h.attributeService.Delete(r.Context(), id); err != nil {
		if isConditionalCheckFailed(err) {
			errs.NotFoundError(w, r, err)
			return
		}

		errs.InternalServerError(w, r, err)
		return
	}

	Success(w, http.StatusOK, "attribute deleted", nil)
}

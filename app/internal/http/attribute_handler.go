package http

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/xanderbilla/bi8s-go/internal/errs"
	"github.com/xanderbilla/bi8s-go/internal/logger"
	"github.com/xanderbilla/bi8s-go/internal/model"
	"github.com/xanderbilla/bi8s-go/internal/response"
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

	if err := response.Success(w, r, http.StatusOK, "attributes fetched", publicAttributes); err != nil {
		logger.ErrorContext(r.Context(), "failed to write response", "error", err)
	}
}

func (h *AttributeHandler) GetAttribute(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "attributeId")

	attribute, err := h.attributeService.Get(r.Context(), id)
	if err != nil {
		errs.Write(w, r, err)
		return
	}

	if err := response.Success(w, r, http.StatusOK, "attribute fetched", toAttributePublicDetail(attribute)); err != nil {
		logger.ErrorContext(r.Context(), "failed to write response", "error", err)
	}
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

	if err := response.Success(w, r, http.StatusCreated, "attribute created", newAttribute); err != nil {
		logger.ErrorContext(r.Context(), "failed to write response", "error", err)
	}
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

	if err := response.Success(w, r, http.StatusOK, "attribute deleted", nil); err != nil {
		logger.ErrorContext(r.Context(), "failed to write response", "error", err)
	}
}

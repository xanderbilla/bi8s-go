package http

import (
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/xanderbilla/bi8s-go/internal/errs"
	"github.com/xanderbilla/bi8s-go/internal/model"
	"github.com/xanderbilla/bi8s-go/internal/service"
)

// EncoderHandler handles encoder-related HTTP routes
type EncoderHandler struct {
	encoderService *service.EncoderService
}

// NewEncoderHandler creates a new encoder handler
func NewEncoderHandler(encoderService *service.EncoderService) *EncoderHandler {
	return &EncoderHandler{
		encoderService: encoderService,
	}
}

// CreateEncodingJob handles POST /v1/c/encoder/new
func (h *EncoderHandler) CreateEncodingJob(w http.ResponseWriter, r *http.Request) {
	formValues, err := ParseMultipartFormForVideo(r)
	if err != nil {
		errs.BadRequestError(w, r, err)
		return
	}

	// Get contentId and contentType from form
	contentID := formValues.Get("contentId")
	if contentID == "" {
		errs.BadRequestError(w, r, errors.New("contentId is required"))
		return
	}

	contentTypeStr := formValues.Get("contentType")
	if contentTypeStr == "" {
		errs.BadRequestError(w, r, errors.New("contentType is required"))
		return
	}

	// Validate contentType
	var contentType model.ContentType
	switch contentTypeStr {
	case string(model.ContentTypeMovie):
		contentType = model.ContentTypeMovie
	case string(model.ContentTypeTV):
		contentType = model.ContentTypeTV
	default:
		errs.BadRequestError(w, r, errors.New("contentType must be MOVIE or TV"))
		return
	}

	// Extract video file (up to 10GB)
	videoInput, err := ExtractVideoFile(r, "video")
	if err != nil {
		errs.BadRequestError(w, r, err)
		return
	}

	// Create encoding job
	job, err := h.encoderService.CreateEncodingJob(r.Context(), contentID, contentType, videoInput)
	if err != nil {
		if errors.Is(err, errs.ErrFileUploaderNotConfigured) {
			errs.InternalServerError(w, r, err)
			return
		}
		errs.InternalServerError(w, r, err)
		return
	}

	Success(w, http.StatusCreated, "encoding job created", job)
}

// GetEncodingJob handles GET /v1/c/encoder/{jobId}
func (h *EncoderHandler) GetEncodingJob(w http.ResponseWriter, r *http.Request) {
	jobID := chi.URLParam(r, "jobId")

	job, err := h.encoderService.GetEncodingJob(r.Context(), jobID)
	if err != nil {
		errs.InternalServerError(w, r, err)
		return
	}

	if job == nil {
		errs.NotFoundError(w, r, errors.New("encoding job not found"))
		return
	}

	Success(w, http.StatusOK, "encoding job fetched", job)
}

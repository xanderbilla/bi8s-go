package http

import (
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/xanderbilla/bi8s-go/internal/errs"
	"github.com/xanderbilla/bi8s-go/internal/model"
	"github.com/xanderbilla/bi8s-go/internal/response"
	"github.com/xanderbilla/bi8s-go/internal/service"
	"github.com/xanderbilla/bi8s-go/internal/validation"
)

type EncoderHandler struct {
	encoderService *service.EncoderService
}

func NewEncoderHandler(encoderService *service.EncoderService) *EncoderHandler {
	return &EncoderHandler{
		encoderService: encoderService,
	}
}

func (h *EncoderHandler) CreateEncodingJob(w http.ResponseWriter, r *http.Request) {

	if err := ParseVideoMultipartForm(r, w); err != nil {
		errs.BadRequestError(w, r, err)
		return
	}

	contentID := r.FormValue("contentId")
	if contentID == "" {
		errs.BadRequestError(w, r, errors.New("contentId is required"))
		return
	}

	contentTypeStr := r.FormValue("contentType")
	if contentTypeStr == "" {
		errs.BadRequestError(w, r, errors.New("contentType is required"))
		return
	}

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

	videoInput, err := validation.ExtractFileToTemp(r, "video", maxVideoFileSize)
	if err != nil {
		errs.BadRequestError(w, r, err)
		return
	}
	if videoInput == nil {
		errs.BadRequestError(w, r, errors.New("video file is required"))
		return
	}

	job, err := h.encoderService.CreateEncodingJob(r.Context(), contentID, contentType, videoInput)
	if err != nil {
		errs.Write(w, r, err)
		return
	}

	response.Success(w, r, http.StatusAccepted, "encoding job queued", job)
}

func (h *EncoderHandler) GetEncodingJob(w http.ResponseWriter, r *http.Request) {
	jobID := chi.URLParam(r, "jobId")

	job, err := h.encoderService.GetEncodingJob(r.Context(), jobID)
	if err != nil {
		errs.Write(w, r, err)
		return
	}

	response.Success(w, r, http.StatusOK, "encoding job fetched", job)
}

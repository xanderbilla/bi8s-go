package http

import (
	"errors"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/xanderbilla/bi8s-go/internal/errs"
	"github.com/xanderbilla/bi8s-go/internal/logger"
	"github.com/xanderbilla/bi8s-go/internal/model"
	"github.com/xanderbilla/bi8s-go/internal/response"
	"github.com/xanderbilla/bi8s-go/internal/service"
)

type ContentHandler struct {
	movieService *service.MovieService
}

func NewContentHandler(svc *service.MovieService) *ContentHandler {
	return &ContentHandler{movieService: svc}
}

func contentTypeFromQuery(r *http.Request, key string) string {
	switch strings.ToLower(strings.TrimSpace(r.URL.Query().Get(key))) {
	case "movie":
		return "movie"
	case "tv":
		return "tv"
	default:
		return "all"
	}
}

func (h *ContentHandler) GetAllContentAdmin(w http.ResponseWriter, r *http.Request) {
	movies, err := h.movieService.GetAllAdmin(r.Context())
	if err != nil {
		errs.Write(w, r, err)
		return
	}
	if err := response.Success(w, r, http.StatusOK, "movies fetched", movies); err != nil {
		logger.ErrorContext(r.Context(), "failed to write response", "error", err)
	}
}

func (h *ContentHandler) GetRecentContent(w http.ResponseWriter, r *http.Request) {
	movies, err := h.movieService.GetRecentContent(r.Context(), contentTypeFromQuery(r, "type"))
	if err != nil {
		errs.Write(w, r, err)
		return
	}
	if err := response.Success(w, r, http.StatusOK, "content fetched", convertToPublicList(movies)); err != nil {
		logger.ErrorContext(r.Context(), "failed to write response", "error", err)
	}
}

func (h *ContentHandler) GetContent(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "contentId")

	movie, err := h.movieService.Get(r.Context(), id)
	if err != nil {
		errs.Write(w, r, err)
		return
	}
	if err := response.Success(w, r, http.StatusOK, "movie fetched", toMoviePublicDetail(movie)); err != nil {
		logger.ErrorContext(r.Context(), "failed to write response", "error", err)
	}
}

func (h *ContentHandler) GetContentAdmin(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "contentId")

	movie, err := h.movieService.GetAdmin(r.Context(), id)
	if err != nil {
		errs.Write(w, r, err)
		return
	}
	if err := response.Success(w, r, http.StatusOK, "movie fetched", movie); err != nil {
		logger.ErrorContext(r.Context(), "failed to write response", "error", err)
	}
}

func (h *ContentHandler) CreateContent(w http.ResponseWriter, r *http.Request) {
	formValues, files, err := ParseFormAndFiles(w, r, []string{"poster", "cover"})
	if err != nil {
		errs.BadRequestError(w, r, err)
		return
	}

	movie, err := ParseMovieFromForm(formValues)
	if err != nil {
		errs.BadRequestError(w, r, err)
		return
	}

	newMovie, err := h.movieService.Create(r.Context(), movie, files["poster"], files["cover"])
	if err != nil {
		errs.Write(w, r, err)
		return
	}

	if err := response.Success(w, r, http.StatusCreated, "movie created", newMovie); err != nil {
		logger.ErrorContext(r.Context(), "failed to write response", "error", err)
	}
}

func (h *ContentHandler) DeleteContent(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "contentId")

	if err := h.movieService.Delete(r.Context(), id); err != nil {
		if errs.IsConditionalCheckFailed(err) {
			errs.NotFoundError(w, r, err)
			return
		}
		errs.Write(w, r, err)
		return
	}
	if err := response.Success(w, r, http.StatusOK, "movie deleted", nil); err != nil {
		logger.ErrorContext(r.Context(), "failed to write response", "error", err)
	}
}

func (h *ContentHandler) GetContentByPersonIdAdmin(w http.ResponseWriter, r *http.Request) {
	movies, err := h.movieService.GetContentByPersonIdAdmin(
		r.Context(),
		chi.URLParam(r, "peopleId"),
		contentTypeFromQuery(r, "type"),
	)
	if err != nil {
		errs.Write(w, r, err)
		return
	}
	if err := response.Success(w, r, http.StatusOK, "content fetched", convertToMinimalList(movies)); err != nil {
		logger.ErrorContext(r.Context(), "failed to write response", "error", err)
	}
}

func (h *ContentHandler) GetContentByPersonId(w http.ResponseWriter, r *http.Request) {
	movies, err := h.movieService.GetContentByPersonId(
		r.Context(),
		chi.URLParam(r, "peopleId"),
		contentTypeFromQuery(r, "type"),
	)
	if err != nil {
		errs.Write(w, r, err)
		return
	}
	if err := response.Success(w, r, http.StatusOK, "content fetched", convertToMinimalList(movies)); err != nil {
		logger.ErrorContext(r.Context(), "failed to write response", "error", err)
	}
}

func (h *ContentHandler) GetContentByAttributeId(w http.ResponseWriter, r *http.Request) {
	movies, err := h.movieService.GetMoviesByAttributeId(
		r.Context(),
		chi.URLParam(r, "id"),
		contentTypeFromQuery(r, "content"),
	)
	if err != nil {
		errs.Write(w, r, err)
		return
	}
	if err := response.Success(w, r, http.StatusOK, "content fetched", convertToMinimalList(movies)); err != nil {
		logger.ErrorContext(r.Context(), "failed to write response", "error", err)
	}
}

func (h *ContentHandler) GetBanner(w http.ResponseWriter, r *http.Request) {
	banner, err := h.movieService.GetBanner(r.Context(), contentTypeFromQuery(r, "type"))
	if err != nil {
		errs.Write(w, r, err)
		return
	}
	if banner == nil {
		errs.Write(w, r, errs.ErrContentNotFound)
		return
	}
	if err := response.Success(w, r, http.StatusOK, "banner fetched", model.BannerContent{
		ID:            banner.ID,
		BackdropPath:  banner.BackdropPath,
		Title:         banner.Title,
		Overview:      banner.Overview,
		ContentRating: banner.ContentRating,
		Assets:        banner.Assets,
	}); err != nil {
		logger.ErrorContext(r.Context(), "failed to write response", "error", err)
	}
}

func (h *ContentHandler) GetDiscoverContent(w http.ResponseWriter, r *http.Request) {
	discoverType := strings.ToLower(r.URL.Query().Get("type"))
	if discoverType == "" {
		discoverType = "latest"
	}
	movies, err := h.movieService.GetDiscoverContent(r.Context(), discoverType, contentTypeFromQuery(r, "content"))
	if err != nil {
		errs.Write(w, r, err)
		return
	}
	if err := response.Success(w, r, http.StatusOK, "content discovered", convertToPublicList(movies)); err != nil {
		logger.ErrorContext(r.Context(), "failed to write response", "error", err)
	}
}

func (h *ContentHandler) UploadAssets(w http.ResponseWriter, r *http.Request) {
	contentID := chi.URLParam(r, "contentId")

	if err := ParseVideoMultipartForm(r, w); err != nil {
		errs.BadRequestError(w, r, errors.New("body must be multipart/form-data"))
		return
	}

	assetType := r.FormValue("assetType")
	if assetType == "" {
		errs.BadRequestError(w, r, errors.New("assetType is required"))
		return
	}

	if !model.AssetType(assetType).IsValid() {
		errs.BadRequestError(w, r, errors.New("invalid assetType: must be TRAILER, TEASER, CLIP, PROMO, or BTS"))
		return
	}

	files := r.MultipartForm.File["videos"]
	if len(files) == 0 {
		errs.BadRequestError(w, r, errors.New("at least one video file is required"))
		return
	}

	uploadedPaths, err := h.movieService.UploadAssets(r.Context(), contentID, model.AssetType(assetType), files)
	if err != nil {
		errs.Write(w, r, err)
		return
	}

	if err := response.Success(w, r, http.StatusCreated, "assets uploaded successfully", map[string]any{
		"contentId":     contentID,
		"assetType":     assetType,
		"uploadedCount": len(uploadedPaths),
		"paths":         uploadedPaths,
	}); err != nil {
		logger.ErrorContext(r.Context(), "failed to write response", "error", err)
	}
}

func (h *ContentHandler) GetPlayback(w http.ResponseWriter, r *http.Request) {

	contentID := chi.URLParam(r, "contentId")

	info, err := h.movieService.GetPlaybackInfo(r.Context(), contentID)
	if err != nil {
		errs.Write(w, r, err)
		return
	}

	if err := response.Success(w, r, http.StatusOK, "playback information fetched", info); err != nil {
		logger.ErrorContext(r.Context(), "failed to write response", "error", err)
	}
}

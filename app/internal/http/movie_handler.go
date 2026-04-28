package http

import (
	"errors"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/xanderbilla/bi8s-go/internal/errs"
	"github.com/xanderbilla/bi8s-go/internal/model"
	"github.com/xanderbilla/bi8s-go/internal/response"
	"github.com/xanderbilla/bi8s-go/internal/service"
)

type MovieHandler struct {
	movieService *service.MovieService
}

var validAssetTypes = map[string]struct{}{
	string(model.AssetTypeTrailer): {},
	string(model.AssetTypeTeaser):  {},
	string(model.AssetTypeClip):    {},
	string(model.AssetTypePromo):   {},
	string(model.AssetTypeBTS):     {},
}

func contentTypeFromQuery(r *http.Request, key string) string {
	v := strings.ToLower(r.URL.Query().Get(key))
	if v == "" {
		return "all"
	}
	return v
}

func (h *MovieHandler) GetAllMoviesAdmin(w http.ResponseWriter, r *http.Request) {
	movies, err := h.movieService.GetAllAdmin(r.Context())
	if err != nil {
		errs.Write(w, r, err)
		return
	}
	response.Success(w, r, http.StatusOK, "movies fetched", movies)
}

func (h *MovieHandler) GetRecentContent(w http.ResponseWriter, r *http.Request) {
	movies, err := h.movieService.GetRecentContent(r.Context(), contentTypeFromQuery(r, "type"))
	if err != nil {
		errs.Write(w, r, err)
		return
	}
	response.Success(w, r, http.StatusOK, "content fetched", convertToPublicList(movies))
}

func (h *MovieHandler) GetMovie(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "contentId")

	movie, err := h.movieService.Get(r.Context(), id)
	if err != nil {
		errs.Write(w, r, err)
		return
	}
	response.Success(w, r, http.StatusOK, "movie fetched", toMoviePublicDetail(movie))
}

func (h *MovieHandler) GetMovieAdmin(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "movieId")

	movie, err := h.movieService.GetAdmin(r.Context(), id)
	if err != nil {
		errs.Write(w, r, err)
		return
	}
	response.Success(w, r, http.StatusOK, "movie fetched", movie)
}

func (h *MovieHandler) CreateMovie(w http.ResponseWriter, r *http.Request) {
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

	response.Success(w, r, http.StatusCreated, "movie created", newMovie)
}

func (h *MovieHandler) DeleteMovie(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "movieId")

	if err := h.movieService.Delete(r.Context(), id); err != nil {
		if errs.IsConditionalCheckFailed(err) {
			errs.NotFoundError(w, r, err)
			return
		}
		errs.Write(w, r, err)
		return
	}
	response.Success(w, r, http.StatusOK, "movie deleted", nil)
}

func (h *MovieHandler) GetContentByPersonIdAdmin(w http.ResponseWriter, r *http.Request) {
	movies, err := h.movieService.GetContentByPersonIdAdmin(
		r.Context(),
		chi.URLParam(r, "peopleId"),
		contentTypeFromQuery(r, "type"),
	)
	if err != nil {
		errs.Write(w, r, err)
		return
	}
	response.Success(w, r, http.StatusOK, "content fetched", convertToMinimalList(movies))
}

func (h *MovieHandler) GetContentByPersonId(w http.ResponseWriter, r *http.Request) {
	movies, err := h.movieService.GetContentByPersonId(
		r.Context(),
		chi.URLParam(r, "peopleId"),
		contentTypeFromQuery(r, "type"),
	)
	if err != nil {
		errs.Write(w, r, err)
		return
	}
	response.Success(w, r, http.StatusOK, "content fetched", convertToMinimalList(movies))
}

func (h *MovieHandler) GetContentByAttributeId(w http.ResponseWriter, r *http.Request) {
	movies, err := h.movieService.GetMoviesByAttributeId(
		r.Context(),
		chi.URLParam(r, "id"),
		contentTypeFromQuery(r, "content"),
	)
	if err != nil {
		errs.Write(w, r, err)
		return
	}
	response.Success(w, r, http.StatusOK, "content fetched", convertToMinimalList(movies))
}

func (h *MovieHandler) GetBanner(w http.ResponseWriter, r *http.Request) {
	banner, err := h.movieService.GetBanner(r.Context(), contentTypeFromQuery(r, "type"))
	if err != nil {
		errs.Write(w, r, err)
		return
	}
	if banner == nil {
		errs.Write(w, r, errs.ErrContentNotFound)
		return
	}
	response.Success(w, r, http.StatusOK, "banner fetched", model.BannerContent{
		ID:            banner.ID,
		BackdropPath:  banner.BackdropPath,
		Title:         banner.Title,
		Overview:      banner.Overview,
		ContentRating: banner.ContentRating,
		Assets:        banner.Assets,
	})
}

func (h *MovieHandler) GetDiscoverContent(w http.ResponseWriter, r *http.Request) {
	discoverType := strings.ToLower(r.URL.Query().Get("type"))
	if discoverType == "" {
		discoverType = "latest"
	}
	movies, err := h.movieService.GetDiscoverContent(r.Context(), discoverType, contentTypeFromQuery(r, "content"))
	if err != nil {
		errs.Write(w, r, err)
		return
	}
	response.Success(w, r, http.StatusOK, "content discovered", convertToPublicList(movies))
}

func (h *MovieHandler) UploadAssets(w http.ResponseWriter, r *http.Request) {
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

	if _, ok := validAssetTypes[assetType]; !ok {
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

	response.Success(w, r, http.StatusCreated, "assets uploaded successfully", map[string]any{
		"contentId":     contentID,
		"assetType":     assetType,
		"uploadedCount": len(uploadedPaths),
		"paths":         uploadedPaths,
	})
}

func (h *MovieHandler) GetPlayback(w http.ResponseWriter, r *http.Request) {
	// Route is mounted with ValidateURLParams(ContentTypeValidator, ContentIDValidator);
	// values are guaranteed to be valid here.
	contentID := chi.URLParam(r, "contentId")

	info, err := h.movieService.GetPlaybackInfo(r.Context(), contentID)
	if err != nil {
		errs.Write(w, r, err)
		return
	}

	response.Success(w, r, http.StatusOK, "playback information fetched", info)
}

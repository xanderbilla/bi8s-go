package http

import (
	"errors"
	"net/http"
	"strings"

	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/go-chi/chi/v5"
	"github.com/xanderbilla/bi8s-go/internal/errs"
	"github.com/xanderbilla/bi8s-go/internal/model"
	"github.com/xanderbilla/bi8s-go/internal/service"
)

// MovieHandler handles movie-related HTTP routes.
// It keeps handler code focused on request/response flow, while shared packages
// handle validation and error formatting consistently.
type MovieHandler struct {
	movieService *service.MovieService
}

// convertToPublicList converts a slice of movies to public list format.
// This helper reduces code duplication across multiple handlers.
func convertToPublicList(movies []model.Movie) []model.MoviePublicList {
	publicMovies := make([]model.MoviePublicList, len(movies))
	for i, movie := range movies {
		publicMovies[i] = model.MoviePublicList{
			ID:            movie.ID,
			Title:         movie.Title,
			BackdropPath:  movie.BackdropPath,
			PosterPath:    movie.PosterPath,
			ReleaseDate:   movie.ReleaseDate,
			Tags:          movie.Tags,
			ContentRating: movie.ContentRating,
			ContentType:   movie.ContentType,
			Assets:        movie.Assets,
		}
	}
	return publicMovies
}

// convertToMinimalList converts a slice of movies to minimal list format (id, title, backdropPath).
// This helper reduces code duplication for endpoints that return minimal data.
func convertToMinimalList(movies []model.Movie) []model.MoviesByPersonList {
	moviesList := make([]model.MoviesByPersonList, len(movies))
	for i, movie := range movies {
		moviesList[i] = model.MoviesByPersonList{
			ID:           movie.ID,
			Title:        movie.Title,
			BackdropPath: movie.BackdropPath,
		}
	}
	return moviesList
}



// GetAllMoviesAdmin handles GET /v1/a/movies and returns all movies without filtering.
// Returns all fields including stats and audit. No visibility or status filtering.
func (h *MovieHandler) GetAllMoviesAdmin(w http.ResponseWriter, r *http.Request) {
	movies, err := h.movieService.GetAllAdmin(r.Context())
	if err != nil {
		errs.InternalServerError(w, r, err)
		return
	}

	Success(w, http.StatusOK, "movies fetched", movies)
}

// GetRecentContent handles GET /v1/c/content?type=all and returns recent content sorted by creation date.
// The type query parameter can be "all", "movie", or "tv" (case-insensitive).
func (h *MovieHandler) GetRecentContent(w http.ResponseWriter, r *http.Request) {
	contentType := strings.ToLower(r.URL.Query().Get("type"))

	// Default to "all" if not provided
	if contentType == "" {
		contentType = "all"
	}

	movies, err := h.movieService.GetRecentContent(r.Context(), contentType)
	if err != nil {
		errs.InternalServerError(w, r, err)
		return
	}

	Success(w, http.StatusOK, "content fetched", convertToPublicList(movies))
}

// GetMovie handles GET /v1/c/content/{contentId}.
// If the service returns nil data, we treat that as "not found" and return 404.
func (h *MovieHandler) GetMovie(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "contentId")

	movie, err := h.movieService.Get(r.Context(), id)
	if err != nil {
		errs.InternalServerError(w, r, err)
		return
	}

	if movie == nil {
		errs.NotFoundError(w, r, errors.New("movie not found"))
		return
	}

	// Convert to public detail format (exclude stats and audit)
	publicMovie := model.MoviePublicDetail{
		ID:               movie.ID,
		Title:            movie.Title,
		Overview:         movie.Overview,
		BackdropPath:     movie.BackdropPath,
		PosterPath:       movie.PosterPath,
		ReleaseDate:      movie.ReleaseDate,
		FirstAirDate:     movie.FirstAirDate,
		Adult:            movie.Adult,
		ContentRating:    movie.ContentRating,
		OriginalLanguage: movie.OriginalLanguage,
		Genres:           movie.Genres,
		Casts:            movie.Casts,
		Tags:             movie.Tags,
		ContentType:      movie.ContentType,
		OriginCountry:    movie.OriginCountry,
		MoodTags:         movie.MoodTags,
		Runtime:          movie.Runtime,
		Status:           movie.Status,
		Tagline:          movie.Tagline,
		Studios:          movie.Studios,
		Assets:           movie.Assets,
	}

	Success(w, http.StatusOK, "movie fetched", publicMovie)
}

// GetMovieAdmin handles GET /v1/a/movies/{movieId} and returns a single movie without filtering.
// Returns all fields including stats and audit. No visibility or status filtering.
func (h *MovieHandler) GetMovieAdmin(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "movieId")

	movie, err := h.movieService.GetAdmin(r.Context(), id)
	if err != nil {
		errs.InternalServerError(w, r, err)
		return
	}

	if movie == nil {
		errs.NotFoundError(w, r, errors.New("movie not found"))
		return
	}

	Success(w, http.StatusOK, "movie fetched", movie)
}

// CreateMovie handles POST /v1/movies.
// Flow: parse multipart form -> build movie -> map files via helper -> save to storage -> create database record.
// Duplicate IDs are mapped to 409 Conflict for a clearer client contract.
func (h *MovieHandler) CreateMovie(w http.ResponseWriter, r *http.Request) {
	formValues, err := ParseMultipartForm(r)
	if err != nil {
		errs.BadRequestError(w, r, err)
		return
	}

	movie, err := ParseMovieFromForm(formValues)
	if err != nil {
		errs.BadRequestError(w, r, err)
		return
	}

	posterInput, err := ExtractFile(r, "poster")
	if err != nil {
		errs.BadRequestError(w, r, err)
		return
	}

	coverInput, err := ExtractFile(r, "cover")
	if err != nil {
		errs.BadRequestError(w, r, err)
		return
	}

	newMovie, err := h.movieService.Create(r.Context(), movie, posterInput, coverInput)
	if err != nil {
		if errors.Is(err, errs.ErrFileUploaderNotConfigured) {
			errs.InternalServerError(w, r, err)
			return
		}

		// Check if it's a performer not found error
		if strings.Contains(err.Error(), "performer with id") && strings.Contains(err.Error(), "not found") {
			errs.BadRequestError(w, r, err)
			return
		}

		// Check if it's an attribute validation error
		if strings.Contains(err.Error(), "attribute with id") {
			errs.BadRequestError(w, r, err)
			return
		}

		if isConditionalCheckFailed(err) {
			errs.ConflictError(w, r, err)
			return
		}

		errs.InternalServerError(w, r, err)
		return
	}

	Success(w, http.StatusCreated, "movie created", newMovie)
}

// DeleteMovie handles DELETE /v1/movies/{movieId}.
// If the movie does not exist, it returns 404; unexpected failures return 500.
func (h *MovieHandler) DeleteMovie(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "movieId")

	if err := h.movieService.Delete(r.Context(), id); err != nil {
		if isConditionalCheckFailed(err) {
			errs.NotFoundError(w, r, err)
			return
		}

		errs.InternalServerError(w, r, err)
		return
	}

	Success(w, http.StatusOK, "movie deleted", nil)
}

// GetMoviesByPersonId handles GET /v1/a/people/{peopleId}/movies (deprecated - use GetContentByPersonIdAdmin).
// Returns all movies where the person is in the cast.
func (h *MovieHandler) GetMoviesByPersonId(w http.ResponseWriter, r *http.Request) {
	personId := chi.URLParam(r, "peopleId")

	movies, err := h.movieService.GetMoviesByPersonId(r.Context(), personId)
	if err != nil {
		errs.InternalServerError(w, r, err)
		return
	}

	Success(w, http.StatusOK, "movies fetched", convertToMinimalList(movies))
}

// GetContentByPersonIdAdmin handles GET /v1/a/people/{peopleId}/content?type=all.
// Returns all content where the person is in the cast, filtered by content type.
// Admin endpoint - returns all content regardless of visibility or status.
func (h *MovieHandler) GetContentByPersonIdAdmin(w http.ResponseWriter, r *http.Request) {
	personId := chi.URLParam(r, "peopleId")
	contentType := strings.ToLower(r.URL.Query().Get("type"))

	// Default to "all" if not provided
	if contentType == "" {
		contentType = "all"
	}

	movies, err := h.movieService.GetContentByPersonIdAdmin(r.Context(), personId, contentType)
	if err != nil {
		errs.InternalServerError(w, r, err)
		return
	}

	Success(w, http.StatusOK, "content fetched", convertToMinimalList(movies))
}

// GetContentByPersonId handles GET /v1/c/people/{peopleId}/content?type=all.
// Returns all content where the person is in the cast, filtered by content type.
// Only returns content with visibility=PUBLIC and status=RELEASED or IN_PRODUCTION.
func (h *MovieHandler) GetContentByPersonId(w http.ResponseWriter, r *http.Request) {
	personId := chi.URLParam(r, "peopleId")
	contentType := strings.ToLower(r.URL.Query().Get("type"))

	// Default to "all" if not provided
	if contentType == "" {
		contentType = "all"
	}

	movies, err := h.movieService.GetContentByPersonId(r.Context(), personId, contentType)
	if err != nil {
		errs.InternalServerError(w, r, err)
		return
	}

	Success(w, http.StatusOK, "content fetched", convertToMinimalList(movies))
}

// GetContentByAttributeId handles GET /v1/c/attributes/{id}?content=all.
// Returns all content that has the specified attribute, filtered by content type.
func (h *MovieHandler) GetContentByAttributeId(w http.ResponseWriter, r *http.Request) {
	attributeId := chi.URLParam(r, "id")
	contentType := strings.ToLower(r.URL.Query().Get("content"))

	// Default to "all" if not provided
	if contentType == "" {
		contentType = "all"
	}

	movies, err := h.movieService.GetMoviesByAttributeId(r.Context(), attributeId, contentType)
	if err != nil {
		errs.InternalServerError(w, r, err)
		return
	}

	Success(w, http.StatusOK, "content fetched", convertToMinimalList(movies))
}

// GetBanner handles GET /v1/c/banner?type=all and returns a random banner content.
// The type query parameter can be "all", "movie", or "tv" (case-insensitive).
func (h *MovieHandler) GetBanner(w http.ResponseWriter, r *http.Request) {
	contentType := strings.ToLower(r.URL.Query().Get("type"))

	// Default to "all" if not provided
	if contentType == "" {
		contentType = "all"
	}

	banner, err := h.movieService.GetBanner(r.Context(), contentType)
	if err != nil {
		errs.InternalServerError(w, r, err)
		return
	}

	if banner == nil {
		errs.NotFoundError(w, r, errors.New("no banner content found"))
		return
	}

	// Convert to banner format
	bannerContent := model.BannerContent{
		ID:            banner.ID,
		BackdropPath:  banner.BackdropPath,
		Title:         banner.Title,
		Overview:      banner.Overview,
		ContentRating: banner.ContentRating,
		Assets:        banner.Assets,
	}

	Success(w, http.StatusOK, "banner fetched", bannerContent)
}

// GetDiscoverContent handles GET /v1/c/discover?type=latest&content=all.
// Returns content for discovery based on type (latest, popular, trending) and content filter.
func (h *MovieHandler) GetDiscoverContent(w http.ResponseWriter, r *http.Request) {
	discoverType := strings.ToLower(r.URL.Query().Get("type"))
	contentType := strings.ToLower(r.URL.Query().Get("content"))

	// Default to "latest" if type not provided
	if discoverType == "" {
		discoverType = "latest"
	}

	// Default to "all" if content not provided
	if contentType == "" {
		contentType = "all"
	}

	movies, err := h.movieService.GetDiscoverContent(r.Context(), discoverType, contentType)
	if err != nil {
		errs.InternalServerError(w, r, err)
		return
	}

	Success(w, http.StatusOK, "content discovered", convertToPublicList(movies))
}

// isConditionalCheckFailed identifies DynamoDB conditional-write failures.
// We use this to map domain-level conflicts/missing-resource cases to 409/404
// instead of returning a generic 500.
func isConditionalCheckFailed(err error) bool {
	var conditionErr *types.ConditionalCheckFailedException
	return errors.As(err, &conditionErr)
}


// UploadAssets handles POST /v1/a/content/{contentId} to upload video assets
func (h *MovieHandler) UploadAssets(w http.ResponseWriter, r *http.Request) {
	contentID := chi.URLParam(r, "contentId")
	
	// Parse multipart form
	if err := r.ParseMultipartForm(10 << 30); err != nil { // 10GB max
		errs.BadRequestError(w, r, err)
		return
	}
	
	// Get asset type from form
	assetType := r.FormValue("assetType")
	if assetType == "" {
		errs.BadRequestError(w, r, errors.New("assetType is required"))
		return
	}
	
	// Validate asset type
	validAssetTypes := map[string]bool{
		string(model.AssetTypeTrailer): true,
		string(model.AssetTypeTeaser):  true,
		string(model.AssetTypeClip):    true,
		string(model.AssetTypePromo):   true,
		string(model.AssetTypeBTS):     true,
	}
	
	if !validAssetTypes[assetType] {
		errs.BadRequestError(w, r, errors.New("invalid assetType. Must be one of: TRAILER, TEASER, CLIP, PROMO, BTS"))
		return
	}
	
	// Get all video files from the form
	files := r.MultipartForm.File["videos"]
	if len(files) == 0 {
		errs.BadRequestError(w, r, errors.New("at least one video file is required"))
		return
	}
	
	// Upload assets
	uploadedPaths, err := h.movieService.UploadAssets(r.Context(), contentID, model.AssetType(assetType), files)
	if err != nil {
		errs.InternalServerError(w, r, err)
		return
	}
	
	Success(w, http.StatusCreated, "assets uploaded successfully", map[string]interface{}{
		"contentId":  contentID,
		"assetType":  assetType,
		"uploadedCount": len(uploadedPaths),
		"paths":      uploadedPaths,
	})
}


// GetPlayback handles GET /v1/c/play/{contentType}/{contentId} to get playback information
func (h *MovieHandler) GetPlayback(w http.ResponseWriter, r *http.Request) {
	contentType := chi.URLParam(r, "contentType")
	contentID := chi.URLParam(r, "contentId")
	
	// Validate content type (should be "movie" or "tv")
	if contentType != "movie" && contentType != "tv" {
		errs.BadRequestError(w, r, errors.New("invalid content type. Must be 'movie' or 'tv'"))
		return
	}
	
	// Get playback information
	playbackData, err := h.movieService.GetPlaybackInfo(r.Context(), contentID)
	if err != nil {
		if errors.Is(err, errs.ErrContentNotFound) {
			errs.NotFoundError(w, r, err)
			return
		}
		if errors.Is(err, errs.ErrNoEncodingFound) || errors.Is(err, errs.ErrNoCompletedEncoding) {
			errs.NotFoundError(w, r, errors.New("playback not available for this content"))
			return
		}
		errs.InternalServerError(w, r, err)
		return
	}
	
	Success(w, http.StatusOK, "playback information fetched", playbackData)
}

package http

import (
	"errors"
	"net/http"
	"sort"
	"strconv"
	"strings"

	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/go-chi/chi/v5"
	"github.com/xanderbilla/bi8s-go/internal/errs"
	"github.com/xanderbilla/bi8s-go/internal/model"
	"github.com/xanderbilla/bi8s-go/internal/repository"
	"github.com/xanderbilla/bi8s-go/internal/response"
	"github.com/xanderbilla/bi8s-go/internal/search"
	"github.com/xanderbilla/bi8s-go/internal/service"
)

type ContentHandler struct {
	contentService *service.ContentService
}

type contentUpdateRequest struct {
	Title            string                 `json:"title"`
	Overview         string                 `json:"overview"`
	ReleaseDate      string                 `json:"releaseDate"`
	FirstAirDate     string                 `json:"firstAirDate"`
	Adult            bool                   `json:"adult"`
	ContentRating    model.Rating           `json:"contentRating"`
	OriginalLanguage model.OriginalLanguage `json:"originalLanguage"`
	OriginCountry    []string               `json:"originCountry"`
	Runtime          int                    `json:"runtime"`
	Status           model.Status           `json:"status"`
	Tagline          string                 `json:"tagline"`
	Visibility       model.Visibility       `json:"visibility"`
	Assets           []model.Asset          `json:"assets"`
}

type contentRelationMutationRequest struct {
	ContentID string `json:"contentId"`
}

func NewContentHandler(svc *service.ContentService) *ContentHandler {
	return &ContentHandler{contentService: svc}
}

func contentTypeFromQuery(r *http.Request, key string) string {
	switch strings.ToLower(strings.TrimSpace(r.URL.Query().Get(key))) {
	case "movie":
		return "movie"
	case "tv":
		return "tv"
	default:
		return ""
	}
}

func (h *ContentHandler) GetAllContentAdmin(w http.ResponseWriter, r *http.Request) {
	query := strings.TrimSpace(r.URL.Query().Get("q"))
	sortMode := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("sort")))
	if query != "" || sortMode != "" {
		if sortMode == "" {
			sortMode = "recent"
		}
		if !isAdminSortValid(sortMode) {
			errs.BadRequestError(w, r, errs.NewBadRequest("sort must be one of: recent, latest, alpha_asc, alpha_desc"))
			return
		}

		limit, err := parseLimitParam(r.URL.Query().Get("limit"))
		if err != nil {
			errs.BadRequestError(w, r, err)
			return
		}
		offset, err := parseOffsetCursor(r.URL.Query().Get("cursor"))
		if err != nil {
			errs.BadRequestError(w, r, err)
			return
		}

		items := make([]model.Movie, 0)
		var startKey map[string]types.AttributeValue
		for {
			pageItems, nextKey, err := h.contentService.GetAllAdmin(r.Context(), maxPageLimit, startKey)
			if err != nil {
				errs.Write(w, r, err)
				return
			}
			items = append(items, pageItems...)
			if len(nextKey) == 0 {
				break
			}
			startKey = nextKey
		}

		items = filterAdminContent(items, query)
		sortAdminContent(items, sortMode)
		page, next := paginateMovies(items, offset, int(limit))

		writeOK(w, r, http.StatusOK, "content fetched", response.PagedData[model.Movie]{
			Items:      page,
			NextCursor: next,
			Count:      len(page),
		})
		return
	}

	limit, startKey, err := parsePaginationParams(r)
	if err != nil {
		errs.BadRequestError(w, r, err)
		return
	}
	contentItems, nextKey, err := h.contentService.GetAllAdmin(r.Context(), limit, startKey)
	if err != nil {
		errs.Write(w, r, err)
		return
	}
	cursor, _ := repository.EncodeCursor(nextKey)
	writeOK(w, r, http.StatusOK, "content fetched", response.PagedData[model.Movie]{
		Items:      contentItems,
		NextCursor: cursor,
		Count:      len(contentItems),
	})
}

func (h *ContentHandler) GetContent(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "contentId")

	content, err := h.contentService.Get(r.Context(), id)
	if err != nil {
		errs.Write(w, r, err)
		return
	}
	writeOK(w, r, http.StatusOK, "content fetched", toContentPublicDetail(content))
}

func (h *ContentHandler) GetContentAdmin(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "contentId")

	content, err := h.contentService.GetAdmin(r.Context(), id)
	if err != nil {
		errs.Write(w, r, err)
		return
	}
	writeOK(w, r, http.StatusOK, "content fetched", content)
}

func (h *ContentHandler) CreateContent(w http.ResponseWriter, r *http.Request) {
	formValues, files, err := ParseFormAndFiles(w, r, []string{"poster", "cover"})
	if err != nil {
		errs.BadRequestError(w, r, err)
		return
	}

	content, err := ParseContentFromForm(formValues)
	if err != nil {
		errs.BadRequestError(w, r, err)
		return
	}

	newContent, err := h.contentService.Create(r.Context(), content, files["poster"], files["cover"])
	if err != nil {
		errs.Write(w, r, err)
		return
	}

	writeOK(w, r, http.StatusCreated, "content created", newContent)
}

func (h *ContentHandler) DeleteContent(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "contentId")

	if err := h.contentService.Delete(r.Context(), id); err != nil {
		if errs.IsConditionalCheckFailed(err) {
			errs.NotFoundError(w, r, err)
			return
		}
		errs.Write(w, r, err)
		return
	}
	writeOK(w, r, http.StatusOK, "content deleted", nil)
}

func (h *ContentHandler) UpdateContent(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "contentId")

	var req contentUpdateRequest
	if err := Decode(w, r, &req); err != nil {
		errs.BadRequestError(w, r, errs.NewBadRequest(err.Error()))
		return
	}

	patch := model.Movie{
		Title:            strings.TrimSpace(req.Title),
		Overview:         clampMaxChars(req.Overview, 150),
		ReleaseDate:      strings.TrimSpace(req.ReleaseDate),
		FirstAirDate:     strings.TrimSpace(req.FirstAirDate),
		Adult:            req.Adult,
		ContentRating:    req.ContentRating,
		OriginalLanguage: req.OriginalLanguage,
		OriginCountry:    req.OriginCountry,
		Runtime:          req.Runtime,
		Status:           req.Status,
		Tagline:          strings.TrimSpace(req.Tagline),
		Visibility:       req.Visibility,
		Assets:           req.Assets,
	}

	updated, err := h.contentService.UpdateCore(r.Context(), id, patch)
	if err != nil {
		errs.Write(w, r, err)
		return
	}

	writeOK(w, r, http.StatusOK, "content updated", updated)
}

func (h *ContentHandler) UpdateContentPoster(w http.ResponseWriter, r *http.Request) {
	h.updateContentImageByPurpose(w, r, "poster")
}

func (h *ContentHandler) UpdateContentBackdrop(w http.ResponseWriter, r *http.Request) {
	h.updateContentImageByPurpose(w, r, "backdrop")
}

func (h *ContentHandler) updateContentImageByPurpose(w http.ResponseWriter, r *http.Request, purpose string) {
	id := chi.URLParam(r, "contentId")

	if _, err := ParseMultipartForm(r, w); err != nil {
		errs.BadRequestError(w, r, err)
		return
	}

	input, err := ExtractFile(r, purpose)
	if err != nil {
		errs.BadRequestError(w, r, err)
		return
	}
	if input == nil {
		input, err = ExtractFile(r, "image")
		if err != nil {
			errs.BadRequestError(w, r, err)
			return
		}
	}
	if input == nil {
		errs.BadRequestError(w, r, errs.NewBadRequest("image file is required"))
		return
	}

	var updated *model.Movie
	if purpose == "poster" {
		updated, err = h.contentService.UpdatePosterImage(r.Context(), id, input)
	} else {
		updated, err = h.contentService.UpdateBackdropImage(r.Context(), id, input)
	}
	if err != nil {
		errs.Write(w, r, err)
		return
	}

	writeOK(w, r, http.StatusOK, "content image updated", updated)
}

func (h *ContentHandler) AddContentRelation(w http.ResponseWriter, r *http.Request) {
	h.mutateContentRelation(w, r, true)
}

func (h *ContentHandler) RemoveContentRelation(w http.ResponseWriter, r *http.Request) {
	h.mutateContentRelation(w, r, false)
}

func (h *ContentHandler) mutateContentRelation(w http.ResponseWriter, r *http.Request, add bool) {
	relationID := chi.URLParam(r, "attributeId")

	var req contentRelationMutationRequest
	if err := Decode(w, r, &req); err != nil {
		errs.BadRequestError(w, r, errs.NewBadRequest(err.Error()))
		return
	}
	if strings.TrimSpace(req.ContentID) == "" {
		errs.BadRequestError(w, r, errs.NewBadRequest("contentId is required"))
		return
	}

	var (
		updated *model.Movie
		err     error
	)
	if add {
		updated, err = h.contentService.AddRelation(r.Context(), req.ContentID, relationID)
	} else {
		updated, err = h.contentService.RemoveRelation(r.Context(), req.ContentID, relationID)
	}
	if err != nil {
		errs.Write(w, r, err)
		return
	}

	msg := "content relation removed"
	if add {
		msg = "content relation added"
	}
	writeOK(w, r, http.StatusOK, msg, updated)
}

func (h *ContentHandler) GetContentByPersonIdAdmin(w http.ResponseWriter, r *http.Request) {
	limit, startKey, err := parsePaginationParams(r)
	if err != nil {
		errs.BadRequestError(w, r, err)
		return
	}
	contentItems, nextKey, err := h.contentService.GetContentByPersonIdAdmin(
		r.Context(),
		chi.URLParam(r, "peopleId"),
		contentTypeFromQuery(r, "type"),
		limit,
		startKey,
	)
	if err != nil {
		errs.Write(w, r, err)
		return
	}
	cursor, _ := repository.EncodeCursor(nextKey)
	writeOK(w, r, http.StatusOK, "content fetched", response.PagedData[model.MoviesByPersonList]{
		Items:      convertToMinimalList(contentItems),
		NextCursor: cursor,
		Count:      len(contentItems),
	})
}

func (h *ContentHandler) GetContentByPersonId(w http.ResponseWriter, r *http.Request) {
	limit, startKey, err := parsePaginationParams(r)
	if err != nil {
		errs.BadRequestError(w, r, err)
		return
	}
	contentItems, nextKey, err := h.contentService.GetContentByPersonId(
		r.Context(),
		chi.URLParam(r, "peopleId"),
		contentTypeFromQuery(r, "type"),
		limit,
		startKey,
	)
	if err != nil {
		errs.Write(w, r, err)
		return
	}
	cursor, _ := repository.EncodeCursor(nextKey)
	writeOK(w, r, http.StatusOK, "content fetched", response.PagedData[model.MoviesByPersonList]{
		Items:      convertToMinimalList(contentItems),
		NextCursor: cursor,
		Count:      len(contentItems),
	})
}

func (h *ContentHandler) GetContentByAttributeId(w http.ResponseWriter, r *http.Request) {
	sortMode, err := parseAlphaSort(r.URL.Query().Get("sort"))
	if err != nil {
		errs.BadRequestError(w, r, err)
		return
	}
	if sortMode != "" {
		h.getAlphabeticallySortedContentByAttributeID(w, r, chi.URLParam(r, "id"), contentTypeFromQuery(r, "content"), sortMode)
		return
	}

	limit, startKey, err := parsePaginationParams(r)
	if err != nil {
		errs.BadRequestError(w, r, err)
		return
	}
	contentItems, nextKey, err := h.contentService.GetContentByAttributeId(
		r.Context(),
		chi.URLParam(r, "id"),
		contentTypeFromQuery(r, "content"),
		limit,
		startKey,
	)
	if err != nil {
		errs.Write(w, r, err)
		return
	}
	cursor, _ := repository.EncodeCursor(nextKey)
	writeOK(w, r, http.StatusOK, "content fetched", response.PagedData[model.MoviesByPersonList]{
		Items:      convertToMinimalList(contentItems),
		NextCursor: cursor,
		Count:      len(contentItems),
	})
}

func (h *ContentHandler) getAlphabeticallySortedContentByAttributeID(w http.ResponseWriter, r *http.Request, attributeID string, contentTypeFilter string, sortMode string) {
	limit, err := parseLimitParam(r.URL.Query().Get("limit"))
	if err != nil {
		errs.BadRequestError(w, r, err)
		return
	}
	offset, err := parseOffsetCursor(r.URL.Query().Get("cursor"))
	if err != nil {
		errs.BadRequestError(w, r, err)
		return
	}
	items, err := h.collectContentByAttributeID(r, attributeID, contentTypeFilter)
	if err != nil {
		errs.Write(w, r, err)
		return
	}
	sortMoviesAlphabetically(items, sortMode == search.SortAlphaDesc)
	pageItems, nextCursor := paginateDiscoverMovies(items, offset, int(limit))

	writeOK(w, r, http.StatusOK, "content fetched", response.PagedData[model.MoviesByPersonList]{
		Items:      convertToMinimalList(pageItems),
		NextCursor: nextCursor,
		Count:      len(pageItems),
	})
}

func (h *ContentHandler) collectContentByAttributeID(r *http.Request, attributeID string, contentTypeFilter string) ([]model.Movie, error) {
	items := make([]model.Movie, 0)
	var startKey map[string]types.AttributeValue
	for {
		pageItems, nextKey, err := h.contentService.GetContentByAttributeId(r.Context(), attributeID, contentTypeFilter, maxPageLimit, startKey)
		if err != nil {
			return nil, err
		}
		items = append(items, pageItems...)
		if len(nextKey) == 0 {
			break
		}
		startKey = nextKey
	}
	return items, nil
}

func (h *ContentHandler) GetBanner(w http.ResponseWriter, r *http.Request) {
	banner, err := h.contentService.GetBanner(r.Context(), contentTypeFromQuery(r, "type"))
	if err != nil {
		errs.Write(w, r, err)
		return
	}
	if banner == nil {
		errs.Write(w, r, errs.ErrContentNotFound)
		return
	}
	writeOK(w, r, http.StatusOK, "banner fetched", model.BannerContent{
		ID:            banner.ID,
		BackdropPath:  banner.BackdropPath,
		Title:         banner.Title,
		Overview:      banner.Overview,
		ContentRating: banner.ContentRating,
		Assets:        banner.Assets,
	})
}

func (h *ContentHandler) GetDiscoverContent(w http.ResponseWriter, r *http.Request) {
	discoverType := strings.ToLower(r.URL.Query().Get("type"))
	if discoverType == "" {
		discoverType = "latest"
	}
	sortMode, err := parseAlphaSort(r.URL.Query().Get("sort"))
	if err != nil {
		errs.BadRequestError(w, r, err)
		return
	}
	if sortMode != "" {
		h.getAlphabeticallySortedDiscoverContent(w, r, discoverType, sortMode)
		return
	}
	limit, startKey, err := parsePaginationParams(r)
	if err != nil {
		errs.BadRequestError(w, r, err)
		return
	}
	contentItems, nextKey, err := h.contentService.GetDiscoverContent(r.Context(), discoverType, contentTypeFromQuery(r, "content"), limit, startKey)
	if err != nil {
		errs.Write(w, r, err)
		return
	}
	cursor, _ := repository.EncodeCursor(nextKey)
	writeOK(w, r, http.StatusOK, "content discovered", response.PagedData[model.MoviePublicList]{
		Items:      convertToPublicList(contentItems),
		NextCursor: cursor,
		Count:      len(contentItems),
	})
}

func (h *ContentHandler) getAlphabeticallySortedDiscoverContent(w http.ResponseWriter, r *http.Request, discoverType string, sortMode string) {
	limit, err := parseLimitParam(r.URL.Query().Get("limit"))
	if err != nil {
		errs.BadRequestError(w, r, err)
		return
	}
	offset, err := parseOffsetCursor(r.URL.Query().Get("cursor"))
	if err != nil {
		errs.BadRequestError(w, r, err)
		return
	}
	items, err := h.collectDiscoverContent(r, discoverType, contentTypeFromQuery(r, "content"))
	if err != nil {
		errs.Write(w, r, err)
		return
	}
	sortMoviesAlphabetically(items, sortMode == search.SortAlphaDesc)
	pageItems, nextCursor := paginateDiscoverMovies(items, offset, int(limit))
	writeOK(w, r, http.StatusOK, "content discovered", response.PagedData[model.MoviePublicList]{
		Items:      convertToPublicList(pageItems),
		NextCursor: nextCursor,
		Count:      len(pageItems),
	})
}

func (h *ContentHandler) collectDiscoverContent(r *http.Request, discoverType string, contentTypeFilter string) ([]model.Movie, error) {
	items := make([]model.Movie, 0)
	var startKey map[string]types.AttributeValue
	for {
		pageItems, nextKey, err := h.contentService.GetDiscoverContent(r.Context(), discoverType, contentTypeFilter, maxPageLimit, startKey)
		if err != nil {
			return nil, err
		}
		items = append(items, pageItems...)
		if len(nextKey) == 0 {
			break
		}
		startKey = nextKey
	}
	return items, nil
}

func parseOffsetCursor(raw string) (int, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return 0, nil
	}
	offset, err := strconv.Atoi(raw)
	if err != nil || offset < 0 {
		return 0, errors.New("invalid cursor")
	}
	return offset, nil
}

func sortMoviesAlphabetically(items []model.Movie, descending bool) {
	sort.SliceStable(items, func(i, j int) bool {
		left := strings.ToLower(strings.TrimSpace(items[i].Title))
		right := strings.ToLower(strings.TrimSpace(items[j].Title))
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

func paginateDiscoverMovies(items []model.Movie, offset int, limit int) ([]model.Movie, string) {
	if offset >= len(items) {
		return []model.Movie{}, ""
	}
	end := offset + limit
	if end > len(items) {
		end = len(items)
	}
	nextCursor := ""
	if end < len(items) {
		nextCursor = strconv.Itoa(end)
	}
	return items[offset:end], nextCursor
}

func (h *ContentHandler) UploadAssets(w http.ResponseWriter, r *http.Request) {
	contentID := chi.URLParam(r, "contentId")

	if err := ParseVideoMultipartForm(r, w); err != nil {
		errs.BadRequestError(w, r, errors.New("body must be multipart/form-data"))
		return
	}

	if bodyContentID := r.FormValue("contentid"); bodyContentID != "" {
		contentID = bodyContentID
	}

	if r.FormValue("contenttype") == "" {
		errs.BadRequestError(w, r, errors.New("contenttype is required"))
		return
	}

	assetType := r.FormValue("assettype")
	if assetType == "" {
		errs.BadRequestError(w, r, errors.New("assettype is required"))
		return
	}

	if !model.AssetType(assetType).IsValid() {
		errs.BadRequestError(w, r, errors.New("invalid assettype: must be TRAILER, TEASER, CLIP, PROMO, or BTS"))
		return
	}

	files := r.MultipartForm.File["video"]
	if len(files) == 0 {
		errs.BadRequestError(w, r, errors.New("video file is required"))
		return
	}

	if len(files) > 1 {
		errs.BadRequestError(w, r, errors.New("only one video file can be uploaded at a time"))
		return
	}

	uploadedPaths, err := h.contentService.UploadAssets(r.Context(), contentID, model.AssetType(assetType), files)
	if err != nil {
		errs.Write(w, r, err)
		return
	}

	writeOK(w, r, http.StatusCreated, "assets uploaded successfully", map[string]any{
		"contentId":     contentID,
		"assetType":     assetType,
		"uploadedCount": len(uploadedPaths),
		"paths":         uploadedPaths,
	})
}

func (h *ContentHandler) DeleteAssetKey(w http.ResponseWriter, r *http.Request) {
	contentID := chi.URLParam(r, "contentId")
	assetTypeStr := chi.URLParam(r, "assetType")
	keyID := chi.URLParam(r, "keyId")

	if !model.AssetType(assetTypeStr).IsValid() {
		errs.BadRequestError(w, r, errors.New("invalid assetType: must be TRAILER, TEASER, CLIP, PROMO, or BTS"))
		return
	}

	if err := h.contentService.DeleteAssetKey(r.Context(), contentID, model.AssetType(assetTypeStr), keyID); err != nil {
		errs.Write(w, r, err)
		return
	}

	writeOK(w, r, http.StatusOK, "asset key deleted successfully", map[string]any{
		"contentId": contentID,
		"assetType": assetTypeStr,
		"keyId":     keyID,
	})
}

// GetPlayback returns presigned playback URLs for a finished encoder job.
func (h *ContentHandler) GetPlayback(w http.ResponseWriter, r *http.Request) {
	contentType := chi.URLParam(r, "contentType")
	contentID := chi.URLParam(r, "contentId")
	playback, err := h.contentService.GetPlayback(r.Context(), contentID, contentType)
	if err != nil {
		errs.Write(w, r, err)
		return
	}
	writeOK(w, r, http.StatusOK, "playback fetched", playback)
}

package http

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/xanderbilla/bi8s-go/internal/errs"
	"github.com/xanderbilla/bi8s-go/internal/service"
)

type SearchHandler struct {
	searchService *service.SearchService
}

func NewSearchHandler(searchService *service.SearchService) *SearchHandler {
	return &SearchHandler{searchService: searchService}
}

func (h *SearchHandler) Search(w http.ResponseWriter, r *http.Request) {
	query := strings.TrimSpace(r.URL.Query().Get("query"))
	if query == "" {
		errs.BadRequestError(w, r, errs.NewBadRequest("query is required"))
		return
	}

	in := strings.TrimSpace(r.URL.Query().Get("in"))
	page := parseIntDefault(r.URL.Query().Get("page"), 1)
	pageSize := parsePageSizeDefault(r.URL.Query().Get("pageSize"), 20)

	contentPage := parseIntDefault(r.URL.Query().Get("contentPage"), page)
	contentPageSize := parsePageSizeDefault(r.URL.Query().Get("contentPageSize"), pageSize)
	peoplePage := parseIntDefault(r.URL.Query().Get("peoplePage"), page)
	peoplePageSize := parsePageSizeDefault(r.URL.Query().Get("peoplePageSize"), pageSize)

	data, err := h.searchService.Search(r.Context(), service.SearchInput{
		Query:           query,
		In:              in,
		Sort:            strings.TrimSpace(r.URL.Query().Get("sort")),
		ContentPage:     contentPage,
		ContentPageSize: contentPageSize,
		PeoplePage:      peoplePage,
		PeoplePageSize:  peoplePageSize,
	})
	if err != nil {
		if isSearchInputError(err) {
			errs.Write(w, r, errs.NewBadRequest(err.Error()))
			return
		}
		errs.Write(w, r, errs.NewInternal(err))
		return
	}

	writeOK(w, r, http.StatusOK, "search results", data)
}

const maxSearchPageSize = 100

func parseIntDefault(raw string, fallback int) int {
	if strings.TrimSpace(raw) == "" {
		return fallback
	}
	v, err := strconv.Atoi(raw)
	if err != nil || v <= 0 {
		return fallback
	}
	return v
}

func parsePageSizeDefault(raw string, fallback int) int {
	v := parseIntDefault(raw, fallback)
	if v > maxSearchPageSize {
		return maxSearchPageSize
	}
	return v
}

func isSearchInputError(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return strings.Contains(msg, "invalid 'in' value") ||
		strings.Contains(msg, "invalid 'sort' value") ||
		strings.Contains(msg, "page exceeds maximum") ||
		strings.Contains(msg, "requested page window is too deep")
}

func (h *SearchHandler) MoreLikeThis(w http.ResponseWriter, r *http.Request) {
	contentID := chi.URLParam(r, "contentId")
	if strings.TrimSpace(contentID) == "" {
		errs.Write(w, r, errs.NewBadRequest("contentId is required"))
		return
	}

	page := parseIntDefault(r.URL.Query().Get("page"), 1)
	pageSize := parsePageSizeDefault(r.URL.Query().Get("pageSize"), 20)

	results, total, err := h.searchService.MoreLikeThis(r.Context(), contentID, page, pageSize)
	if err != nil {
		errs.Write(w, r, errs.NewInternal(err))
		return
	}

	writeOK(w, r, http.StatusOK, "related content", map[string]any{
		"items": results,
		"total": total,
		"page":  page,
		"size":  len(results),
	})
}

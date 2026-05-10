package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/xanderbilla/bi8s-go/internal/logger"
	"github.com/xanderbilla/bi8s-go/internal/model"
	"github.com/xanderbilla/bi8s-go/internal/search"
)

const (
	SearchInAll    = "all"
	SearchInMovie  = "movie"
	SearchInTV     = "tv"
	SearchInPeople = "people"

	maxSearchPage = 500
	maxSearchFrom = 10000
)

const (
	warningCodePartialSearch = "PARTIAL_SEARCH"
	warningScopeContent      = "content"
	warningScopePeople       = "people"
)

type SearchService struct {
	provider              search.Provider
	enabled               bool
	disabledIndexWarnOnce sync.Once
}

func NewSearchService(provider search.Provider, enabled bool) *SearchService {
	if provider == nil {
		provider = search.NoopProvider{}
	}
	return &SearchService{provider: provider, enabled: enabled}
}

func (s *SearchService) EnsureIndexes(ctx context.Context) error {
	if !s.enabled {
		return nil
	}
	return s.provider.EnsureIndexes(ctx)
}

type SearchInput struct {
	Query           string
	In              string
	Sort            string
	ContentPage     int
	ContentPageSize int
	PeoplePage      int
	PeoplePageSize  int
}

func normalizeSearchSort(raw string) (string, error) {
	token := strings.ToLower(strings.TrimSpace(raw))
	if token == "" {
		return search.SortRecent, nil
	}

	if strings.Contains(token, ",") {
		return "", errors.New("invalid 'sort' value")
	}

	switch token {
	case search.SortRecent, search.SortLatest, search.SortAlphaAsc, search.SortAlphaDesc:
		return token, nil
	default:
		return "", errors.New("invalid 'sort' value")
	}
}

func validatePagination(page, pageSize int, scope string) error {
	if page > maxSearchPage {
		return fmt.Errorf("%s page exceeds maximum limit of %d", scope, maxSearchPage)
	}
	if (page-1)*pageSize+pageSize > maxSearchFrom {
		return fmt.Errorf("%s requested page window is too deep (max from=%d)", scope, maxSearchFrom)
	}
	return nil
}

func (s *SearchService) Search(ctx context.Context, input SearchInput) (resp model.SearchResponse, err error) {
	start := time.Now()
	defer func() {
		attrs := []any{
			"scope", "search",
			"in", input.In,
			"sort", input.Sort,
			"contentPage", input.ContentPage,
			"contentPageSize", input.ContentPageSize,
			"peoplePage", input.PeoplePage,
			"peoplePageSize", input.PeoplePageSize,
			"contentCount", resp.Content.Count,
			"peopleCount", resp.People.Count,
			"warningCount", len(resp.Warnings),
			"durationMs", time.Since(start).Milliseconds(),
		}
		if err != nil {
			logger.WarnContext(ctx, "search failed", append(attrs, "error", err.Error())...)
			return
		}
		logger.InfoContext(ctx, "search completed", attrs...)
	}()

	if !s.enabled {
		return model.SearchResponse{}, errors.New("search is disabled")
	}
	input.In = normalizeSearchIn(input.In)
	var sortErr error
	input.Sort, sortErr = normalizeSearchSort(input.Sort)
	if sortErr != nil {
		return model.SearchResponse{}, sortErr
	}

	if input.ContentPage < 1 {
		input.ContentPage = 1
	}
	if input.ContentPageSize < 1 {
		input.ContentPageSize = 20
	}
	if input.ContentPageSize > 100 {
		input.ContentPageSize = 100
	}

	if input.PeoplePage < 1 {
		input.PeoplePage = 1
	}
	if input.PeoplePageSize < 1 {
		input.PeoplePageSize = 20
	}
	if input.PeoplePageSize > 100 {
		input.PeoplePageSize = 100
	}

	resp = model.SearchResponse{
		Content: model.SearchContentPage{Results: []model.MoviePublicList{}, Page: input.ContentPage, PageSize: input.ContentPageSize},
		People:  model.SearchPeoplePage{Results: []model.SearchPersonResult{}, Page: input.PeoplePage, PageSize: input.PeoplePageSize},
	}

	switch input.In {
	case SearchInMovie, SearchInTV:
		if err := validatePagination(input.ContentPage, input.ContentPageSize, "content"); err != nil {
			return model.SearchResponse{}, err
		}
		content, total, err := s.provider.SearchContent(ctx, input.Query, input.In, input.Sort, input.ContentPage, input.ContentPageSize)
		if err != nil {
			return model.SearchResponse{}, err
		}
		resp.Content.Results = content
		resp.Content.Count = total
		return resp, nil
	case SearchInPeople:
		if err := validatePagination(input.PeoplePage, input.PeoplePageSize, "people"); err != nil {
			return model.SearchResponse{}, err
		}
		people, total, err := s.provider.SearchPeople(ctx, input.Query, input.Sort, input.PeoplePage, input.PeoplePageSize)
		if err != nil {
			return model.SearchResponse{}, err
		}
		resp.People.Results = people
		resp.People.Count = total
		return resp, nil
	case SearchInAll:
		if err := validatePagination(input.ContentPage, input.ContentPageSize, "content"); err != nil {
			return model.SearchResponse{}, err
		}
		if err := validatePagination(input.PeoplePage, input.PeoplePageSize, "people"); err != nil {
			return model.SearchResponse{}, err
		}
		type contentResult struct {
			items []model.MoviePublicList
			total int
			err   error
		}
		type peopleResult struct {
			items []model.SearchPersonResult
			total int
			err   error
		}

		contentCh := make(chan contentResult, 1)
		peopleCh := make(chan peopleResult, 1)

		go func() {
			items, total, callErr := s.provider.SearchContent(ctx, input.Query, "", input.Sort, input.ContentPage, input.ContentPageSize)
			contentCh <- contentResult{items: items, total: total, err: callErr}
		}()

		go func() {
			items, total, callErr := s.provider.SearchPeople(ctx, input.Query, input.Sort, input.PeoplePage, input.PeoplePageSize)
			peopleCh <- peopleResult{items: items, total: total, err: callErr}
		}()

		contentRes := <-contentCh
		peopleRes := <-peopleCh

		if contentRes.err == nil {
			resp.Content.Results = contentRes.items
			resp.Content.Count = contentRes.total
		}
		if peopleRes.err == nil {
			resp.People.Results = peopleRes.items
			resp.People.Count = peopleRes.total
		}

		if contentRes.err != nil && peopleRes.err != nil {
			return model.SearchResponse{}, fmt.Errorf("content search: %w; people search: %w", contentRes.err, peopleRes.err)
		}
		if contentRes.err != nil {
			resp.Warnings = append(resp.Warnings, model.SearchWarning{
				Scope:   warningScopeContent,
				Code:    warningCodePartialSearch,
				Message: "content search unavailable",
			})
			logger.WarnContext(ctx, "partial search result", "scope", warningScopeContent, "error", contentRes.err.Error())
		}
		if peopleRes.err != nil {
			resp.Warnings = append(resp.Warnings, model.SearchWarning{
				Scope:   warningScopePeople,
				Code:    warningCodePartialSearch,
				Message: "people search unavailable",
			})
			logger.WarnContext(ctx, "partial search result", "scope", warningScopePeople, "error", peopleRes.err.Error())
		}

		return resp, nil
	default:
		return model.SearchResponse{}, errors.New("invalid 'in' value")
	}
}

func (s *SearchService) IndexContent(ctx context.Context, movie model.Movie) error {
	if !s.enabled {
		s.warnIndexingDisabled(ctx)
		return nil
	}
	return s.provider.IndexContent(ctx, movie)
}

func (s *SearchService) DeleteContent(ctx context.Context, id string) error {
	if !s.enabled {
		s.warnIndexingDisabled(ctx)
		return nil
	}
	return s.provider.DeleteContent(ctx, id)
}

func (s *SearchService) IndexPerson(ctx context.Context, person model.Person) error {
	if !s.enabled {
		s.warnIndexingDisabled(ctx)
		return nil
	}
	return s.provider.IndexPerson(ctx, person)
}

func (s *SearchService) DeletePerson(ctx context.Context, id string) error {
	if !s.enabled {
		s.warnIndexingDisabled(ctx)
		return nil
	}
	return s.provider.DeletePerson(ctx, id)
}

func (s *SearchService) MoreLikeThis(ctx context.Context, contentID string, page, pageSize int) ([]model.MoviePublicList, int, error) {
	if !s.enabled {
		return []model.MoviePublicList{}, 0, nil
	}
	return s.provider.MoreLikeThis(ctx, contentID, page, pageSize)
}

func (s *SearchService) warnIndexingDisabled(ctx context.Context) {
	s.disabledIndexWarnOnce.Do(func() {
		logger.WarnContext(ctx, "search indexing is disabled; create/update/delete operations will not be indexed")
	})
}

func normalizeSearchIn(in string) string {
	switch strings.ToLower(strings.TrimSpace(in)) {
	case "", SearchInAll:
		return SearchInAll
	case SearchInMovie:
		return SearchInMovie
	case SearchInTV:
		return SearchInTV
	case SearchInPeople:
		return SearchInPeople
	default:
		return "invalid"
	}
}

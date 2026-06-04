package http

import (
	"net/http"
	"sort"
	"strconv"
	"strings"

	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/xanderbilla/bi8s-go/internal/errs"
	"github.com/xanderbilla/bi8s-go/internal/model"
	"github.com/xanderbilla/bi8s-go/internal/search"
	"github.com/xanderbilla/bi8s-go/internal/service"
)

type AdminSearchHandler struct {
	contentService   *service.ContentService
	personService    *service.PersonService
	attributeService *service.AttributeService
}

func NewAdminSearchHandler(contentService *service.ContentService, personService *service.PersonService, attributeService *service.AttributeService) *AdminSearchHandler {
	return &AdminSearchHandler{
		contentService:   contentService,
		personService:    personService,
		attributeService: attributeService,
	}
}

type adminSearchSection[T any] struct {
	Items      []T    `json:"items"`
	Count      int    `json:"count"`
	Total      int    `json:"total"`
	NextCursor string `json:"nextCursor,omitempty"`
}

type adminSearchData struct {
	Query      string                               `json:"query"`
	Entity     string                               `json:"entity"`
	Sort       string                               `json:"sort"`
	Limit      int32                                `json:"limit"`
	Cursor     int                                  `json:"cursor"`
	Content    *adminSearchSection[model.Movie]     `json:"content,omitempty"`
	People     *adminSearchSection[model.Person]    `json:"people,omitempty"`
	Attributes *adminSearchSection[model.Attribute] `json:"attributes,omitempty"`
	Detail     map[string]any                       `json:"detail,omitempty"`
}

func (h *AdminSearchHandler) SearchAdmin(w http.ResponseWriter, r *http.Request) {
	query := strings.TrimSpace(r.URL.Query().Get("q"))
	entity := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("entity")))
	if entity == "" {
		entity = "all"
	}
	if entity != "all" && entity != "content" && entity != "people" && entity != "attributes" {
		errs.BadRequestError(w, r, errs.NewBadRequest("entity must be one of: all, content, people, attributes"))
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

	sortMode := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("sort")))
	if sortMode == "" {
		sortMode = "recent"
	}
	if !isAdminSortValid(sortMode) {
		errs.BadRequestError(w, r, errs.NewBadRequest("sort must be one of: recent, latest, alpha_asc, alpha_desc"))
		return
	}

	if id := strings.TrimSpace(r.URL.Query().Get("id")); id != "" {
		h.handleAdminDetailLookup(w, r, entity, id, query, sortMode, limit, offset)
		return
	}

	data := adminSearchData{
		Query:  query,
		Entity: entity,
		Sort:   sortMode,
		Limit:  limit,
		Cursor: offset,
	}

	attributeTypeFilter := strings.TrimSpace(r.URL.Query().Get("attributeType"))

	if entity == "all" || entity == "content" {
		contentItems, err := h.collectAllAdminContent(r)
		if err != nil {
			errs.Write(w, r, err)
			return
		}
		contentItems = filterAdminContent(contentItems, query)
		sortAdminContent(contentItems, sortMode)
		page, next := paginateMovies(contentItems, offset, int(limit))
		data.Content = &adminSearchSection[model.Movie]{
			Items:      page,
			Count:      len(page),
			Total:      len(contentItems),
			NextCursor: next,
		}
	}

	if entity == "all" || entity == "people" {
		peopleItems, err := h.collectAllPeople(r)
		if err != nil {
			errs.Write(w, r, err)
			return
		}
		peopleItems = filterAdminPeople(peopleItems, query)
		sortAdminPeople(peopleItems, sortMode)
		page, next := paginatePeople(peopleItems, offset, int(limit))
		data.People = &adminSearchSection[model.Person]{
			Items:      page,
			Count:      len(page),
			Total:      len(peopleItems),
			NextCursor: next,
		}
	}

	if entity == "all" || entity == "attributes" {
		attributes, err := h.attributeService.GetAll(r.Context())
		if err != nil {
			errs.Write(w, r, err)
			return
		}
		attributes = filterAdminAttributes(attributes, query, attributeTypeFilter)
		sortAdminAttributes(attributes, sortMode)
		page, next := paginateAttributes(attributes, offset, int(limit))
		data.Attributes = &adminSearchSection[model.Attribute]{
			Items:      page,
			Count:      len(page),
			Total:      len(attributes),
			NextCursor: next,
		}
	}

	writeOK(w, r, http.StatusOK, "admin search results", data)
}

func (h *AdminSearchHandler) handleAdminDetailLookup(w http.ResponseWriter, r *http.Request, entity string, id string, query string, sortMode string, limit int32, offset int) {
	data := adminSearchData{Query: query, Entity: entity, Sort: sortMode, Limit: limit, Cursor: offset}

	switch entity {
	case "content":
		item, err := h.contentService.GetAdmin(r.Context(), id)
		if err != nil {
			errs.Write(w, r, err)
			return
		}
		data.Detail = map[string]any{"entity": "content", "item": item}
	case "people":
		item, err := h.personService.Get(r.Context(), id)
		if err != nil {
			errs.Write(w, r, err)
			return
		}
		data.Detail = map[string]any{"entity": "people", "item": item}
	case "attributes":
		item, err := h.attributeService.Get(r.Context(), id)
		if err != nil {
			errs.Write(w, r, err)
			return
		}
		data.Detail = map[string]any{"entity": "attributes", "item": item}
	default:
		if item, err := h.contentService.GetAdmin(r.Context(), id); err == nil {
			data.Detail = map[string]any{"entity": "content", "item": item}
		} else if item, err := h.personService.Get(r.Context(), id); err == nil {
			data.Detail = map[string]any{"entity": "people", "item": item}
		} else if item, err := h.attributeService.Get(r.Context(), id); err == nil {
			data.Detail = map[string]any{"entity": "attributes", "item": item}
		} else {
			errs.Write(w, r, errs.NewNotFound("resource"))
			return
		}
	}

	writeOK(w, r, http.StatusOK, "admin detail fetched", data)
}

func (h *AdminSearchHandler) collectAllAdminContent(r *http.Request) ([]model.Movie, error) {
	items := make([]model.Movie, 0)
	var startKey map[string]types.AttributeValue
	for {
		pageItems, nextKey, err := h.contentService.GetAllAdmin(r.Context(), maxPageLimit, startKey)
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

func (h *AdminSearchHandler) collectAllPeople(r *http.Request) ([]model.Person, error) {
	items := make([]model.Person, 0)
	var startKey map[string]types.AttributeValue
	for {
		pageItems, nextKey, err := h.personService.GetAll(r.Context(), maxPageLimit, startKey)
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

func isAdminSortValid(sortMode string) bool {
	switch sortMode {
	case "recent", "latest", search.SortAlphaAsc, search.SortAlphaDesc:
		return true
	default:
		return false
	}
}

func filterAdminContent(items []model.Movie, query string) []model.Movie {
	if query == "" {
		return keepOnlyContent(items)
	}
	q := strings.ToLower(query)
	out := make([]model.Movie, 0, len(items))
	for _, it := range items {
		if !isContentEntity(it) {
			continue
		}
		if strings.Contains(strings.ToLower(it.ID), q) ||
			strings.Contains(strings.ToLower(it.Title), q) ||
			strings.Contains(strings.ToLower(it.Overview), q) ||
			strings.Contains(strings.ToLower(it.Tagline), q) ||
			strings.Contains(strings.ToLower(string(it.ContentType)), q) ||
			strings.Contains(strings.ToLower(string(it.ContentRating)), q) ||
			strings.Contains(strings.ToLower(string(it.OriginalLanguage)), q) ||
			strings.Contains(strings.ToLower(string(it.Status)), q) ||
			strings.Contains(strings.ToLower(string(it.Visibility)), q) ||
			containsEntityRef(it.Genres, q) ||
			containsEntityRef(it.Casts, q) ||
			containsEntityRef(it.Tags, q) ||
			containsEntityRef(it.MoodTags, q) ||
			containsEntityRef(it.Studios, q) {
			out = append(out, it)
		}
	}
	return out
}

func keepOnlyContent(items []model.Movie) []model.Movie {
	out := make([]model.Movie, 0, len(items))
	for _, it := range items {
		if isContentEntity(it) {
			out = append(out, it)
		}
	}
	return out
}

func isContentEntity(it model.Movie) bool {
	return (it.ContentType == model.ContentTypeMovie || it.ContentType == model.ContentTypeTV) && strings.TrimSpace(it.ID) != ""
}

func filterAdminPeople(items []model.Person, query string) []model.Person {
	q := strings.ToLower(query)
	out := make([]model.Person, 0, len(items))
	for _, it := range items {
		if it.ContentType != model.ContentTypePerson {
			continue
		}
		if q == "" ||
			strings.Contains(strings.ToLower(it.ID), q) ||
			strings.Contains(strings.ToLower(it.Name), q) ||
			strings.Contains(strings.ToLower(it.LegalName), q) ||
			strings.Contains(strings.ToLower(it.StageName), q) ||
			strings.Contains(strings.ToLower(it.Bio), q) ||
			strings.Contains(strings.ToLower(it.BirthPlace), q) ||
			strings.Contains(strings.ToLower(it.Nationality), q) ||
			strings.Contains(strings.ToLower(string(it.Gender)), q) ||
			containsStringSlice(it.Aliases, q) ||
			containsRoles(it.Roles, q) ||
			containsEntityRef(it.Tags, q) ||
			containsEntityRef(it.Categories, q) ||
			containsEntityRef(it.Specialties, q) {
			out = append(out, it)
		}
	}
	return out
}

func filterAdminAttributes(items []model.Attribute, query string, attributeTypeFilter string) []model.Attribute {
	q := strings.ToLower(query)
	filterType := strings.ToUpper(strings.TrimSpace(attributeTypeFilter))
	out := make([]model.Attribute, 0, len(items))
	for _, it := range items {
		if it.ContentType != model.ContentTypeAttribute {
			continue
		}
		if filterType != "" && !attributeContainsType(it, filterType) {
			continue
		}
		if q == "" ||
			strings.Contains(strings.ToLower(it.ID), q) ||
			strings.Contains(strings.ToLower(it.Name), q) ||
			strings.Contains(strings.ToLower(it.Logo), q) ||
			strings.Contains(strings.ToLower(it.SVG), q) ||
			containsAttributeType(it.AttributeType, q) {
			out = append(out, it)
		}
	}
	return out
}

func containsEntityRef(items []model.EntityRef, q string) bool {
	for _, it := range items {
		if strings.Contains(strings.ToLower(it.ID), q) || strings.Contains(strings.ToLower(it.Name), q) {
			return true
		}
	}
	return false
}

func containsStringSlice(items []string, q string) bool {
	for _, it := range items {
		if strings.Contains(strings.ToLower(it), q) {
			return true
		}
	}
	return false
}

func containsRoles(items []model.EntityType, q string) bool {
	for _, it := range items {
		if strings.Contains(strings.ToLower(string(it)), q) {
			return true
		}
	}
	return false
}

func containsAttributeType(items []model.AttributeType, q string) bool {
	for _, it := range items {
		if strings.Contains(strings.ToLower(string(it)), q) {
			return true
		}
	}
	return false
}

func attributeContainsType(item model.Attribute, t string) bool {
	for _, it := range item.AttributeType {
		if strings.EqualFold(string(it), t) {
			return true
		}
	}
	return false
}

func sortAdminContent(items []model.Movie, sortMode string) {
	switch sortMode {
	case search.SortAlphaAsc:
		sort.SliceStable(items, func(i, j int) bool {
			left := strings.ToLower(strings.TrimSpace(items[i].Title))
			right := strings.ToLower(strings.TrimSpace(items[j].Title))
			if left == right {
				return items[i].ID < items[j].ID
			}
			return left < right
		})
	case search.SortAlphaDesc:
		sort.SliceStable(items, func(i, j int) bool {
			left := strings.ToLower(strings.TrimSpace(items[i].Title))
			right := strings.ToLower(strings.TrimSpace(items[j].Title))
			if left == right {
				return items[i].ID > items[j].ID
			}
			return left > right
		})
	case "latest":
		sort.SliceStable(items, func(i, j int) bool {
			left := strings.TrimSpace(items[i].EffectiveReleaseDate())
			right := strings.TrimSpace(items[j].EffectiveReleaseDate())
			if left == right {
				return items[i].Audit.CreatedAt.After(items[j].Audit.CreatedAt)
			}
			if left == "" {
				return false
			}
			if right == "" {
				return true
			}
			return left > right
		})
	default:
		sort.SliceStable(items, func(i, j int) bool {
			left := items[i].Audit.CreatedAt
			right := items[j].Audit.CreatedAt
			if left.Equal(right) {
				return items[i].ID > items[j].ID
			}
			return left.After(right)
		})
	}
}

func sortAdminPeople(items []model.Person, sortMode string) {
	switch sortMode {
	case search.SortAlphaAsc:
		sort.SliceStable(items, func(i, j int) bool {
			left := strings.ToLower(strings.TrimSpace(items[i].Name))
			right := strings.ToLower(strings.TrimSpace(items[j].Name))
			if left == right {
				return items[i].ID < items[j].ID
			}
			return left < right
		})
	case search.SortAlphaDesc:
		sort.SliceStable(items, func(i, j int) bool {
			left := strings.ToLower(strings.TrimSpace(items[i].Name))
			right := strings.ToLower(strings.TrimSpace(items[j].Name))
			if left == right {
				return items[i].ID > items[j].ID
			}
			return left > right
		})
	default:
		sort.SliceStable(items, func(i, j int) bool {
			left := items[i].Audit.CreatedAt
			right := items[j].Audit.CreatedAt
			if left.Equal(right) {
				return items[i].ID > items[j].ID
			}
			return left.After(right)
		})
	}
}

func sortAdminAttributes(items []model.Attribute, sortMode string) {
	switch sortMode {
	case search.SortAlphaAsc:
		sort.SliceStable(items, func(i, j int) bool {
			left := strings.ToLower(strings.TrimSpace(items[i].Name))
			right := strings.ToLower(strings.TrimSpace(items[j].Name))
			if left == right {
				return items[i].ID < items[j].ID
			}
			return left < right
		})
	case search.SortAlphaDesc:
		sort.SliceStable(items, func(i, j int) bool {
			left := strings.ToLower(strings.TrimSpace(items[i].Name))
			right := strings.ToLower(strings.TrimSpace(items[j].Name))
			if left == right {
				return items[i].ID > items[j].ID
			}
			return left > right
		})
	default:
		sort.SliceStable(items, func(i, j int) bool {
			left := items[i].Audit.CreatedAt
			right := items[j].Audit.CreatedAt
			if left.Equal(right) {
				return items[i].ID > items[j].ID
			}
			return left.After(right)
		})
	}
}

func paginateMovies(items []model.Movie, offset int, limit int) ([]model.Movie, string) {
	if offset >= len(items) {
		return []model.Movie{}, ""
	}
	end := offset + limit
	if end > len(items) {
		end = len(items)
	}
	next := ""
	if end < len(items) {
		next = strconv.Itoa(end)
	}
	return items[offset:end], next
}

func paginatePeople(items []model.Person, offset int, limit int) ([]model.Person, string) {
	if offset >= len(items) {
		return []model.Person{}, ""
	}
	end := offset + limit
	if end > len(items) {
		end = len(items)
	}
	next := ""
	if end < len(items) {
		next = strconv.Itoa(end)
	}
	return items[offset:end], next
}

func paginateAttributes(items []model.Attribute, offset int, limit int) ([]model.Attribute, string) {
	if offset >= len(items) {
		return []model.Attribute{}, ""
	}
	end := offset + limit
	if end > len(items) {
		end = len(items)
	}
	next := ""
	if end < len(items) {
		next = strconv.Itoa(end)
	}
	return items[offset:end], next
}

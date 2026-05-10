package search

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/xanderbilla/bi8s-go/internal/model"
)

type OpenSearchConfig struct {
	Endpoint       string
	Username       string
	Password       string
	ContentIndex   string
	PeopleIndex    string
	RequestTimeout time.Duration
}

type OpenSearchProvider struct {
	endpoint     string
	username     string
	password     string
	contentIndex string
	peopleIndex  string
	httpClient   *http.Client
}

func NewOpenSearchProvider(cfg OpenSearchConfig) (*OpenSearchProvider, error) {
	ep := strings.TrimSpace(cfg.Endpoint)
	if ep == "" {
		return nil, fmt.Errorf("search endpoint is required")
	}
	if _, err := url.Parse(ep); err != nil {
		return nil, fmt.Errorf("invalid search endpoint: %w", err)
	}
	timeout := cfg.RequestTimeout
	if timeout <= 0 {
		timeout = 3 * time.Second
	}
	return &OpenSearchProvider{
		endpoint:     strings.TrimRight(ep, "/"),
		username:     cfg.Username,
		password:     cfg.Password,
		contentIndex: cfg.ContentIndex,
		peopleIndex:  cfg.PeopleIndex,
		httpClient: &http.Client{
			Timeout: timeout,
		},
	}, nil
}

func (o *OpenSearchProvider) EnsureIndexes(ctx context.Context) error {
	if err := o.ensureIndex(ctx, o.contentIndex, contentIndexMapping()); err != nil {
		return err
	}
	if err := o.ensureIndex(ctx, o.peopleIndex, peopleIndexMapping()); err != nil {
		return err
	}
	if err := o.ensureIndexMapping(ctx, o.contentIndex, contentIndexMapping()); err != nil {
		return err
	}
	return o.ensureIndexMapping(ctx, o.peopleIndex, peopleIndexMapping())
}

func (o *OpenSearchProvider) ensureIndex(ctx context.Context, index string, mapping map[string]any) error {
	headReq, err := http.NewRequestWithContext(ctx, http.MethodHead, o.endpoint+"/"+index, nil)
	if err != nil {
		return err
	}
	o.applyAuth(headReq)
	headResp, err := o.httpClient.Do(headReq)
	if err != nil {
		return err
	}
	_ = headResp.Body.Close()
	if headResp.StatusCode == http.StatusOK {
		return nil
	}
	if headResp.StatusCode != http.StatusNotFound {
		return fmt.Errorf("opensearch head index %s failed: %d", index, headResp.StatusCode)
	}
	body, err := json.Marshal(mapping)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPut, o.endpoint+"/"+index, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	o.applyAuth(req)
	resp, err := o.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode >= 300 {
		b, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("opensearch create index %s failed: %d: %s", index, resp.StatusCode, string(b))
	}
	return nil
}

func (o *OpenSearchProvider) ensureIndexMapping(ctx context.Context, index string, mapping map[string]any) error {
	body, err := json.Marshal(map[string]any{"properties": mapping["mappings"].(map[string]any)["properties"]})
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPut, o.endpoint+"/"+index+"/_mapping", bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	o.applyAuth(req)
	resp, err := o.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode >= 300 {
		b, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("opensearch update mapping %s failed: %d: %s", index, resp.StatusCode, string(b))
	}
	return nil
}

func (o *OpenSearchProvider) SearchContent(ctx context.Context, query string, contentType string, sort string, page, pageSize int) ([]model.MoviePublicList, int, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 20
	}
	body := buildContentSearchBody(query, contentType, sort, page, pageSize)

	var out searchResponse[contentDoc]
	if err := o.doJSON(ctx, http.MethodPost, "/"+o.contentIndex+"/_search", body, &out); err != nil {
		return nil, 0, err
	}
	results := make([]model.MoviePublicList, 0, len(out.Hits.Hits))
	for _, h := range out.Hits.Hits {
		s := h.Source
		results = append(results, model.MoviePublicList{
			ID:            s.ID,
			Title:         s.Title,
			BackdropPath:  s.BackdropPath,
			PosterPath:    s.PosterPath,
			ReleaseDate:   s.ReleaseDate,
			FirstAirDate:  s.FirstAirDate,
			Tags:          s.Tags,
			ContentRating: model.Rating(s.ContentRating),
			ContentType:   model.ContentType(s.ContentType),
		})
	}
	return results, out.Hits.Total.Value, nil
}

func (o *OpenSearchProvider) MoreLikeThis(ctx context.Context, contentID string, page, pageSize int) ([]model.MoviePublicList, int, error) {
	body := buildMLTBody(o.contentIndex, contentID, page, pageSize)

	var out searchResponse[contentDoc]
	if err := o.doJSON(ctx, http.MethodPost, "/"+o.contentIndex+"/_search", body, &out); err != nil {
		return nil, 0, err
	}
	results := make([]model.MoviePublicList, 0, len(out.Hits.Hits))
	for _, h := range out.Hits.Hits {
		s := h.Source
		results = append(results, model.MoviePublicList{
			ID:            s.ID,
			Title:         s.Title,
			BackdropPath:  s.BackdropPath,
			PosterPath:    s.PosterPath,
			ReleaseDate:   s.ReleaseDate,
			FirstAirDate:  s.FirstAirDate,
			Tags:          s.Tags,
			ContentRating: model.Rating(s.ContentRating),
			ContentType:   model.ContentType(s.ContentType),
		})
	}
	return results, out.Hits.Total.Value, nil
}

func (o *OpenSearchProvider) SearchPeople(ctx context.Context, query string, sort string, page, pageSize int) ([]model.SearchPersonResult, int, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 20
	}
	body := buildPeopleSearchBody(query, sort, page, pageSize)

	var out searchResponse[personDoc]
	if err := o.doJSON(ctx, http.MethodPost, "/"+o.peopleIndex+"/_search", body, &out); err != nil {
		return nil, 0, err
	}
	results := make([]model.SearchPersonResult, 0, len(out.Hits.Hits))
	for _, h := range out.Hits.Hits {
		s := h.Source
		results = append(results, model.SearchPersonResult{
			ID:           s.ID,
			ContentType:  model.ContentType(s.ContentType),
			Name:         s.Name,
			Roles:        toEntityRoles(s.Roles),
			StageName:    s.StageName,
			Bio:          s.Bio,
			Gender:       model.Gender(s.Gender),
			Verified:     s.Verified,
			ProfilePath:  s.ProfilePath,
			BackdropPath: s.BackdropPath,
			Categories:   s.Categories,
			Specialties:  s.Specialties,
		})
	}
	return results, out.Hits.Total.Value, nil
}

func (o *OpenSearchProvider) IndexContent(ctx context.Context, movie model.Movie) error {
	doc := contentDoc{
		ID:                   movie.ID,
		Title:                movie.Title,
		Overview:             movie.Overview,
		BackdropPath:         movie.BackdropPath,
		PosterPath:           movie.PosterPath,
		CreatedAt:            movie.Audit.CreatedAt.UTC().Format(time.RFC3339Nano),
		ReleaseDate:          movie.ReleaseDate,
		FirstAirDate:         movie.FirstAirDate,
		ContentRating:        string(movie.ContentRating),
		ContentType:          string(movie.ContentType),
		Tags:                 movie.Tags,
		Genres:               movie.Genres,
		Casts:                movie.Casts,
		Studios:              movie.Studios,
		MoodTags:             movie.MoodTags,
		EffectiveReleaseDate: movie.EffectiveReleaseDate(),
	}
	return o.doJSON(ctx, http.MethodPut, "/"+o.contentIndex+"/_doc/"+movie.ID, doc, nil)
}

func (o *OpenSearchProvider) DeleteContent(ctx context.Context, id string) error {
	err := o.doJSON(ctx, http.MethodDelete, "/"+o.contentIndex+"/_doc/"+id, nil, nil)
	if isNotFound(err) {
		return nil
	}
	return err
}

func (o *OpenSearchProvider) IndexPerson(ctx context.Context, person model.Person) error {
	doc := personDoc{
		ID:           person.ID,
		ContentType:  string(person.ContentType),
		Name:         person.Name,
		Roles:        fromEntityRoles(person.Roles),
		StageName:    person.StageName,
		Bio:          person.Bio,
		Gender:       string(person.Gender),
		Verified:     person.Verified,
		ProfilePath:  person.ProfilePath,
		BackdropPath: person.BackdropPath,
		Categories:   person.Categories,
		Specialties:  person.Specialties,
	}
	return o.doJSON(ctx, http.MethodPut, "/"+o.peopleIndex+"/_doc/"+person.ID, doc, nil)
}

func (o *OpenSearchProvider) DeletePerson(ctx context.Context, id string) error {
	err := o.doJSON(ctx, http.MethodDelete, "/"+o.peopleIndex+"/_doc/"+id, nil, nil)
	if isNotFound(err) {
		return nil
	}
	return err
}

func (o *OpenSearchProvider) doJSON(ctx context.Context, method, path string, in any, out any) error {
	var body io.Reader
	if in != nil {
		b, err := json.Marshal(in)
		if err != nil {
			return err
		}
		body = bytes.NewReader(b)
	}
	req, err := http.NewRequestWithContext(ctx, method, o.endpoint+path, body)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	o.applyAuth(req)

	resp, err := o.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode >= 300 {
		b, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("opensearch request failed: %d: %s", resp.StatusCode, string(b))
	}
	if out == nil {
		return nil
	}
	return json.NewDecoder(resp.Body).Decode(out)
}

func (o *OpenSearchProvider) applyAuth(req *http.Request) {
	if o.username != "" {
		req.SetBasicAuth(o.username, o.password)
	}
}

func (o *OpenSearchProvider) DocCount(ctx context.Context) (int64, error) {
	var total int64
	for _, index := range []string{o.contentIndex, o.peopleIndex} {
		var result struct {
			Count int64 `json:"count"`
		}
		if err := o.doJSON(ctx, http.MethodGet, "/"+index+"/_count", nil, &result); err != nil {
			return 0, err
		}
		total += result.Count
	}
	return total, nil
}

type searchResponse[T any] struct {
	Hits struct {
		Total struct {
			Value int `json:"value"`
		} `json:"total"`
		Hits []struct {
			Source T `json:"_source"`
		} `json:"hits"`
	} `json:"hits"`
}

type contentDoc struct {
	ID                   string            `json:"id"`
	Title                string            `json:"title"`
	Overview             string            `json:"overview"`
	BackdropPath         string            `json:"backdropPath,omitempty"`
	PosterPath           string            `json:"posterPath,omitempty"`
	CreatedAt            string            `json:"createdAt,omitempty"`
	ReleaseDate          string            `json:"releaseDate,omitempty"`
	FirstAirDate         string            `json:"firstAirDate,omitempty"`
	Tags                 []model.EntityRef `json:"tags,omitempty"`
	Genres               []model.EntityRef `json:"genres,omitempty"`
	Casts                []model.EntityRef `json:"casts,omitempty"`
	Studios              []model.EntityRef `json:"studios,omitempty"`
	MoodTags             []model.EntityRef `json:"moodTags,omitempty"`
	ContentRating        string            `json:"contentRating,omitempty"`
	ContentType          string            `json:"contentType"`
	EffectiveReleaseDate string            `json:"effectiveReleaseDate,omitempty"`
}

type personDoc struct {
	ID           string            `json:"id"`
	ContentType  string            `json:"contentType"`
	Name         string            `json:"name"`
	Roles        []string          `json:"roles"`
	StageName    string            `json:"stageName"`
	Bio          string            `json:"bio"`
	Gender       string            `json:"gender"`
	Verified     bool              `json:"verified"`
	ProfilePath  string            `json:"profilePath"`
	BackdropPath string            `json:"backdropPath"`
	Categories   []model.EntityRef `json:"categories"`
	Specialties  []model.EntityRef `json:"specialties"`
}

func buildTextQuery(query string, fields []string) map[string]any {
	q := strings.TrimSpace(query)
	if q == "" {
		return map[string]any{"match_all": map[string]any{}}
	}
	return map[string]any{
		"bool": map[string]any{
			"should": []map[string]any{
				{
					"multi_match": map[string]any{
						"query":     q,
						"fields":    fields,
						"fuzziness": "AUTO",
					},
				},
				{
					"simple_query_string": map[string]any{
						"query":            q + "*",
						"fields":           fields,
						"default_operator": "and",
					},
				},
			},
			"minimum_should_match": 1,
		},
	}
}

func buildContentSearchBody(query, contentType, sort string, page, pageSize int) map[string]any {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 20
	}

	from := (page - 1) * pageSize
	must := []map[string]any{buildTextQuery(query, []string{
		"title^6", "overview^3", "genres.name^2", "tags.name^2", "moodTags.name^2", "casts.name^2", "studios.name^2",
	})}
	filter := make([]map[string]any, 0, 1)
	if strings.TrimSpace(contentType) != "" {
		filter = append(filter, map[string]any{"term": map[string]any{"contentType": strings.ToUpper(contentType)}})
	}
	sortList := buildContentSort(sort)

	return map[string]any{
		"from": from,
		"size": pageSize,
		"query": map[string]any{
			"bool": map[string]any{
				"must":   must,
				"filter": filter,
			},
		},
		"sort": sortList,
	}
}

func buildMLTBody(contentIndex, contentID string, page, pageSize int) map[string]any {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 20
	}
	from := (page - 1) * pageSize
	return map[string]any{
		"from": from,
		"size": pageSize,
		"query": map[string]any{
			"more_like_this": map[string]any{
				"fields":          []string{"title", "overview", "genres.name", "tags.name", "moodTags.name"},
				"like":            []map[string]any{{"_index": contentIndex, "_id": contentID}},
				"min_term_freq":   1,
				"max_query_terms": 25,
				"min_doc_freq":    1,
			},
		},
		"sort": []map[string]any{
			{"_score": map[string]any{"order": "desc"}},
		},
	}
}

func buildPeopleSearchBody(query, sort string, page, pageSize int) map[string]any {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 20
	}

	from := (page - 1) * pageSize
	return map[string]any{
		"from": from,
		"size": pageSize,
		"query": buildTextQuery(query, []string{
			"name^6", "stageName^5", "bio^3", "categories.name^2", "specialties.name^2",
		}),
		"sort": buildPeopleSort(sort),
	}
}

func buildContentSort(sort string) []map[string]any {
	tokens := splitSortTokens(sort)
	if len(tokens) == 0 {

		return []map[string]any{
			{"createdAt": map[string]any{"order": "desc", "missing": "_last"}},
			{"_score": map[string]any{"order": "desc"}},
		}
	}

	sortList := make([]map[string]any, 0, len(tokens))
	for _, token := range tokens {
		switch token {
		case SortRecent:
			sortList = append(sortList, map[string]any{"createdAt": map[string]any{"order": "desc", "missing": "_last"}})
		case SortLatest:
			sortList = append(sortList, map[string]any{"effectiveReleaseDate": map[string]any{"order": "desc", "missing": "_last"}})
		case SortAlphaAsc:
			sortList = append(sortList, map[string]any{"title.keyword": map[string]any{"order": "asc", "missing": "_last"}})
		case SortAlphaDesc:
			sortList = append(sortList, map[string]any{"title.keyword": map[string]any{"order": "desc", "missing": "_last"}})
		}
	}

	return sortList
}

func buildPeopleSort(sort string) []map[string]any {
	for _, token := range splitSortTokens(sort) {
		switch token {
		case SortAlphaDesc:
			return []map[string]any{
				{"name.keyword": map[string]any{"order": "desc", "missing": "_last"}},
				{"_score": map[string]any{"order": "desc"}},
			}
		case SortAlphaAsc:
			return []map[string]any{
				{"name.keyword": map[string]any{"order": "asc", "missing": "_last"}},
				{"_score": map[string]any{"order": "desc"}},
			}
		}
	}

	return []map[string]any{
		{"name.keyword": map[string]any{"order": "asc", "missing": "_last"}},
		{"_score": map[string]any{"order": "desc"}},
	}
}

func splitSortTokens(sort string) []string {
	parts := strings.Split(strings.TrimSpace(sort), ",")
	tokens := make([]string, 0, len(parts))
	for _, part := range parts {
		token := strings.ToLower(strings.TrimSpace(part))
		if token != "" {
			tokens = append(tokens, token)
		}
	}
	return tokens
}

func contentIndexMapping() map[string]any {
	return map[string]any{
		"settings": map[string]any{
			"index": map[string]any{"number_of_shards": 1, "number_of_replicas": 0},
		},
		"mappings": map[string]any{
			"properties": map[string]any{
				"id":                   map[string]any{"type": "keyword"},
				"title":                map[string]any{"type": "text", "fields": map[string]any{"keyword": map[string]any{"type": "keyword"}}},
				"overview":             map[string]any{"type": "text"},
				"backdropPath":         map[string]any{"type": "keyword"},
				"posterPath":           map[string]any{"type": "keyword"},
				"createdAt":            map[string]any{"type": "date", "format": "strict_date_optional_time||epoch_millis"},
				"releaseDate":          map[string]any{"type": "date", "format": "strict_date_optional_time||yyyy-MM-dd"},
				"firstAirDate":         map[string]any{"type": "date", "format": "strict_date_optional_time||yyyy-MM-dd"},
				"effectiveReleaseDate": map[string]any{"type": "date", "format": "strict_date_optional_time||yyyy-MM-dd"},
				"contentRating":        map[string]any{"type": "keyword"},
				"contentType":          map[string]any{"type": "keyword"},
				"tags":                 map[string]any{"properties": map[string]any{"id": map[string]any{"type": "keyword"}, "name": map[string]any{"type": "text"}}},
				"genres":               map[string]any{"properties": map[string]any{"id": map[string]any{"type": "keyword"}, "name": map[string]any{"type": "text"}}},
				"casts":                map[string]any{"properties": map[string]any{"id": map[string]any{"type": "keyword"}, "name": map[string]any{"type": "text"}}},
				"studios":              map[string]any{"properties": map[string]any{"id": map[string]any{"type": "keyword"}, "name": map[string]any{"type": "text"}}}, "moodTags": map[string]any{"properties": map[string]any{"id": map[string]any{"type": "keyword"}, "name": map[string]any{"type": "text"}}}},
		},
	}
}

func peopleIndexMapping() map[string]any {
	return map[string]any{
		"settings": map[string]any{
			"index": map[string]any{"number_of_shards": 1, "number_of_replicas": 0},
		},
		"mappings": map[string]any{
			"properties": map[string]any{
				"id":           map[string]any{"type": "keyword"},
				"contentType":  map[string]any{"type": "keyword"},
				"name":         map[string]any{"type": "text", "fields": map[string]any{"keyword": map[string]any{"type": "keyword"}}},
				"roles":        map[string]any{"type": "keyword"},
				"stageName":    map[string]any{"type": "text"},
				"bio":          map[string]any{"type": "text"},
				"gender":       map[string]any{"type": "keyword"},
				"verified":     map[string]any{"type": "boolean"},
				"profilePath":  map[string]any{"type": "keyword"},
				"backdropPath": map[string]any{"type": "keyword"},
				"categories":   map[string]any{"properties": map[string]any{"id": map[string]any{"type": "keyword"}, "name": map[string]any{"type": "text"}}},
				"specialties":  map[string]any{"properties": map[string]any{"id": map[string]any{"type": "keyword"}, "name": map[string]any{"type": "text"}}},
			},
		},
	}
}

func fromEntityRoles(roles []model.EntityType) []string {
	out := make([]string, 0, len(roles))
	for _, r := range roles {
		out = append(out, string(r))
	}
	return out
}

func toEntityRoles(roles []string) []model.EntityType {
	out := make([]model.EntityType, 0, len(roles))
	for _, r := range roles {
		out = append(out, model.EntityType(r))
	}
	return out
}

func isNotFound(err error) bool {
	if err == nil {
		return false
	}
	return strings.Contains(err.Error(), " 404:")
}

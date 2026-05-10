package search

import "testing"

func TestBuildContentSearchBody_UsesRecentSortByDefault(t *testing.T) {
	t.Parallel()

	body := buildContentSearchBody("Angelina", "movie", "", 2, 25)

	if got := body["from"].(int); got != 25 {
		t.Fatalf("expected from=25, got %d", got)
	}
	if got := body["size"].(int); got != 25 {
		t.Fatalf("expected size=25, got %d", got)
	}

	sortList, ok := body["sort"].([]map[string]any)
	if !ok {
		t.Fatalf("expected sort to be []map[string]any")
	}
	if _, ok := sortList[0]["createdAt"]; !ok {
		t.Fatalf("expected first sort to be createdAt")
	}
	if _, ok := sortList[1]["_score"]; !ok {
		t.Fatalf("expected second sort to be _score")
	}

	query := body["query"].(map[string]any)
	boolQuery := query["bool"].(map[string]any)
	must := boolQuery["must"].([]map[string]any)
	textBool := must[0]["bool"].(map[string]any)
	should := textBool["should"].([]map[string]any)
	multiMatch := should[0]["multi_match"].(map[string]any)
	fields := multiMatch["fields"].([]string)

	foundCasts := false
	for _, field := range fields {
		if field == "casts.name^2" {
			foundCasts = true
			break
		}
	}
	if !foundCasts {
		t.Fatalf("expected casts.name^2 in content search fields")
	}

	filter := boolQuery["filter"].([]map[string]any)
	term := filter[0]["term"].(map[string]any)
	if got := term["contentType"].(string); got != "MOVIE" {
		t.Fatalf("expected contentType filter MOVIE, got %q", got)
	}
}

func TestBuildContentSearchBody_NoContentTypeFilter(t *testing.T) {
	t.Parallel()

	body := buildContentSearchBody("Angelina", "", SortRecent, 1, 20)
	query := body["query"].(map[string]any)
	boolQuery := query["bool"].(map[string]any)
	filter := boolQuery["filter"].([]map[string]any)
	if len(filter) != 0 {
		t.Fatalf("expected empty filter, got %d", len(filter))
	}
}

func TestBuildContentSearchBody_LatestSortUsesReleaseDate(t *testing.T) {
	t.Parallel()

	body := buildContentSearchBody("Angelina", "movie", SortLatest, 1, 20)
	sortList := body["sort"].([]map[string]any)
	if _, ok := sortList[0]["effectiveReleaseDate"]; !ok {
		t.Fatalf("expected first sort to be effectiveReleaseDate")
	}
}

func TestBuildContentSearchBody_RecentSortDoesNotAppendScoreFallback(t *testing.T) {
	t.Parallel()

	body := buildContentSearchBody("Angelina", "movie", SortRecent, 1, 20)
	sortList := body["sort"].([]map[string]any)
	if _, ok := sortList[0]["createdAt"]; !ok {
		t.Fatalf("expected explicit recent sort to use createdAt")
	}
	if len(sortList) != 1 {
		t.Fatalf("expected explicit sort list length 1, got %d", len(sortList))
	}
}

func TestBuildContentSearchBody_AlphaDescUsesTitleKeywordAsOnlySort(t *testing.T) {
	t.Parallel()

	body := buildContentSearchBody("Angelina", "movie", SortAlphaDesc, 1, 20)
	sortList := body["sort"].([]map[string]any)
	if _, ok := sortList[0]["title.keyword"]; !ok {
		t.Fatalf("expected first sort to be title.keyword")
	}
	if len(sortList) != 1 {
		t.Fatalf("expected explicit sort list length 1, got %d", len(sortList))
	}
}

func TestBuildContentSearchBody_AlphaDescUsesTitleKeyword(t *testing.T) {
	t.Parallel()

	body := buildContentSearchBody("Angelina", "movie", SortAlphaDesc, 1, 20)
	sortList := body["sort"].([]map[string]any)
	titleSort, ok := sortList[0]["title.keyword"].(map[string]any)
	if !ok {
		t.Fatalf("expected first sort to be title.keyword")
	}
	if got := titleSort["order"].(string); got != "desc" {
		t.Fatalf("expected title.keyword order desc, got %q", got)
	}
}

func TestBuildPeopleSearchBody_AlphaDescUsesNameKeyword(t *testing.T) {
	t.Parallel()

	body := buildPeopleSearchBody("Angelina", SortAlphaDesc, 1, 20)
	sortList := body["sort"].([]map[string]any)
	nameSort, ok := sortList[0]["name.keyword"].(map[string]any)
	if !ok {
		t.Fatalf("expected first sort to be name.keyword")
	}
	if got := nameSort["order"].(string); got != "desc" {
		t.Fatalf("expected name.keyword order desc, got %q", got)
	}
}

func TestBuildPeopleSearchBody_IgnoresTemporalAndUsesAlphaWhenPresent(t *testing.T) {
	t.Parallel()

	body := buildPeopleSearchBody("Angelina", "latest,alpha_asc", 1, 20)
	sortList := body["sort"].([]map[string]any)
	nameSort, ok := sortList[0]["name.keyword"].(map[string]any)
	if !ok {
		t.Fatalf("expected first sort to be name.keyword")
	}
	if got := nameSort["order"].(string); got != "asc" {
		t.Fatalf("expected name.keyword order asc, got %q", got)
	}
}

func TestBuildMLTBody(t *testing.T) {
	t.Parallel()

	body := buildMLTBody("bi8s-content", "content-123", 2, 10)

	if got := body["from"].(int); got != 10 {
		t.Fatalf("expected from=10, got %d", got)
	}
	if got := body["size"].(int); got != 10 {
		t.Fatalf("expected size=10, got %d", got)
	}

	query := body["query"].(map[string]any)
	mlt := query["more_like_this"].(map[string]any)

	fields := mlt["fields"].([]string)
	foundTitle := false
	foundMoodTags := false
	for _, f := range fields {
		if f == "title" {
			foundTitle = true
		}
		if f == "moodTags.name" {
			foundMoodTags = true
		}
	}
	if !foundTitle {
		t.Fatalf("expected 'title' in MLT fields")
	}
	if !foundMoodTags {
		t.Fatalf("expected 'moodTags.name' in MLT fields")
	}

	like := mlt["like"].([]map[string]any)
	if len(like) != 1 {
		t.Fatalf("expected 1 like doc, got %d", len(like))
	}
	if like[0]["_id"] != "content-123" {
		t.Fatalf("expected _id=content-123, got %v", like[0]["_id"])
	}
	if like[0]["_index"] != "bi8s-content" {
		t.Fatalf("expected _index=bi8s-content, got %v", like[0]["_index"])
	}
}

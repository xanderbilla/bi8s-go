package http

import (
	"testing"

	"github.com/xanderbilla/bi8s-go/internal/model"
)

func TestToAttributePublicDetail(t *testing.T) {
	src := &model.Attribute{
		ID:            "attr-1",
		Name:          "Action",
		AttributeType: []model.AttributeType{model.AttributeTypeGenre},
		ContentType:   model.ContentTypeAttribute,
		Active:        true,
	}
	got := toAttributePublicDetail(src)
	if got.ID != src.ID || got.Name != src.Name || got.Active != src.Active {
		t.Fatalf("toAttributePublicDetail mismatch: %+v", got)
	}
	if len(got.AttributeType) != 1 || got.AttributeType[0] != model.AttributeTypeGenre {
		t.Fatalf("AttributeType not copied: %+v", got.AttributeType)
	}
	if got.ContentType != src.ContentType {
		t.Fatalf("ContentType mismatch: %v", got.ContentType)
	}
}

func TestToMoviePublicDetailCopiesScalarFields(t *testing.T) {
	src := &model.Movie{
		ID:               "m-1",
		Title:            "Title",
		Overview:         "Overview",
		BackdropPath:     "/b",
		PosterPath:       "/p",
		Adult:            true,
		ContentRating:    "PG",
		OriginalLanguage: "en",
		Runtime:          120,
		Tagline:          "tagline",
	}
	got := toMoviePublicDetail(src)
	if got.ID != src.ID ||
		got.Title != src.Title ||
		got.Overview != src.Overview ||
		got.BackdropPath != src.BackdropPath ||
		got.PosterPath != src.PosterPath ||
		got.Adult != src.Adult ||
		got.ContentRating != src.ContentRating ||
		got.OriginalLanguage != src.OriginalLanguage ||
		got.Runtime != src.Runtime ||
		got.Tagline != src.Tagline {
		t.Fatalf("toMoviePublicDetail mismatch: %+v", got)
	}
}

func TestToPersonPublicDetailCopiesScalarFields(t *testing.T) {
	src := &model.Person{
		ID:           "p-1",
		Name:         "Jane",
		StageName:    "JD",
		Bio:          "bio",
		BirthPlace:   "Earth",
		Nationality:  "USA",
		Gender:       "F",
		Verified:     true,
		Active:       true,
		ProfilePath:  "/prof",
		BackdropPath: "/back",
	}
	got := toPersonPublicDetail(src)
	if got.ID != src.ID ||
		got.Name != src.Name ||
		got.StageName != src.StageName ||
		got.Bio != src.Bio ||
		got.BirthPlace != src.BirthPlace ||
		got.Nationality != src.Nationality ||
		got.Gender != src.Gender ||
		got.Verified != src.Verified ||
		got.Active != src.Active ||
		got.ProfilePath != src.ProfilePath ||
		got.BackdropPath != src.BackdropPath {
		t.Fatalf("toPersonPublicDetail mismatch: %+v", got)
	}
}

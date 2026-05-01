package http

import (
	"errors"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/xanderbilla/bi8s-go/internal/errs"
	"github.com/xanderbilla/bi8s-go/internal/model"
	"github.com/xanderbilla/bi8s-go/internal/validation"
)

func ParseMovieFromForm(formValues url.Values) (model.Movie, error) {
	var releaseDate string
	if rd := strings.TrimSpace(formValues.Get("release_date")); rd != "" {

		t, err := time.Parse("2006-01-02", rd)
		if err != nil {
			return model.Movie{}, errors.New("release_date must be in YYYY-MM-DD format")
		}
		releaseDate = t.Format("2006-01-02")
	}

	var firstAirDate string
	if fad := strings.TrimSpace(formValues.Get("first_air_date")); fad != "" {

		t, err := time.Parse("2006-01-02", fad)
		if err != nil {
			return model.Movie{}, errors.New("first_air_date must be in YYYY-MM-DD format")
		}
		firstAirDate = t.Format("2006-01-02")
	}

	runtime, _ := strconv.Atoi(strings.TrimSpace(formValues.Get("runtime")))
	adult, _ := strconv.ParseBool(strings.TrimSpace(formValues.Get("adult")))

	genres := parseEntityRefs(formValues, "genres")
	casts := parseEntityRefs(formValues, "casts")
	tags := parseEntityRefs(formValues, "tags")
	moodTags := parseEntityRefs(formValues, "mood_tags")
	studios := parseEntityRefs(formValues, "studios")
	originCountry := parseStringArray(formValues, "origin_country")

	movie := model.Movie{
		ID:               strings.TrimSpace(formValues.Get("id")),
		Title:            strings.TrimSpace(formValues.Get("title")),
		Overview:         strings.TrimSpace(formValues.Get("overview")),
		ReleaseDate:      releaseDate,
		FirstAirDate:     firstAirDate,
		Adult:            adult,
		ContentRating:    model.Rating(strings.TrimSpace(formValues.Get("content_rating"))),
		OriginalLanguage: model.OriginalLanguage(strings.TrimSpace(formValues.Get("original_language"))),
		ContentType:      model.ContentType(strings.TrimSpace(formValues.Get("content_type"))),
		Runtime:          runtime,
		Status:           model.Status(strings.TrimSpace(formValues.Get("status"))),
		Tagline:          strings.TrimSpace(formValues.Get("tagline")),
		Visibility:       model.Visibility(strings.TrimSpace(formValues.Get("visibility"))),
		Genres:           genres,
		Casts:            casts,
		Tags:             tags,
		MoodTags:         moodTags,
		Studios:          studios,
		OriginCountry:    originCountry,
	}

	if err := validation.ValidateStruct(movie); err != nil {
		return model.Movie{}, errs.NewValidation(validation.FieldErrors(err))
	}

	return movie, nil
}

func parseEntityRefs(formValues url.Values, fieldName string) []model.EntityRef {
	value := strings.TrimSpace(formValues.Get(fieldName))
	if value == "" {
		return nil
	}

	var refs []model.EntityRef
	pairs := strings.Split(value, ",")
	for _, pair := range pairs {
		parts := strings.Split(strings.TrimSpace(pair), ":")
		var id, name string

		switch len(parts) {
		case 1:
			id = strings.TrimSpace(parts[0])
		case 2:
			id = strings.TrimSpace(parts[0])
			name = strings.TrimSpace(parts[1])
		}

		if id != "" {
			refs = append(refs, model.EntityRef{
				ID:   id,
				Name: name,
			})
		}
	}
	return refs
}

func parseStringArray(formValues url.Values, fieldName string) []string {
	value := strings.TrimSpace(formValues.Get(fieldName))
	if value == "" {
		return nil
	}

	var result []string
	items := strings.Split(value, ",")
	for _, item := range items {
		trimmed := strings.TrimSpace(item)
		if trimmed != "" {
			result = append(result, trimmed)
		}
	}
	return result
}

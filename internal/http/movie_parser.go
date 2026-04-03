package http

import (
	"errors"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/xanderbilla/bi8s-go/internal/model"
	"github.com/xanderbilla/bi8s-go/internal/utils"
	"github.com/xanderbilla/bi8s-go/internal/validation"
)

// ParseMovieFromForm builds a Movie struct explicitly from parsed multipart text entries.
// It actively executes structural validations isolating bad user input immediately at the router edge.
func ParseMovieFromForm(formValues url.Values) (model.Movie, error) {
	var releaseDate string
	if rd := strings.TrimSpace(formValues.Get("release_date")); rd != "" {
		// Parse YYYY-MM-DD
		t, err := time.Parse("2006-01-02", rd)
		if err != nil {
			return model.Movie{}, errors.New("release_date must be in YYYY-MM-DD format")
		}
		releaseDate = t.Format("2006-01-02")
	}

	var firstAirDate string
	if fad := strings.TrimSpace(formValues.Get("first_air_date")); fad != "" {
		// Parse YYYY-MM-DD
		t, err := time.Parse("2006-01-02", fad)
		if err != nil {
			return model.Movie{}, errors.New("first_air_date must be in YYYY-MM-DD format")
		}
		firstAirDate = t.Format("2006-01-02")
	}

	runtime, _ := strconv.Atoi(strings.TrimSpace(formValues.Get("runtime")))
	adult, _ := strconv.ParseBool(strings.TrimSpace(formValues.Get("adult")))

	// Parse array fields
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
		return model.Movie{}, err
	}

	return movie, nil
}

// parseEntityRefs parses comma-separated IDs into EntityRef slices
// For casts, genres, tags, moodTags, and studios: only IDs are accepted (names will be fetched from respective tables)
// Format: "id1,id2,id3"
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

		// For casts, genres, tags, moodTags, categories, specialties, and studios: only accept IDs (names will be fetched from tables)
		if fieldName == "casts" || fieldName == "genres" || fieldName == "tags" || fieldName == "mood_tags" || fieldName == "categories" || fieldName == "specialties" || fieldName == "studios" {
			if len(parts) == 1 {
				id = strings.TrimSpace(parts[0])
				name = "" // Will be populated by service layer
			} else if len(parts) == 2 {
				id = strings.TrimSpace(parts[0])
				name = "" // Ignore provided name, will be fetched from table
			}
		} else {
			// For other fields, use existing logic
			if len(parts) == 2 {
				id = strings.TrimSpace(parts[0])
				name = strings.TrimSpace(parts[1])
			} else if len(parts) == 1 {
				// Auto-generate ID if not provided
				id = utils.GenerateNumericID()
				name = strings.TrimSpace(parts[0])
			}
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

// parseStringArray parses comma-separated strings into a string slice
// Format: "value1,value2,value3"
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

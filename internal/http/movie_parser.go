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
	var releaseDate utils.Date
	if rd := strings.TrimSpace(formValues.Get("release_date")); rd != "" {
		// Parse YYYY-MM-DD
		t, err := time.Parse("2006-01-02", rd)
		if err != nil {
			return model.Movie{}, errors.New("release_date must be in YYYY-MM-DD format")
		}
		releaseDate = utils.Date{Time: t}
	}

	var firstAirDate utils.Date
	if fad := strings.TrimSpace(formValues.Get("first_air_date")); fad != "" {
		// Parse YYYY-MM-DD
		t, err := time.Parse("2006-01-02", fad)
		if err != nil {
			return model.Movie{}, errors.New("first_air_date must be in YYYY-MM-DD format")
		}
		firstAirDate = utils.Date{Time: t}
	}

	voteAverage, _ := strconv.ParseFloat(strings.TrimSpace(formValues.Get("vote_average")), 64)
	voteCount, _ := strconv.Atoi(strings.TrimSpace(formValues.Get("vote_count")))
	popularity, _ := strconv.ParseFloat(strings.TrimSpace(formValues.Get("popularity")), 64)
	runtime, _ := strconv.Atoi(strings.TrimSpace(formValues.Get("runtime")))
	adult, _ := strconv.ParseBool(strings.TrimSpace(formValues.Get("adult")))

	// Parse array fields
	genres := parseEntityRefs(formValues, "genres")
	casts := parseEntityRefsWithImage(formValues, "casts")
	tags := parseEntityRefs(formValues, "tags")
	moodTags := parseEntityRefs(formValues, "mood_tags")
	studios := parseCompanies(formValues, "studios")
	originCountry := parseStringArray(formValues, "origin_country")

	movie := model.Movie{
		ID:               strings.TrimSpace(formValues.Get("id")),
		Title:            strings.TrimSpace(formValues.Get("title")),
		Overview:         strings.TrimSpace(formValues.Get("overview")),
		ReleaseDate:      releaseDate,
		FirstAirDate:     firstAirDate,
		VoteAverage:      voteAverage,
		VoteCount:        voteCount,
		Popularity:       popularity,
		Adult:            adult,
		ContentRating:    model.Rating(strings.TrimSpace(formValues.Get("content_rating"))),
		OriginalLanguage: model.OriginalLanguage(strings.TrimSpace(formValues.Get("original_language"))),
		ContentType:      model.MediaType(strings.TrimSpace(formValues.Get("content_type"))),
		Runtime:          runtime,
		Status:           model.Status(strings.TrimSpace(formValues.Get("status"))),
		Tagline:          strings.TrimSpace(formValues.Get("tagline")),
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

// parseEntityRefs parses comma-separated "id:name" or "name" pairs into EntityRef slices
// If ID is not provided, it auto-generates a 6-digit numeric ID
// Format: "id1:name1,id2:name2" or "name1,name2"
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
		
		if len(parts) == 2 {
			id = strings.TrimSpace(parts[0])
			name = strings.TrimSpace(parts[1])
		} else if len(parts) == 1 {
			// Auto-generate ID if not provided
			id = utils.GenerateNumericID()
			name = strings.TrimSpace(parts[0])
		}
		
		if name != "" {
			refs = append(refs, model.EntityRef{
				ID:   id,
				Name: name,
			})
		}
	}
	return refs
}

// parseEntityRefsWithImage parses comma-separated "id:name:image", "id:name", or "name" tuples into EntityRefImg slices
// If ID is not provided, it auto-generates a 6-digit numeric ID
// Format: "id1:name1:image1,id2:name2:image2" or "name1:image1,name2:image2" or "name1,name2"
func parseEntityRefsWithImage(formValues url.Values, fieldName string) []model.EntityRefImg {
	value := strings.TrimSpace(formValues.Get(fieldName))
	if value == "" {
		return nil
	}

	var refs []model.EntityRefImg
	tuples := strings.Split(value, ",")
	for _, tuple := range tuples {
		parts := strings.Split(strings.TrimSpace(tuple), ":")
		var id, name, image string
		
		if len(parts) == 3 {
			id = strings.TrimSpace(parts[0])
			name = strings.TrimSpace(parts[1])
			image = strings.TrimSpace(parts[2])
		} else if len(parts) == 2 {
			// Check if first part looks like an ID (6 digits) or a name
			first := strings.TrimSpace(parts[0])
			if len(first) == 6 && isNumeric(first) {
				id = first
				name = strings.TrimSpace(parts[1])
			} else {
				// Auto-generate ID, treat as name:image
				id = utils.GenerateNumericID()
				name = first
				image = strings.TrimSpace(parts[1])
			}
		} else if len(parts) == 1 {
			// Auto-generate ID if only name provided
			id = utils.GenerateNumericID()
			name = strings.TrimSpace(parts[0])
		}
		
		if name != "" {
			refs = append(refs, model.EntityRefImg{
				ID:           id,
				Name:         name,
				CoverPicture: image,
			})
		}
	}
	return refs
}

// isNumeric checks if a string contains only digits
func isNumeric(s string) bool {
	for _, c := range s {
		if c < '0' || c > '9' {
			return false
		}
	}
	return true
}

// parseCompanies parses comma-separated "id:name:image", "id:name", or "name" tuples into Company slices
// If ID is not provided, it auto-generates a 6-digit numeric ID
// Format: "id1:name1:image1,id2:name2:image2" or "name1:image1,name2:image2" or "name1,name2"
func parseCompanies(formValues url.Values, fieldName string) []model.Company {
	value := strings.TrimSpace(formValues.Get(fieldName))
	if value == "" {
		return nil
	}

	var companies []model.Company
	tuples := strings.Split(value, ",")
	for _, tuple := range tuples {
		parts := strings.Split(strings.TrimSpace(tuple), ":")
		var id, name, image string
		
		if len(parts) == 3 {
			id = strings.TrimSpace(parts[0])
			name = strings.TrimSpace(parts[1])
			image = strings.TrimSpace(parts[2])
		} else if len(parts) == 2 {
			// Check if first part looks like an ID (6 digits) or a name
			first := strings.TrimSpace(parts[0])
			if len(first) == 6 && isNumeric(first) {
				id = first
				name = strings.TrimSpace(parts[1])
			} else {
				// Auto-generate ID, treat as name:image
				id = utils.GenerateNumericID()
				name = first
				image = strings.TrimSpace(parts[1])
			}
		} else if len(parts) == 1 {
			// Auto-generate ID if only name provided
			id = utils.GenerateNumericID()
			name = strings.TrimSpace(parts[0])
		}
		
		if name != "" {
			companies = append(companies, model.Company{
				ID:           id,
				Name:         name,
				CoverPicture: image,
			})
		}
	}
	return companies
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

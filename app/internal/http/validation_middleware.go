package http

import (
	"net/http"
	"regexp"
	"strings"
	"unicode/utf8"

	"github.com/go-chi/chi/v5"
	"github.com/xanderbilla/bi8s-go/internal/errs"
)

type URLParamValidator struct {
	ParamName   string
	Required    bool
	MaxLength   int
	MinLength   int
	Pattern     *regexp.Regexp
	AllowedVals []string
	ErrorMsg    string
}

func ValidateURLParams(validators ...URLParamValidator) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			for _, validator := range validators {
				value := chi.URLParam(r, validator.ParamName)

				if validator.Required && value == "" {
					writeParamValidation(w, r, validator, "required parameter missing")
					return
				}

				if value == "" {
					continue
				}

				runes := utf8.RuneCountInString(value)
				if validator.MaxLength > 0 && runes > validator.MaxLength {
					writeParamValidation(w, r, validator, "parameter too long")
					return
				}

				if validator.MinLength > 0 && runes < validator.MinLength {
					writeParamValidation(w, r, validator, "parameter too short")
					return
				}

				if validator.Pattern != nil && !validator.Pattern.MatchString(value) {
					writeParamValidation(w, r, validator, "parameter format invalid")
					return
				}

				if len(validator.AllowedVals) > 0 {
					allowed := false
					for _, allowedVal := range validator.AllowedVals {
						if value == allowedVal {
							allowed = true
							break
						}
					}
					if !allowed {
						writeParamValidation(w, r, validator, "parameter value not allowed")
						return
					}
				}
			}

			next.ServeHTTP(w, r)
		})
	}
}

func writeParamValidation(w http.ResponseWriter, r *http.Request, v URLParamValidator, reason string) {
	errs.Write(w, r, errs.NewValidation(map[string]any{
		"param":  v.ParamName,
		"reason": reason,
		"hint":   v.ErrorMsg,
	}))
}

var (
	AlphanumericPattern = regexp.MustCompile(`^[a-zA-Z0-9_-]+$`)

	UUIDPattern = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`)

	JobIDPattern = regexp.MustCompile(`^job_[a-zA-Z0-9-]+$`)

	SlugPattern = regexp.MustCompile(`^[a-z0-9]+(?:-[a-z0-9]+)*$`)

	YearPattern = regexp.MustCompile(`^[12][0-9]{3}$`)
)

var MovieIDValidator = URLParamValidator{
	ParamName: "movieId",
	Required:  true,
	MaxLength: 100,
	MinLength: 1,
	Pattern:   AlphanumericPattern,
	ErrorMsg:  "Invalid movie ID format",
}

var PersonIDValidator = URLParamValidator{
	ParamName: "peopleId",
	Required:  true,
	MaxLength: 100,
	MinLength: 1,
	Pattern:   AlphanumericPattern,
	ErrorMsg:  "Invalid person ID format",
}

var AttributeIDValidator = URLParamValidator{
	ParamName: "attributeId",
	Required:  true,
	MaxLength: 100,
	MinLength: 1,
	Pattern:   AlphanumericPattern,
	ErrorMsg:  "Invalid attribute ID format",
}

var ConsumerAttributeIDValidator = URLParamValidator{
	ParamName: "id",
	Required:  true,
	MaxLength: 100,
	MinLength: 1,
	Pattern:   AlphanumericPattern,
	ErrorMsg:  "Invalid attribute ID format",
}

var JobIDValidator = URLParamValidator{
	ParamName: "jobId",
	Required:  true,
	MaxLength: 50,
	MinLength: 5,
	Pattern:   JobIDPattern,
	ErrorMsg:  "Invalid job ID format",
}

var ContentTypeValidator = URLParamValidator{
	ParamName:   "contentType",
	Required:    true,
	AllowedVals: []string{"movie", "tv"},
	ErrorMsg:    "Invalid content type, must be 'movie' or 'tv'",
}

var ContentIDValidator = URLParamValidator{
	ParamName: "contentId",
	Required:  true,
	MaxLength: 100,
	MinLength: 1,
	Pattern:   AlphanumericPattern,
	ErrorMsg:  "Invalid content ID format",
}

func ValidateContentType(contentType string) (string, bool) {
	switch strings.ToLower(contentType) {
	case "movie", "movies":
		return "movie", true
	case "person", "persons", "people":
		return "person", true
	default:
		return "", false
	}
}

func ValidateID(id string) error {
	if id == "" {
		return ErrInvalidID("ID cannot be empty")
	}

	if len(id) > 100 {
		return ErrInvalidID("ID too long (max 100 characters)")
	}

	if !AlphanumericPattern.MatchString(id) {
		return ErrInvalidID("ID contains invalid characters (only alphanumeric, underscore, hyphen allowed)")
	}

	if strings.Contains(id, "..") || strings.Contains(id, "/") || strings.Contains(id, "\\") {
		return ErrInvalidID("ID contains path traversal characters")
	}

	return nil
}

func ErrInvalidID(msg string) error {
	return &ValidationError{Field: "id", Message: msg}
}

type ValidationError struct {
	Field   string
	Message string
}

func (e *ValidationError) Error() string {
	return e.Field + ": " + e.Message
}

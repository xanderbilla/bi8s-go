package http

import (
	"errors"
	"net/url"
	"strconv"
	"strings"

	"github.com/xanderbilla/bi8s-go/internal/repository"
	"github.com/xanderbilla/bi8s-go/internal/validation"
)

// ParseMovieFromForm builds a Movie struct explicitly from parsed multipart text entries.
// It actively executes structural validations isolating bad user input immediately at the router edge.
func ParseMovieFromForm(formValues url.Values) (repository.Movie, error) {
	year, err := strconv.Atoi(strings.TrimSpace(formValues.Get("year")))
	if err != nil {
		return repository.Movie{}, errors.New("year must be a valid integer")
	}

	movie := repository.Movie{
		ID:          strings.TrimSpace(formValues.Get("id")),
		Title:       strings.TrimSpace(formValues.Get("title")),
		Description: strings.TrimSpace(formValues.Get("description")),
		Performer:   strings.TrimSpace(formValues.Get("performer")),
		Year:        year,
	}

	if err := validation.ValidateStruct(movie); err != nil {
		return repository.Movie{}, err
	}

	return movie, nil
}

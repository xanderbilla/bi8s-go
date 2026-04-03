package validation

import (
	"context"

	"github.com/xanderbilla/bi8s-go/internal/errs"
	"github.com/xanderbilla/bi8s-go/internal/model"
)

// PersonRepository defines the minimal interface needed for performer validation.
type PersonRepository interface {
	Get(ctx context.Context, id string) (*model.Person, error)
}

// ValidateAndPopulateCasts validates that all cast members exist in the person table
// and populates their names from the database.
func ValidateAndPopulateCasts(ctx context.Context, casts []model.EntityRef, personRepo PersonRepository) ([]model.EntityRef, error) {
	if len(casts) == 0 {
		return casts, nil
	}

	validatedCasts := make([]model.EntityRef, len(casts))
	for i, cast := range casts {
		person, err := personRepo.Get(ctx, cast.ID)
		if err != nil {
			return nil, err
		}
		if person == nil {
			return nil, errs.ErrPerformerNotFound(cast.ID)
		}
		// Populate cast name from person table
		validatedCasts[i] = model.EntityRef{
			ID:   cast.ID,
			Name: person.Name,
		}
	}

	return validatedCasts, nil
}

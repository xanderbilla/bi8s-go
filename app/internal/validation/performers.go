package validation

import (
	"context"

	"github.com/xanderbilla/bi8s-go/internal/errs"
	"github.com/xanderbilla/bi8s-go/internal/model"
)

type PersonGetter interface {
	Get(ctx context.Context, id string) (*model.Person, error)
}

func ValidateAndPopulateCasts(
	ctx context.Context,
	casts []model.EntityRef,
	personRepo PersonGetter,
) ([]model.EntityRef, error) {
	if len(casts) == 0 {
		return casts, nil
	}

	validated := make([]model.EntityRef, len(casts))

	for i, cast := range casts {
		person, err := personRepo.Get(ctx, cast.ID)
		if err != nil {
			return nil, err
		}
		if person == nil {
			return nil, &errs.PerformerNotFoundError{ID: cast.ID}
		}
		validated[i] = model.EntityRef{
			ID:   cast.ID,
			Name: person.Name,
		}
	}

	return validated, nil
}

package validation

import (
	"context"

	"github.com/xanderbilla/bi8s-go/internal/errs"
	"github.com/xanderbilla/bi8s-go/internal/model"
	"github.com/xanderbilla/bi8s-go/internal/repository"
)

func ValidateAndPopulateAttributes(
	ctx context.Context,
	attributes []model.EntityRef,
	expectedType model.AttributeType,
	attributeRepo repository.AttributeRepository,
) ([]model.EntityRef, error) {
	if len(attributes) == 0 {
		return attributes, nil
	}

	validated := make([]model.EntityRef, 0, len(attributes))

	for _, attr := range attributes {
		attribute, err := attributeRepo.Get(ctx, attr.ID)
		if err != nil {
			return nil, err
		}

		if attribute == nil || !hasAttributeType(attribute.AttributeType, expectedType) {
			return nil, &errs.AttributeNotFoundError{
				ID:           attr.ID,
				ExpectedType: string(expectedType),
			}
		}

		validated = append(validated, model.EntityRef{
			ID:   attr.ID,
			Name: attribute.Name,
		})
	}

	return validated, nil
}

type AttributeGroup struct {
	Refs         []model.EntityRef
	ExpectedType model.AttributeType
	Assign       func([]model.EntityRef)
}

func ValidateAndPopulateAttributeGroups(
	ctx context.Context,
	attributeRepo repository.AttributeRepository,
	groups ...AttributeGroup,
) error {
	for _, g := range groups {
		validated, err := ValidateAndPopulateAttributes(ctx, g.Refs, g.ExpectedType, attributeRepo)
		if err != nil {
			return err
		}
		if g.Assign != nil {
			g.Assign(validated)
		}
	}
	return nil
}

func hasAttributeType(attributeTypes []model.AttributeType, expectedType model.AttributeType) bool {
	for _, t := range attributeTypes {
		if t == expectedType {
			return true
		}
	}
	return false
}

package validation

import (
	"context"
	"fmt"

	"github.com/xanderbilla/bi8s-go/internal/model"
	"github.com/xanderbilla/bi8s-go/internal/repository"
)

// ValidateAndPopulateAttributes validates that attribute IDs exist in the attribute table
// and have the correct attributeType, then populates their names.
func ValidateAndPopulateAttributes(
	ctx context.Context,
	attributes []model.EntityRef,
	expectedType model.AttributeType,
	attributeRepo repository.AttributeRepository,
) ([]model.EntityRef, error) {
	if len(attributes) == 0 {
		return attributes, nil
	}

	validatedAttributes := make([]model.EntityRef, 0, len(attributes))

	for _, attr := range attributes {
		// Get attribute from database
		attribute, err := attributeRepo.Get(ctx, attr.ID)
		if err != nil {
			return nil, fmt.Errorf("failed to fetch attribute with id '%s': %w", attr.ID, err)
		}

		if attribute == nil {
			return nil, fmt.Errorf("attribute with id '%s' not found", attr.ID)
		}

		// Check if attribute has the expected type
		if !hasAttributeType(attribute.AttributeType, expectedType) {
			return nil, fmt.Errorf("attribute with id '%s' does not have type '%s'", attr.ID, expectedType)
		}

		// Populate name from database
		validatedAttributes = append(validatedAttributes, model.EntityRef{
			ID:   attr.ID,
			Name: attribute.Name,
		})
	}

	return validatedAttributes, nil
}

// hasAttributeType checks if the attribute has the expected type in its attributeType array.
func hasAttributeType(attributeTypes []model.AttributeType, expectedType model.AttributeType) bool {
	for _, attrType := range attributeTypes {
		if attrType == expectedType {
			return true
		}
	}
	return false
}

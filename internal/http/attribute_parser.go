package http

import (
	"net/url"
	"strings"

	"github.com/xanderbilla/bi8s-go/internal/model"
	"github.com/xanderbilla/bi8s-go/internal/validation"
)

// ParseAttributeFromForm builds an Attribute struct from parsed multipart form data.
func ParseAttributeFromForm(formValues url.Values) (model.Attribute, error) {
	// Parse attribute types (comma-separated)
	attributeTypes := parseAttributeTypes(formValues, "attribute_type")

	attribute := model.Attribute{
		Name:          strings.TrimSpace(formValues.Get("name")),
		AttributeType: attributeTypes,
		// Active is set to true by default in service layer
	}

	if err := validation.ValidateStruct(attribute); err != nil {
		return model.Attribute{}, err
	}

	return attribute, nil
}

// parseAttributeTypes parses comma-separated attribute type values into AttributeType slice
func parseAttributeTypes(formValues url.Values, fieldName string) []model.AttributeType {
	value := strings.TrimSpace(formValues.Get(fieldName))
	if value == "" {
		return nil
	}

	var attributeTypes []model.AttributeType
	items := strings.Split(value, ",")
	for _, item := range items {
		trimmed := strings.TrimSpace(item)
		if trimmed != "" {
			attributeTypes = append(attributeTypes, model.AttributeType(trimmed))
		}
	}
	return attributeTypes
}

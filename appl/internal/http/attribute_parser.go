package http

import (
	"net/url"
	"strings"

	"github.com/xanderbilla/bi8s-go/internal/errs"
	"github.com/xanderbilla/bi8s-go/internal/model"
	"github.com/xanderbilla/bi8s-go/internal/validation"
)

func ParseAttributeFromForm(formValues url.Values) (model.Attribute, error) {

	attributeTypes := parseAttributeTypes(formValues, "attribute_type")

	attribute := model.Attribute{
		Name:          strings.TrimSpace(formValues.Get("name")),
		AttributeType: attributeTypes,
	}

	if err := validation.ValidateStruct(attribute); err != nil {
		return model.Attribute{}, errs.NewValidation(validation.FieldErrors(err))
	}

	return attribute, nil
}

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

package validation

import "github.com/go-playground/validator/v10"

// validate is a shared validator instance used across request validation.
// WithRequiredStructEnabled enforces required tags on nested struct values.
var validate = validator.New(validator.WithRequiredStructEnabled())

// ValidateStruct runs tag-based validation rules on a struct value.
func ValidateStruct(value interface{}) error {
	return validate.Struct(value)
}

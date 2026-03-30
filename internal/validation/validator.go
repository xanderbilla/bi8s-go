package validation

import (
	"time"

	"github.com/go-playground/validator/v10"
	"github.com/xanderbilla/bi8s-go/internal/utils"
)

// WithRequiredStructEnabled enforces required tags on nested struct values.
var validate = validator.New(validator.WithRequiredStructEnabled())

func init() {
	// customdate confirms util.Date correctly unwraps time payloads securely natively.
	_ = validate.RegisterValidation("customdate", func(fl validator.FieldLevel) bool {
		date, ok := fl.Field().Interface().(utils.Date)
		if !ok {
			return false // Abort poorly cast validation attempts aggressively.
		}
		// Require custom dates be actual configured dates and not standard zeroed structures.
		return !date.IsZero()
	})

	// daterange validates that dates are between 2006-01-01 and today
	_ = validate.RegisterValidation("daterange", func(fl validator.FieldLevel) bool {
		var t time.Time
		if d, ok := fl.Field().Interface().(utils.Date); ok {
			t = d.Time
		} else if tVal, ok := fl.Field().Interface().(time.Time); ok {
			t = tVal
		} else {
			return false
		}

		if t.IsZero() {
			return true // Allow empty dates, omitempty will handle it
		}

		minDate := time.Date(2006, 1, 1, 0, 0, 0, 0, time.UTC)
		today := time.Now().UTC().Truncate(24 * time.Hour)
		input := t.UTC().Truncate(24 * time.Hour)

		if input.Before(minDate) {
			return false
		}

		if input.After(today) {
			return false
		}

		return true
	})
}

// ValidateStruct runs tag-based validation rules on a struct value.
func ValidateStruct(value interface{}) error {
	return validate.Struct(value)
}

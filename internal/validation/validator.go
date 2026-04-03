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
		} else if sVal, ok := fl.Field().Interface().(string); ok {
			if sVal == "" {
				return true
			}
			parsedTime, err := time.Parse("2006-01-02", sVal)
			if err != nil {
				return false
			}
			t = parsedTime
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

	// age18plus validates that the person is at least 18 years old
	_ = validate.RegisterValidation("age18plus", func(fl validator.FieldLevel) bool {
		var birthDate time.Time
		if sVal, ok := fl.Field().Interface().(string); ok {
			if sVal == "" {
				return true // Allow empty, omitempty will handle it
			}
			parsedTime, err := time.Parse("2006-01-02", sVal)
			if err != nil {
				return false
			}
			birthDate = parsedTime
		} else {
			return false
		}

		if birthDate.IsZero() {
			return true
		}

		today := time.Now().UTC()
		age := today.Year() - birthDate.Year()

		// Adjust age if birthday hasn't occurred this year
		if today.Month() < birthDate.Month() || (today.Month() == birthDate.Month() && today.Day() < birthDate.Day()) {
			age--
		}

		return age >= 18
	})
}

// ValidateStruct runs tag-based validation rules on a struct value.
func ValidateStruct(value interface{}) error {
	return validate.Struct(value)
}

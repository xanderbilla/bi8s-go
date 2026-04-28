package validation

import (
	"time"

	"github.com/go-playground/validator/v10"
	"github.com/xanderbilla/bi8s-go/internal/model"
)

var validate = validator.New(validator.WithRequiredStructEnabled())

func init() {

	_ = validate.RegisterValidation("customdate", func(fl validator.FieldLevel) bool {
		date, ok := fl.Field().Interface().(model.Date)
		if !ok {
			return false
		}

		return !date.IsZero()
	})

	_ = validate.RegisterValidation("daterange", func(fl validator.FieldLevel) bool {
		var t time.Time
		if d, ok := fl.Field().Interface().(model.Date); ok {
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
			return true
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

	_ = validate.RegisterValidation("age18plus", func(fl validator.FieldLevel) bool {
		var birthDate time.Time
		if sVal, ok := fl.Field().Interface().(string); ok {
			if sVal == "" {
				return true
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

		if today.Month() < birthDate.Month() || (today.Month() == birthDate.Month() && today.Day() < birthDate.Day()) {
			age--
		}

		return age >= 18
	})
}

func ValidateStruct(value any) error {
	return validate.Struct(value)
}

type FieldError struct {
	Field   string `json:"field"`
	Code    string `json:"code"`
	Message string `json:"message,omitempty"`
}

func FieldErrors(err error) []FieldError {
	if err == nil {
		return nil
	}
	ves, ok := err.(validator.ValidationErrors)
	if !ok {
		return nil
	}
	out := make([]FieldError, 0, len(ves))
	for _, ve := range ves {
		out = append(out, FieldError{
			Field:   ve.Field(),
			Code:    ve.Tag(),
			Message: ve.Error(),
		})
	}
	return out
}

package validation

import (
	"time"

	"github.com/go-playground/validator/v10"
	"github.com/xanderbilla/bi8s-go/internal/utils"
)

type Validator struct {
	validate *validator.Validate
}

func New() *Validator {
	v := validator.New()

	v.RegisterValidation("daterange", validateDateRange)

	return &Validator{
		validate: v,
	}
}

func (v *Validator) Validate(i any) error {
	return v.validate.Struct(i)
}

func validateDateRange(fl validator.FieldLevel) bool {
	date, ok := fl.Field().Interface().(utils.Date)
	if !ok {
		return false
	}

	if date.Time.IsZero() {
		return true
	}

	minDate := time.Date(2006, 1, 1, 0, 0, 0, 0, time.UTC)
	today := time.Now().UTC().Truncate(24 * time.Hour)
	input := date.Time.UTC().Truncate(24 * time.Hour)

	if input.Before(minDate) {
		return false
	}

	if input.After(today) {
		return false
	}

	return true
}
package http

import (
	"errors"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/xanderbilla/bi8s-go/internal/errs"
	"github.com/xanderbilla/bi8s-go/internal/model"
	"github.com/xanderbilla/bi8s-go/internal/validation"
)

func ParsePersonFromForm(formValues url.Values) (model.Person, error) {
	var birthDate string
	if bd := strings.TrimSpace(formValues.Get("birth_date")); bd != "" {
		t, err := time.Parse("2006-01-02", bd)
		if err != nil {
			return model.Person{}, errors.New("birth_date must be in YYYY-MM-DD format")
		}
		birthDate = t.Format("2006-01-02")
	}

	height, _ := strconv.Atoi(strings.TrimSpace(formValues.Get("height")))
	debutYear, _ := strconv.Atoi(strings.TrimSpace(formValues.Get("debut_year")))
	active, _ := strconv.ParseBool(strings.TrimSpace(formValues.Get("active")))
	verified, _ := strconv.ParseBool(strings.TrimSpace(formValues.Get("verified")))

	roles := parseRoles(formValues, "roles")

	measurements := parseMeasurements(formValues)

	tags := parseEntityRefs(formValues, "tags")
	categories := parseEntityRefs(formValues, "categories")
	specialties := parseEntityRefs(formValues, "specialties")

	person := model.Person{
		Name:         strings.TrimSpace(formValues.Get("name")),
		Roles:        roles,
		StageName:    strings.TrimSpace(formValues.Get("stage_name")),
		Bio:          strings.TrimSpace(formValues.Get("bio")),
		BirthDate:    birthDate,
		BirthPlace:   strings.TrimSpace(formValues.Get("birth_place")),
		Nationality:  strings.TrimSpace(formValues.Get("nationality")),
		Gender:       model.Gender(strings.TrimSpace(formValues.Get("gender"))),
		Height:       height,
		Verified:     verified,
		Active:       active,
		DebutYear:    debutYear,
		CareerStatus: model.CareerStatus(strings.TrimSpace(formValues.Get("career_status"))),
		Measurements: measurements,
		Tags:         tags,
		Categories:   categories,
		Specialties:  specialties,
	}

	if err := validation.ValidateStruct(person); err != nil {
		return model.Person{}, errs.NewValidation(validation.FieldErrors(err))
	}

	return person, nil
}

func parseRoles(formValues url.Values, fieldName string) []model.EntityType {
	value := strings.TrimSpace(formValues.Get(fieldName))
	if value == "" {
		return nil
	}

	var roles []model.EntityType
	items := strings.Split(value, ",")
	for _, item := range items {
		trimmed := strings.TrimSpace(item)
		if trimmed != "" {
			roles = append(roles, model.EntityType(trimmed))
		}
	}
	return roles
}

func parseMeasurements(formValues url.Values) model.Measurements {
	bust, _ := strconv.Atoi(strings.TrimSpace(formValues.Get("measurements_bust")))
	waist, _ := strconv.Atoi(strings.TrimSpace(formValues.Get("measurements_waist")))
	hips, _ := strconv.Atoi(strings.TrimSpace(formValues.Get("measurements_hips")))

	return model.Measurements{
		Bust:      bust,
		Waist:     waist,
		Hips:      hips,
		Unit:      strings.TrimSpace(formValues.Get("measurements_unit")),
		BodyType:  strings.TrimSpace(formValues.Get("measurements_body_type")),
		EyeColor:  strings.TrimSpace(formValues.Get("measurements_eye_color")),
		HairColor: strings.TrimSpace(formValues.Get("measurements_hair_color")),
	}
}

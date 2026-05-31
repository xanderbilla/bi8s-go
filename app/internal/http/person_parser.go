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
	weight, _ := strconv.Atoi(strings.TrimSpace(formValues.Get("weight")))
	debutYear, _ := strconv.Atoi(strings.TrimSpace(formValues.Get("debut_year")))
	active, _ := strconv.ParseBool(strings.TrimSpace(formValues.Get("active")))
	verified, _ := strconv.ParseBool(strings.TrimSpace(formValues.Get("verified")))

	roles := parseRoles(formValues, "roles")
	aliases := parseStringSlice(formValues, "aliases")

	measurements := parseMeasurements(formValues)
	career := parseCareer(formValues)
	socialPresence := parseSocialPresenceEntries(formValues)
	sourceMetadata := parseSourceMetadata(formValues)

	tags := parseEntityRefs(formValues, "tags")
	categories := parseEntityRefs(formValues, "categories")
	specialties := parseEntityRefs(formValues, "specialties")

	person := model.Person{
		Name:           strings.TrimSpace(formValues.Get("name")),
		LegalName:      strings.TrimSpace(formValues.Get("legal_name")),
		Roles:          roles,
		StageName:      strings.TrimSpace(formValues.Get("stage_name")),
		Bio:            clampMaxChars(formValues.Get("bio"), 150),
		BirthDate:      birthDate,
		BirthPlace:     strings.TrimSpace(formValues.Get("birth_place")),
		Nationality:    strings.TrimSpace(formValues.Get("nationality")),
		Gender:         model.Gender(strings.TrimSpace(formValues.Get("gender"))),
		Height:         height,
		Weight:         weight,
		Aliases:        aliases,
		Verified:       verified,
		Active:         active,
		DebutYear:      debutYear,
		CareerStatus:   model.CareerStatus(strings.TrimSpace(formValues.Get("career_status"))),
		Measurements:   measurements,
		Career:         career,
		SocialPresence: socialPresence,
		SourceMetadata: sourceMetadata,
		Tags:           tags,
		Categories:     categories,
		Specialties:    specialties,
	}

	if err := validation.ValidateStruct(person); err != nil {
		return model.Person{}, errs.NewValidation(validation.FieldErrors(err))
	}

	if person.Gender == model.GenderMale {
		var fieldErrs []validation.FieldError
		if measurements.Bust != 0 {
			fieldErrs = append(fieldErrs, validation.FieldError{Field: "measurements.bust", Code: "invalid_for_gender", Message: "bust is not applicable for male"})
		}
		if measurements.Waist != 0 {
			fieldErrs = append(fieldErrs, validation.FieldError{Field: "measurements.waist", Code: "invalid_for_gender", Message: "waist is not applicable for male"})
		}
		if measurements.Hips != 0 {
			fieldErrs = append(fieldErrs, validation.FieldError{Field: "measurements.hips", Code: "invalid_for_gender", Message: "hips is not applicable for male"})
		}
		if len(fieldErrs) > 0 {
			return model.Person{}, errs.NewValidation(fieldErrs)
		}
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

func parseStringSlice(formValues url.Values, fieldName string) []string {
	value := strings.TrimSpace(formValues.Get(fieldName))
	if value == "" {
		return nil
	}
	var result []string
	for _, item := range strings.Split(value, ",") {
		if trimmed := strings.TrimSpace(item); trimmed != "" {
			result = append(result, trimmed)
		}
	}
	return result
}

func parseCareer(formValues url.Values) *model.PersonCareer {
	startYear, _ := strconv.Atoi(strings.TrimSpace(formValues.Get("career_start_year")))
	previousProfession := strings.TrimSpace(formValues.Get("career_previous_profession"))
	knownFor := parseStringSlice(formValues, "career_known_for")

	if startYear == 0 && previousProfession == "" && len(knownFor) == 0 {
		return nil
	}
	return &model.PersonCareer{
		StartYear:          startYear,
		PreviousProfession: previousProfession,
		KnownFor:           knownFor,
	}
}

// parseSocialPresenceEntries parses indexed social presence form fields.
// Each entry uses the prefix "social[N]" with sub-fields:
//   - social[N][platform_id] (required)
//   - social[N][username]    (optional)
//   - social[N][verified]    (optional bool, default false)
//   - social[N][available]   (optional bool, default false)
func parseSocialPresenceEntries(formValues url.Values) []model.SocialPresenceEntry {
	var entries []model.SocialPresenceEntry
	for i := 0; ; i++ {
		pfx := "social[" + strconv.Itoa(i) + "]"
		platformID := strings.TrimSpace(formValues.Get(pfx + "[platform_id]"))
		if platformID == "" {
			break
		}
		verified, _ := strconv.ParseBool(strings.TrimSpace(formValues.Get(pfx + "[verified]")))
		available, _ := strconv.ParseBool(strings.TrimSpace(formValues.Get(pfx + "[available]")))
		entries = append(entries, model.SocialPresenceEntry{
			PlatformID: platformID,
			Username:   strings.TrimSpace(formValues.Get(pfx + "[username]")),
			Verified:   verified,
			Available:  available,
		})
	}
	return entries
}

func parseSourceMetadata(formValues url.Values) *model.PersonSourceMetadata {
	confidenceStr := strings.TrimSpace(formValues.Get("sourceMetadata_confidence"))
	sources := parseStringSlice(formValues, "sourceMetadata_sources")
	notes := strings.TrimSpace(formValues.Get("sourceMetadata_notes"))
	if confidenceStr == "" && len(sources) == 0 && notes == "" {
		return nil
	}
	confidence, _ := strconv.ParseFloat(confidenceStr, 64)
	return &model.PersonSourceMetadata{
		ConfidenceScore: confidence,
		Sources:         sources,
		Notes:           notes,
	}
}

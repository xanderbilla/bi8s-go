package model

// Rating represents content rating classifications.
type Rating string

const (
	Rating18Plus Rating = "18_PLUS"
	Rating21Plus Rating = "21_PLUS"
)

// ContentType represents the type of content.
type ContentType string

const (
	ContentTypeMovie      ContentType = "MOVIE"
	ContentTypeTV         ContentType = "TV"
	ContentTypePerson     ContentType = "PERSON"
	ContentTypeAttribute  ContentType = "ATTRIBUTE"
)

// AttributeType represents the type of attribute.
type AttributeType string

const (
	AttributeTypeTag        AttributeType = "TAG"
	AttributeTypeMood       AttributeType = "MOOD"
	AttributeTypeGenre      AttributeType = "GENRE"
	AttributeTypeCategory   AttributeType = "CATEGORY"
	AttributeTypeSpeciality AttributeType = "SPECIALITY"
	AttributeTypeStudio     AttributeType = "STUDIO"
)

// Status represents the production/release status of content.
type Status string

const (
	StatusRumored         Status = "RUMORED"
	StatusPlanned         Status = "PLANNED"
	StatusInProduction    Status = "IN_PRODUCTION"
	StatusPostProduction  Status = "POST_PRODUCTION"
	StatusReleased        Status = "RELEASED"
	StatusEnded           Status = "ENDED"
	StatusReturningSeries Status = "RETURNING_SERIES"
	StatusCanceled        Status = "CANCELED"
	StatusPilot           Status = "PILOT"
)

// Visibility represents the visibility status of content.
type Visibility string

const (
	VisibilityPublic  Visibility = "PUBLIC"
	VisibilityPrivate Visibility = "PRIVATE"
)

// OriginalLanguage represents supported language codes.
type OriginalLanguage string

const (
	LanguageEN OriginalLanguage = "en"
	LanguageHI OriginalLanguage = "hi"
	LanguageJA OriginalLanguage = "ja"
	LanguageKO OriginalLanguage = "ko"
	LanguageFR OriginalLanguage = "fr"
	LanguageES OriginalLanguage = "es"
)

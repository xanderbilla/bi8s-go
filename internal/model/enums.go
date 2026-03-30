package model

// Rating represents content rating classifications.
type Rating string

const (
	Rating18Plus Rating = "18_PLUS"
	Rating21Plus Rating = "21_PLUS"
)

// MediaType represents the type of media content.
type MediaType string

const (
	MediaTypeMovie  MediaType = "MOVIE"
	MediaTypeTV     MediaType = "TV"
	MediaTypePerson MediaType = "PERSON"
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

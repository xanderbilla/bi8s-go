package model

type Rating string

const (
	Rating18Plus Rating = "18_PLUS"
	Rating21Plus Rating = "21_PLUS"
)

type ContentType string

const (
	ContentTypeMovie     ContentType = "MOVIE"
	ContentTypeTV        ContentType = "TV"
	ContentTypePerson    ContentType = "PERSON"
	ContentTypeAttribute ContentType = "ATTRIBUTE"
)

func (ct ContentType) ToPath() string {
	switch ct {
	case ContentTypeMovie:
		return "movies"
	case ContentTypeTV:
		return "tv"
	case ContentTypePerson:
		return "persons"
	default:
		return "movies"
	}
}

func ParseContentType(s string) (ContentType, bool) {
	switch s {
	case "movie", "MOVIE":
		return ContentTypeMovie, true
	case "tv", "TV":
		return ContentTypeTV, true
	default:
		return "", false
	}
}

type AttributeType string

const (
	AttributeTypeTag        AttributeType = "TAG"
	AttributeTypeMood       AttributeType = "MOOD"
	AttributeTypeGenre      AttributeType = "GENRE"
	AttributeTypeCategory   AttributeType = "CATEGORY"
	AttributeTypeSpeciality AttributeType = "SPECIALITY"
	AttributeTypeStudio     AttributeType = "STUDIO"
)

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

type Visibility string

const (
	VisibilityPublic  Visibility = "PUBLIC"
	VisibilityPrivate Visibility = "PRIVATE"
)

type OriginalLanguage string

const (
	LanguageEN OriginalLanguage = "en"
	LanguageHI OriginalLanguage = "hi"
	LanguageJA OriginalLanguage = "ja"
	LanguageKO OriginalLanguage = "ko"
	LanguageFR OriginalLanguage = "fr"
	LanguageES OriginalLanguage = "es"
)

type AssetType string

const (
	AssetTypeTrailer AssetType = "TRAILER"
	AssetTypeTeaser  AssetType = "TEASER"
	AssetTypeClip    AssetType = "CLIP"
	AssetTypePromo   AssetType = "PROMO"
	AssetTypeBTS     AssetType = "BTS"
)

func (a AssetType) IsValid() bool {
	switch a {
	case AssetTypeTrailer, AssetTypeTeaser, AssetTypeClip, AssetTypePromo, AssetTypeBTS:
		return true
	default:
		return false
	}
}

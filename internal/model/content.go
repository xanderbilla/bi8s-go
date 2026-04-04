// Package model contains domain models and business entities.
package model

// Content represents the complete movie/TV show domain model.
// It includes all metadata, relationships, and content information.
// This is a shared struct that can be used for both MOVIE and TV content types.
type Content struct {
	ID               string           `json:"id" dynamodbav:"id" validate:"omitempty,min=1,max=64"`
	Title            string           `json:"title,omitempty" dynamodbav:"title,omitempty" validate:"omitempty,min=1,max=128"`
	Overview         string           `json:"overview" dynamodbav:"overview" validate:"required,min=1,max=1000"`
	BackdropPath     string           `json:"backdropPath,omitempty" dynamodbav:"backdropPath,omitempty" validate:"omitempty,max=512"`
	PosterPath       string           `json:"posterPath,omitempty" dynamodbav:"posterPath,omitempty" validate:"omitempty,max=512"`
	ReleaseDate      string           `json:"releaseDate,omitempty" dynamodbav:"releaseDate,omitempty" validate:"omitempty,daterange"`
	FirstAirDate     string           `json:"firstAirDate,omitempty" dynamodbav:"firstAirDate,omitempty" validate:"omitempty,daterange"`
	Adult            bool             `json:"adult,omitempty" dynamodbav:"adult,omitempty"`
	ContentRating    Rating           `json:"contentRating,omitempty" dynamodbav:"contentRating,omitempty" validate:"omitempty,oneof=18_PLUS 21_PLUS"`
	OriginalLanguage OriginalLanguage `json:"originalLanguage" dynamodbav:"originalLanguage" validate:"required,oneof=en hi ja ko fr es"`
	Genres           []EntityRef      `json:"genres,omitempty" dynamodbav:"genres,omitempty" validate:"omitempty,dive"`
	Casts            []EntityRef      `json:"casts,omitempty" dynamodbav:"casts,omitempty" validate:"omitempty,dive"`
	CastIds          []string         `json:"-" dynamodbav:"castIds,omitempty"` // For querying by cast
	Tags             []EntityRef      `json:"tags,omitempty" dynamodbav:"tags,omitempty" validate:"omitempty,dive"`
	AttributeIds     []string         `json:"-" dynamodbav:"attributeIds,omitempty"` // For querying by attributes (genres, tags, moodTags)
	ContentType      ContentType      `json:"contentType,omitempty" dynamodbav:"contentType,omitempty" validate:"omitempty,oneof=MOVIE TV"`
	OriginCountry    []string         `json:"originCountry,omitempty" dynamodbav:"originCountry,omitempty" validate:"omitempty,dive,min=2,max=8"`
	MoodTags         []EntityRef      `json:"moodTags,omitempty" dynamodbav:"moodTags,omitempty" validate:"omitempty,dive"`
	Runtime          int              `json:"runtime,omitempty" dynamodbav:"runtime,omitempty" validate:"omitempty,gte=0"`
	Status           Status           `json:"status,omitempty" dynamodbav:"status,omitempty" validate:"omitempty,oneof=RUMORED PLANNED IN_PRODUCTION POST_PRODUCTION RELEASED ENDED RETURNING_SERIES CANCELED PILOT"`
	Tagline          string           `json:"tagline,omitempty" dynamodbav:"tagline,omitempty" validate:"omitempty,max=255"`
	Studios          []EntityRef      `json:"studios,omitempty" dynamodbav:"studios,omitempty" validate:"omitempty,dive"`
	Visibility       Visibility       `json:"visibility,omitempty" dynamodbav:"visibility,omitempty" validate:"omitempty,oneof=PUBLIC PRIVATE"`
	Assets           []Asset          `json:"assets,omitempty" dynamodbav:"assets,omitempty" validate:"omitempty,dive"`
	Stats            ContentStats     `json:"stats,omitempty" dynamodbav:"stats,omitempty"`
	Audit            Audit            `json:"audit,omitempty" dynamodbav:"audit,omitempty"`
}

// Asset represents a video asset (trailer, teaser, clip, etc.)
type Asset struct {
	Type AssetType `json:"asset" dynamodbav:"asset" validate:"required,oneof=TRAILER TEASER CLIP PROMO BTS"`
	Keys []string  `json:"key" dynamodbav:"key" validate:"required,min=1,dive,min=1"`
}

// ContentStats represents statistics for content (movies/TV shows).
type ContentStats struct {
	TotalViews    int64   `json:"totalViews" dynamodbav:"totalViews"`
	TotalLikes    int64   `json:"totalLikes" dynamodbav:"totalLikes"`
	AverageRating float64 `json:"averageRating" dynamodbav:"averageRating" validate:"gte=0,lte=10"`
}

// ContentPublicList represents the public fields returned in GET all content endpoint.
type ContentPublicList struct {
	ID            string      `json:"id"`
	Title         string      `json:"title,omitempty"`
	BackdropPath  string      `json:"backdropPath,omitempty"`
	PosterPath    string      `json:"posterPath,omitempty"`
	ReleaseDate   string      `json:"releaseDate,omitempty"`
	Tags          []EntityRef `json:"tags,omitempty"`
	ContentRating Rating      `json:"contentRating,omitempty"`
	ContentType   ContentType `json:"contentType,omitempty"`
	Assets        []Asset     `json:"assets,omitempty"`
}

// ContentPublicDetail represents the public fields returned in GET single content endpoint.
type ContentPublicDetail struct {
	ID               string           `json:"id"`
	Title            string           `json:"title,omitempty"`
	Overview         string           `json:"overview"`
	BackdropPath     string           `json:"backdropPath,omitempty"`
	PosterPath       string           `json:"posterPath,omitempty"`
	ReleaseDate      string           `json:"releaseDate,omitempty"`
	FirstAirDate     string           `json:"firstAirDate,omitempty"`
	Adult            bool             `json:"adult,omitempty"`
	ContentRating    Rating           `json:"contentRating,omitempty"`
	OriginalLanguage OriginalLanguage `json:"originalLanguage"`
	Genres           []EntityRef      `json:"genres,omitempty"`
	Casts            []EntityRef      `json:"casts,omitempty"`
	Tags             []EntityRef      `json:"tags,omitempty"`
	ContentType      ContentType      `json:"contentType,omitempty"`
	OriginCountry    []string         `json:"originCountry,omitempty"`
	MoodTags         []EntityRef      `json:"moodTags,omitempty"`
	Runtime          int              `json:"runtime,omitempty"`
	Status           Status           `json:"status,omitempty"`
	Tagline          string           `json:"tagline,omitempty"`
	Studios          []EntityRef      `json:"studios,omitempty"`
	Assets           []Asset          `json:"assets,omitempty"`
}

// ContentsByPersonList represents content returned when querying by person ID.
type ContentsByPersonList struct {
	ID           string `json:"id"`
	Title        string `json:"title,omitempty"`
	BackdropPath string `json:"backdropPath,omitempty"`
}

// BannerContent represents content returned for banner display.
type BannerContent struct {
	ID            string  `json:"id"`
	BackdropPath  string  `json:"backdropPath,omitempty"`
	Title         string  `json:"title,omitempty"`
	Overview      string  `json:"overview,omitempty"`
	ContentRating Rating  `json:"contentRating,omitempty"`
	Assets        []Asset `json:"assets,omitempty"`
}

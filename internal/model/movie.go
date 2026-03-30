// Package model contains domain models and business entities.
// These models are independent of storage implementation and can be used
// across different layers of the application.
package model

import "github.com/xanderbilla/bi8s-go/internal/utils"

// Movie represents the complete movie/TV show domain model.
// It includes all metadata, relationships, and content information.
type Movie struct {
	ID                  string               `json:"id" dynamodbav:"id" validate:"omitempty,min=1,max=64"`
	Title               string               `json:"title,omitempty" dynamodbav:"title,omitempty" validate:"omitempty,min=1,max=128"`
	Overview            string               `json:"overview" dynamodbav:"overview" validate:"required,min=1,max=1000"`
	BackdropPath        string               `json:"backdrop_path,omitempty" dynamodbav:"backdrop_path,omitempty" validate:"omitempty,max=512"`
	PosterPath          string               `json:"poster_path,omitempty" dynamodbav:"poster_path,omitempty" validate:"omitempty,max=512"`
	ReleaseDate         utils.Date           `json:"release_date,omitempty" dynamodbav:"release_date,omitempty" validate:"omitempty,daterange"`
	FirstAirDate        utils.Date           `json:"first_air_date,omitempty" dynamodbav:"first_air_date,omitempty" validate:"omitempty,daterange"`
	VoteAverage         float64              `json:"vote_average" dynamodbav:"vote_average" validate:"gte=0,lte=10"`
	VoteCount           int                  `json:"vote_count" dynamodbav:"vote_count" validate:"gte=0"`
	Popularity          float64              `json:"popularity" dynamodbav:"popularity" validate:"gte=0"`
	Adult               bool                 `json:"adult,omitempty" dynamodbav:"adult,omitempty"`
	Ratings             Rating               `json:"ratings,omitempty" dynamodbav:"ratings,omitempty" validate:"omitempty,oneof=18_PLUS 21_PLUS"`
	OriginalLanguage    OriginalLanguage     `json:"original_language" dynamodbav:"original_language" validate:"required,oneof=en hi ja ko fr es"`
	Genres              []EntityRef          `json:"genres,omitempty" dynamodbav:"genres,omitempty" validate:"omitempty,dive"`
	Casts               []EntityRefImg       `json:"casts,omitempty" dynamodbav:"casts,omitempty" validate:"omitempty,dive"`
	Tags                []EntityRef          `json:"tags,omitempty" dynamodbav:"tags,omitempty" validate:"omitempty,dive"`
	MediaType           MediaType            `json:"media_type,omitempty" dynamodbav:"media_type,omitempty" validate:"omitempty,oneof=MOVIE TV PERSON"`
	OriginCountry       []string             `json:"origin_country,omitempty" dynamodbav:"origin_country,omitempty" validate:"omitempty,dive,min=2,max=8"`
	MoodTags            []EntityRef          `json:"mood_tags,omitempty" dynamodbav:"mood_tags,omitempty" validate:"omitempty,dive"`
	Runtime             int                  `json:"runtime,omitempty" dynamodbav:"runtime,omitempty" validate:"omitempty,gte=0"`
	Status              Status               `json:"status,omitempty" dynamodbav:"status,omitempty" validate:"omitempty,oneof=RUMORED PLANNED IN_PRODUCTION POST_PRODUCTION RELEASED ENDED RETURNING_SERIES CANCELED PILOT"`
	Tagline             string               `json:"tagline,omitempty" dynamodbav:"tagline,omitempty" validate:"omitempty,max=255"`
	ProductionCompanies []Company            `json:"production_companies,omitempty" dynamodbav:"production_companies,omitempty" validate:"omitempty,dive"`
	Audit               Audit                `json:"audit" dynamodbav:"audit"`
}

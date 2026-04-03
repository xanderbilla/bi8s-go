// Package model contains domain models and business entities.
// This file contains movie-specific types that use the shared Content struct.
package model

// Movie is a type alias for Content, used for movie-specific operations.
// Currently uses all fields from Content. In the future, this can be extended
// with movie-specific fields if needed.
type Movie = Content

// MovieStats is a type alias for ContentStats.
type MovieStats = ContentStats

// MoviePublicList is a type alias for ContentPublicList.
type MoviePublicList = ContentPublicList

// MoviePublicDetail is a type alias for ContentPublicDetail.
type MoviePublicDetail = ContentPublicDetail

// MoviesByPersonList is a type alias for ContentsByPersonList.
type MoviesByPersonList = ContentsByPersonList

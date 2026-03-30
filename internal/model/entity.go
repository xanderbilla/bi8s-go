package model

import "time"

// EntityRef represents a generic reference to an entity with ID and Name.
// Used for genres, casts, tags, mood tags, etc.
type EntityRef struct {
	ID   string `json:"id" dynamodbav:"id" validate:"required,min=1,max=64"`
	Name string `json:"name" dynamodbav:"name" validate:"required,min=1,max=128"`
}

// EntityRefImg extends EntityRef with an optional cover picture.
// Used for entities that have associated images like creators.
type EntityRefImg struct {
	ID           string `json:"id" dynamodbav:"id" validate:"required,min=1,max=64"`
	Name         string `json:"name" dynamodbav:"name" validate:"required,min=1,max=128"`
	CoverPicture string `json:"cover_picture,omitempty" dynamodbav:"cover_picture,omitempty" validate:"omitempty,max=512"`
}

// Company represents a production company with optional cover picture.
type Company struct {
	ID           string `json:"id" dynamodbav:"id" validate:"required,min=1,max=64"`
	Name         string `json:"name" dynamodbav:"name" validate:"required,min=1,max=128"`
	CoverPicture string `json:"cover_picture,omitempty" dynamodbav:"cover_picture,omitempty" validate:"omitempty,max=512"`
}

// Audit contains audit trail information for tracking entity lifecycle.
type Audit struct {
	CreatedAt time.Time  `json:"created_at" dynamodbav:"created_at"`
	UpdatedAt *time.Time `json:"updated_at,omitempty" dynamodbav:"updated_at,omitempty"`
	DeletedAt *time.Time `json:"deleted_at,omitempty" dynamodbav:"deleted_at,omitempty"`
	IsDeleted bool       `json:"is_deleted" dynamodbav:"is_deleted"`
}

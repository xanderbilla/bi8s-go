package model

import "time"

// EntityRef represents a generic reference to an entity with ID and Name.
// Used for genres, casts, tags, mood tags, studios, etc.
type EntityRef struct {
	ID   string `json:"id" dynamodbav:"id" validate:"required,min=1,max=64"`
	Name string `json:"name" dynamodbav:"name" validate:"omitempty,min=1,max=128"`
}

// Audit contains audit trail information for tracking entity lifecycle.
type Audit struct {
	CreatedAt time.Time  `json:"createdAt" dynamodbav:"createdAt"`
	CreatedBy string     `json:"createdBy,omitempty" dynamodbav:"createdBy,omitempty"`
	UpdatedAt *time.Time `json:"updatedAt,omitempty" dynamodbav:"updatedAt,omitempty"`
	UpdatedBy string     `json:"updatedBy,omitempty" dynamodbav:"updatedBy,omitempty"`
	Version   int        `json:"version" dynamodbav:"version"`
	DeletedAt *time.Time `json:"deletedAt,omitempty" dynamodbav:"deletedAt,omitempty"`
	IsDeleted bool       `json:"isDeleted" dynamodbav:"isDeleted"`
}

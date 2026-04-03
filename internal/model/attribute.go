// Package model contains domain models and business entities.
package model

// Attribute represents a reusable attribute like tags, genres, categories, etc.
type Attribute struct {
	ID            string          `json:"id" dynamodbav:"id" validate:"omitempty,min=1,max=64"`
	Name          string          `json:"name" dynamodbav:"name" validate:"required,min=1,max=128"`
	AttributeType []AttributeType `json:"attributeType" dynamodbav:"attributeType" validate:"required,min=1,dive,oneof=TAG MOOD GENRE CATEGORY SPECIALITY STUDIO"`
	ContentType   ContentType     `json:"contentType" dynamodbav:"contentType" validate:"omitempty,oneof=ATTRIBUTE"`
	Active        bool            `json:"active" dynamodbav:"active"`
	Audit         Audit           `json:"audit" dynamodbav:"audit"`
}

// AttributePublicDetail represents the public fields returned in GET single attribute endpoint.
type AttributePublicDetail struct {
	ID            string          `json:"id"`
	Name          string          `json:"name"`
	AttributeType []AttributeType `json:"attributeType"`
	ContentType   ContentType     `json:"contentType"`
	Active        bool            `json:"active"`
}

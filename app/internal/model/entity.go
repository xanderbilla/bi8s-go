package model

import "time"

type EntityRef struct {
	ID   string `json:"id" dynamodbav:"id" validate:"required,min=1,max=64"`
	Name string `json:"name" dynamodbav:"name" validate:"omitempty,min=1,max=128"`
}

type Audit struct {
	CreatedAt time.Time  `json:"createdAt" dynamodbav:"createdAt"`
	CreatedBy string     `json:"createdBy,omitempty" dynamodbav:"createdBy,omitempty"`
	UpdatedAt *time.Time `json:"updatedAt,omitempty" dynamodbav:"updatedAt,omitempty"`
	UpdatedBy string     `json:"updatedBy,omitempty" dynamodbav:"updatedBy,omitempty"`
	Version   int        `json:"version" dynamodbav:"version"`
	DeletedAt *time.Time `json:"deletedAt,omitempty" dynamodbav:"deletedAt,omitempty"`
}

func (a Audit) IsDeleted() bool { return a.DeletedAt != nil }

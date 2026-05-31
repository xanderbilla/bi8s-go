package model

type Attribute struct {
	ID            string          `json:"id" dynamodbav:"id" validate:"omitempty,min=1,max=64"`
	Name          string          `json:"name" dynamodbav:"name" validate:"required,min=1,max=128"`
	AttributeType []AttributeType `json:"attributeType" dynamodbav:"attributeType" validate:"required,min=1,dive,oneof=TAG MOOD GENRE CATEGORY SPECIALITY STUDIO SOCIAL PLATFORM"`
	Logo          string          `json:"logo,omitempty" dynamodbav:"logo,omitempty" validate:"omitempty,max=512"`
	SVG           string          `json:"svg,omitempty" dynamodbav:"svg,omitempty"`
	ContentType   ContentType     `json:"contentType" dynamodbav:"contentType"`
	Active        bool            `json:"active" dynamodbav:"active"`
	Audit         Audit           `json:"audit" dynamodbav:"audit"`
}

type AttributePublicDetail struct {
	ID            string          `json:"id"`
	Name          string          `json:"name"`
	AttributeType []AttributeType `json:"attributeType"`
	Logo          string          `json:"logo,omitempty"`
	SVG           string          `json:"svg,omitempty"`
	ContentType   ContentType     `json:"contentType"`
	Active        bool            `json:"active"`
}

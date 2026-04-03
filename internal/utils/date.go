package utils

import (
	"fmt"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
)

// Date represents a custom date time that serializes to/from YYYY-MM-DD
type Date struct {
	time.Time
}

const layout = "2006-01-02"

func (d *Date) UnmarshalJSON(b []byte) error {
	s := strings.Trim(string(b), `"`)
	if s == "" || s == "null" {
		d.Time = time.Time{}
		return nil
	}
	t, err := time.Parse(layout, s)
	if err != nil {
		return err
	}
	d.Time = t
	return nil
}

func (d Date) MarshalJSON() ([]byte, error) {
	if d.IsZero() {
		return []byte(`""`), nil // Return empty string or null instead of "0001-01-01"
	}
	return []byte(`"` + d.Format(layout) + `"`), nil
}

// MarshalDynamoDBAttributeValue implements the attributevalue.Marshaler interface.
func (d Date) MarshalDynamoDBAttributeValue() (types.AttributeValue, error) {
	if d.IsZero() {
		return &types.AttributeValueMemberNULL{Value: true}, nil
	}
	return &types.AttributeValueMemberS{Value: d.Format(layout)}, nil
}

// UnmarshalDynamoDBAttributeValue implements the attributevalue.Unmarshaler interface.
func (d *Date) UnmarshalDynamoDBAttributeValue(av types.AttributeValue) error {
	if av == nil {
		return nil
	}
	if _, ok := av.(*types.AttributeValueMemberNULL); ok {
		d.Time = time.Time{}
		return nil
	}
	s, ok := av.(*types.AttributeValueMemberS)
	if !ok {
		return fmt.Errorf("expected string for Date, got %T", av)
	}
	if s.Value == "" {
		d.Time = time.Time{}
		return nil
	}
	t, err := time.Parse(layout, s.Value)
	if err != nil {
		return err
	}
	d.Time = t
	return nil
}

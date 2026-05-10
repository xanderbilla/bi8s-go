package repository

import (
	"encoding/base64"
	"encoding/json"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
)

type rawAV struct {
	S *string `json:"S,omitempty"`
	N *string `json:"N,omitempty"`
}

func EncodeCursor(key map[string]types.AttributeValue) (string, error) {
	if len(key) == 0 {
		return "", nil
	}
	raw := make(map[string]rawAV, len(key))
	for k, v := range key {
		switch av := v.(type) {
		case *types.AttributeValueMemberS:
			s := av.Value
			raw[k] = rawAV{S: &s}
		case *types.AttributeValueMemberN:
			n := av.Value
			raw[k] = rawAV{N: &n}
		default:
			return "", fmt.Errorf("cursor: unsupported attribute type for key %q: %T", k, v)
		}
	}
	b, err := json.Marshal(raw)
	if err != nil {
		return "", err
	}
	return base64.URLEncoding.EncodeToString(b), nil
}

func DecodeCursor(cursor string) (map[string]types.AttributeValue, error) {
	if cursor == "" {
		return nil, nil
	}
	b, err := base64.URLEncoding.DecodeString(cursor)
	if err != nil {
		return nil, fmt.Errorf("cursor: invalid base64: %w", err)
	}
	var raw map[string]rawAV
	if err := json.Unmarshal(b, &raw); err != nil {
		return nil, fmt.Errorf("cursor: invalid json: %w", err)
	}
	key := make(map[string]types.AttributeValue, len(raw))
	for k, v := range raw {
		switch {
		case v.S != nil:
			key[k] = &types.AttributeValueMemberS{Value: *v.S}
		case v.N != nil:
			key[k] = &types.AttributeValueMemberN{Value: *v.N}
		default:
			return nil, fmt.Errorf("cursor: empty attribute for key %q", k)
		}
	}
	return key, nil
}

// Package utils contains stateless helper functions used systematically throughout the architecture securely validating ids and dates universally.
package utils

import (
	"github.com/google/uuid"
)

// GenerateID constructs a globally unique version 4 UUID string.
// Crucial for maintaining stateless distributions and avoiding sequential ID guessing vectors.
func GenerateID() string {
	id := uuid.New().String()
	return id
}

package utils

import (
	"github.com/google/uuid"
)

func GenerateID() string {
	id := uuid.New().String()
	return id
}

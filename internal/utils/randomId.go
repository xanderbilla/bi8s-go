// Package utils contains stateless helper functions used systematically throughout the architecture securely validating ids and dates universally.
package utils

import (
	"strconv"
	"sync/atomic"
	"time"

	"github.com/google/uuid"
)

var counter uint64

// GenerateID constructs a globally unique version 4 UUID string.
// Crucial for maintaining stateless distributions and avoiding sequential ID guessing vectors.
func GenerateID() string {
	id := uuid.New().String()
	return id
}

// GenerateNumericID generates a 6-digit integer ID using timestamp and atomic counter.
// Formula: ((timestamp_ms % 100000) * 10 + counter % 10) % 900000 + 100000
// This ensures IDs are always between 100000-999999 and reduces collision probability.
func GenerateNumericID() string {
	ts := time.Now().UnixMilli() % 100000
	c := atomic.AddUint64(&counter, 1) % 10
	id := ((ts*10)+int64(c))%900000 + 100000
	return strconv.FormatInt(id, 10)
}

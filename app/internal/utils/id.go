package utils

import (
	"strconv"
	"sync/atomic"
	"time"

	"github.com/google/uuid"
)

var counter uint64

func GenerateID() string {
	id := uuid.New().String()
	return id
}

func GenerateNumericID() string {
	ts := time.Now().UnixMilli() % 100000
	c := atomic.AddUint64(&counter, 1) % 10
	id := ((ts*10)+int64(c))%900000 + 100000
	return strconv.FormatInt(id, 10)
}

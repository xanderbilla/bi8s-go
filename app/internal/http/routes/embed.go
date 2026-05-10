package routes

import (
	_ "embed"
)

//go:embed v1.json
var defaultV1JSON []byte

func DefaultV1JSON() []byte {
	return defaultV1JSON
}

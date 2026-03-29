// Package env supplies safe environment variable extraction utilities defaulting smoothly when components randomly omit configurations natively.
package env

import (
	"log"
	"os"
	"strconv"
)

// GetString reads an environment variable as a string.
// If the variable isn't set, it logs a warning and returns the fallback value instead.
func GetString(key, fallback string) string {
	val, ok := os.LookupEnv(key)
	if !ok {
		log.Printf("Environment variable %s not set, using default: %s", key, fallback)
		return fallback
	}
	return val
}

// GetInt reads an environment variable and converts it to an int.
// If the variable isn't set or can't be parsed as a number, the fallback is returned silently.
func GetInt(key string, fallback int) int {
	val, ok := os.LookupEnv(key)
	if !ok {
		return fallback
	}

	valAsInt, err := strconv.Atoi(val)
	if err != nil {
		// Value is set but isn't a valid integer — just use the fallback.
		return fallback
	}
	return valAsInt
}

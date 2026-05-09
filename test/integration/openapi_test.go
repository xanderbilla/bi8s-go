//go:build integration

package integration

import (
	"path/filepath"
	"testing"

	"github.com/getkin/kin-openapi/openapi3"
)

// TestOpenAPISpec_ValidDocument validates docs/openapi.yaml parses and passes
// structural OpenAPI validation.
func TestOpenAPISpec_ValidDocument(t *testing.T) {
	t.Parallel()

	specPath := filepath.Join("..", "..", "docs", "openapi.yaml")
	loader := openapi3.NewLoader()
	doc, err := loader.LoadFromFile(specPath)
	if err != nil {
		t.Fatalf("load OpenAPI spec %s: %v", specPath, err)
	}
	if err := doc.Validate(loader.Context); err != nil {
		t.Fatalf("OpenAPI spec validation failed: %v", err)
	}
}

// TestOpenAPISpec_CorePaths ensures critical consumer routes remain in contract.
func TestOpenAPISpec_CorePaths(t *testing.T) {
	t.Parallel()

	specPath := filepath.Join("..", "..", "docs", "openapi.yaml")
	loader := openapi3.NewLoader()
	doc, err := loader.LoadFromFile(specPath)
	if err != nil {
		t.Fatalf("load OpenAPI spec %s: %v", specPath, err)
	}

	requiredPaths := []string{
		"/c/search",
		"/c/content/{contentId}",
		"/c/play/{contentType}/{contentId}",
	}
	for _, p := range requiredPaths {
		if doc.Paths.Value(p) == nil {
			t.Fatalf("missing required OpenAPI path %q", p)
		}
	}
}

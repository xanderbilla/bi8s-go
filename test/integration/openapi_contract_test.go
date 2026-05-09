//go:build integration

package integration

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/url"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/getkin/kin-openapi/openapi3filter"
	"github.com/getkin/kin-openapi/routers"
	"github.com/getkin/kin-openapi/routers/gorillamux"
)

// TestOpenAPIContract_LiveResponses validates that real responses from the
// running stack conform to docs/openapi.yaml. Skips cleanly when the stack
// is unreachable so unit-only runs keep working.
func TestOpenAPIContract_LiveResponses(t *testing.T) {
	t.Parallel()

	specPath := filepath.Join("..", "..", "docs", "openapi.yaml")
	loader := openapi3.NewLoader()
	loader.IsExternalRefsAllowed = true
	doc, err := loader.LoadFromFile(specPath)
	if err != nil {
		t.Fatalf("load OpenAPI spec %s: %v", specPath, err)
	}
	if err := doc.Validate(loader.Context); err != nil {
		t.Fatalf("OpenAPI spec invalid: %v", err)
	}
	router, err := gorillamux.NewRouter(doc)
	if err != nil {
		t.Fatalf("build router: %v", err)
	}

	base := baseURL()
	client := &http.Client{Timeout: 5 * time.Second}

	// Probe reachability once so the rest of the cases can fail loudly.
	if _, err := client.Get(base + "/v1/livez"); err != nil {
		t.Skipf("local stack not reachable at %s: %v (run `make docker-up`)", base, err)
	}

	cases := []struct {
		name         string
		method, path string
		// Expected status family (e.g. 2 for 2xx, 4 for 4xx). 0 = accept any
		// status that is documented in the spec.
		statusFamily int
	}{
		{name: "livez", method: http.MethodGet, path: "/v1/livez", statusFamily: 2},
		{name: "readyz", method: http.MethodGet, path: "/v1/readyz", statusFamily: 0},
		{name: "search_missing_query", method: http.MethodGet, path: "/v1/c/search", statusFamily: 4},
		{name: "content_not_found", method: http.MethodGet, path: "/v1/c/content/does-not-exist-" + nowSuffix(), statusFamily: 4},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req, err := http.NewRequest(tc.method, base+tc.path, nil)
			if err != nil {
				t.Fatalf("build request: %v", err)
			}
			resp, err := client.Do(req)
			if err != nil {
				t.Fatalf("request: %v", err)
			}
			defer resp.Body.Close()

			if tc.statusFamily != 0 && resp.StatusCode/100 != tc.statusFamily {
				t.Fatalf("status %d not in %dxx family", resp.StatusCode, tc.statusFamily)
			}

			body, _ := io.ReadAll(resp.Body)
			validateAgainstSpec(t, router, req, resp, body)
		})
	}
}

func validateAgainstSpec(t *testing.T, router routers.Router, req *http.Request, resp *http.Response, body []byte) {
	t.Helper()

	// Re-resolve the request against the spec router. The router needs the
	// path component only; rewrite to strip any base URL.
	reqForRouter := req.Clone(context.Background())
	if u, err := url.Parse(req.URL.String()); err == nil {
		reqForRouter.URL = u
	}
	route, pathParams, err := router.FindRoute(reqForRouter)
	if err != nil {
		// Path not in spec — that's a contract gap worth flagging.
		t.Fatalf("path %s %s not in OpenAPI spec: %v", req.Method, req.URL.Path, err)
	}

	respValidationInput := &openapi3filter.ResponseValidationInput{
		RequestValidationInput: &openapi3filter.RequestValidationInput{
			Request:    reqForRouter,
			PathParams: pathParams,
			Route:      route,
		},
		Status: resp.StatusCode,
		Header: resp.Header,
	}
	if len(body) > 0 {
		respValidationInput.SetBodyBytes(body)
		// Keep the response body re-readable for downstream assertions.
		resp.Body = io.NopCloser(bytes.NewReader(body))
	}

	if err := openapi3filter.ValidateResponse(context.Background(), respValidationInput); err != nil {
		// Truncate body in error to avoid noisy failures.
		preview := string(body)
		if len(preview) > 512 {
			preview = preview[:512] + "...(truncated)"
		}
		t.Fatalf("response %s %s [%d] does not match OpenAPI schema: %v\nbody: %s",
			req.Method, req.URL.Path, resp.StatusCode, err, strings.TrimSpace(preview))
	}
}

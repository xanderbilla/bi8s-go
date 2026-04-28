package http

import (
	"expvar"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
)

func TestStatusClass(t *testing.T) {
	cases := map[int]string{
		100: "1xx", 200: "2xx", 204: "2xx",
		301: "3xx", 404: "4xx", 422: "4xx",
		500: "5xx", 503: "5xx",
	}
	for code, want := range cases {
		if got := statusClass(code); got != want {
			t.Errorf("statusClass(%d)=%s want=%s", code, got, want)
		}
	}
}

func snapshot(m *expvar.Map, key string) int64 {
	v := m.Get(key)
	if v == nil {
		return 0
	}
	if iv, ok := v.(*expvar.Int); ok {
		return iv.Value()
	}
	return 0
}

func TestMetricsMiddleware(t *testing.T) {
	before2xx := snapshot(httpRequestsTotal, "2xx")
	before4xx := snapshot(httpRequestsTotal, "4xx")
	beforeAll := snapshot(httpRequestsTotal, "all")
	before200 := snapshot(httpRequestsByStatus, strconv.Itoa(http.StatusOK))

	h := MetricsMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/bad" {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		w.Write([]byte("ok"))
	}))

	for _, p := range []string{"/", "/", "/bad"} {
		req := httptest.NewRequest(http.MethodGet, p, nil)
		h.ServeHTTP(httptest.NewRecorder(), req)
	}

	if got := snapshot(httpRequestsTotal, "2xx") - before2xx; got != 2 {
		t.Errorf("2xx delta=%d want=2", got)
	}
	if got := snapshot(httpRequestsTotal, "4xx") - before4xx; got != 1 {
		t.Errorf("4xx delta=%d want=1", got)
	}
	if got := snapshot(httpRequestsTotal, "all") - beforeAll; got != 3 {
		t.Errorf("all delta=%d want=3", got)
	}
	if got := snapshot(httpRequestsByStatus, strconv.Itoa(http.StatusOK)) - before200; got != 2 {
		t.Errorf("200 delta=%d want=2", got)
	}

	if v := inFlightGauge.Load(); v != 0 {
		t.Errorf("inFlightGauge=%d want=0", v)
	}
}

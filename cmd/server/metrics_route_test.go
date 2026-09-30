package main

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"vigil/internal/models"
)

// TestMetricsRouteIsNotSwallowedBySPA guards the routing: "/" serves the SPA
// for any unknown path, so if GET /metrics were ever dropped from the mux a
// scraper would get index.html with a 200 and fail with a confusing parse
// error instead of a clear 401/404.
func TestMetricsRouteIsNotSwallowedBySPA(t *testing.T) {
	cases := []struct {
		name  string
		token string
		want  int
	}{
		{"disabled", "", http.StatusNotFound},
		{"enabled, no credentials", "0123456789abcdef0123456789abcdef", http.StatusUnauthorized},
	}
	for _, c := range cases {
		mux := setupRoutes(models.Config{AuthEnabled: true, MetricsToken: c.token})
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/metrics", nil))
		if rec.Code != c.want {
			t.Errorf("%s: GET /metrics = %d, want %d", c.name, rec.Code, c.want)
		}
	}
}

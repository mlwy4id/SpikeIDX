package http

import (
	stdhttp "net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestOpenAPISpecServesYAML(t *testing.T) {
	d := testDeps()
	req := httptest.NewRequest("GET", "/openapi.yaml", nil)
	rec := httptest.NewRecorder()
	d.Router().ServeHTTP(rec, req)

	if rec.Code != stdhttp.StatusOK {
		t.Fatalf("got %d %s", rec.Code, rec.Body.String())
	}

	if ct := rec.Header().Get("Content-Type"); !strings.Contains(ct, "application/yaml") {
		t.Fatalf("want application/yaml content-type, got %q", ct)
	}

	body := rec.Body.String()

	for _, want := range []string{"openapi:", "/api/v1/search", "/api/v1/watchlist", "/api/v1/signals"} {
		if !strings.Contains(body, want) {
			t.Fatalf("spec missing %q", want)
		}
	}
}

func TestSwaggerUIServesHTML(t *testing.T) {
	d := testDeps()
	req := httptest.NewRequest("GET", "/swagger/index.html", nil)
	rec := httptest.NewRecorder()
	d.Router().ServeHTTP(rec, req)

	if rec.Code != stdhttp.StatusOK {
		t.Fatalf("got %d %s", rec.Code, rec.Body.String())
	}

	body := rec.Body.String()

	for _, want := range []string{"swagger-ui", "/openapi.yaml", "SwaggerUIBundle"} {
		if !strings.Contains(body, want) {
			t.Fatalf("swagger UI missing %q", want)
		}
	}
}

func TestDocsRedirects(t *testing.T) {
	d := testDeps()

	for _, path := range []string{"/swagger", "/docs"} {
		req := httptest.NewRequest("GET", path, nil)
		rec := httptest.NewRecorder()
		d.Router().ServeHTTP(rec, req)

		if rec.Code != stdhttp.StatusMovedPermanently {
			t.Fatalf("%s: got %d, want 301", path, rec.Code)
		}

		if loc := rec.Header().Get("Location"); loc != "/swagger/index.html" {
			t.Fatalf("%s: want Location /swagger/index.html, got %q", path, loc)
		}
	}
}

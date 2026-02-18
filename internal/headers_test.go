package internal

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestTrustXOriginalURI(t *testing.T) {
	t.Run("disabled strips header", func(t *testing.T) {
		var got string
		next := http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
			got = r.Header.Get("X-Original-URI")
		})

		h := TrustXOriginalURI(false, next)
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.Header.Set("X-Original-URI", "/admin")
		rr := httptest.NewRecorder()

		h.ServeHTTP(rr, req)

		if got != "" {
			t.Fatalf("expected X-Original-URI to be stripped, got %q", got)
		}
	})

	t.Run("enabled keeps header", func(t *testing.T) {
		var got string
		next := http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
			got = r.Header.Get("X-Original-URI")
		})

		h := TrustXOriginalURI(true, next)
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.Header.Set("X-Original-URI", "/admin")
		rr := httptest.NewRecorder()

		h.ServeHTTP(rr, req)

		if got != "/admin" {
			t.Fatalf("expected X-Original-URI to be preserved, got %q", got)
		}
	})
}

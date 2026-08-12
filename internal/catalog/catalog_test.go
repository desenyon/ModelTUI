package catalog_test

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/desenyon/ModelTUI/internal/catalog"
)

func TestParseEmbeddedSnapshot(t *testing.T) {
	client := catalog.NewClient()
	client.BaseURL = "http://127.0.0.1:1"
	client.CacheDir = t.TempDir()
	cat, source, err := client.LoadCatalog(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if source != "embedded snapshot" {
		t.Fatalf("expected embedded snapshot, got %q", source)
	}
	idx := catalog.BuildIndex(cat, source)
	if len(idx.Models) == 0 || len(idx.Providers) == 0 || len(idx.Offerings) == 0 || len(idx.Labs) == 0 {
		t.Fatalf("unexpected empty index")
	}
}

func TestRateLimitSpacing(t *testing.T) {
	hits := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits++
		w.Header().Set("ETag", `"v1"`)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"models":{},"providers":{}}`))
	}))
	defer srv.Close()

	client := catalog.NewClient()
	client.BaseURL = srv.URL
	client.CacheDir = t.TempDir()

	res1 := client.RefreshCatalog(t.Context(), true)
	if res1.Err != nil {
		t.Fatalf("first refresh: %v", res1.Err)
	}
	res2 := client.RefreshCatalog(t.Context(), true)
	if res2.Err == nil {
		t.Fatal("expected second refresh to be rate limited")
	}
	if hits != 1 {
		t.Fatalf("expected 1 network hit, got %d", hits)
	}
	ok, wait := client.CanRefresh(true)
	if ok || wait <= 0 {
		t.Fatalf("expected wait, ok=%v wait=%s", ok, wait)
	}
}

func TestHonors429RetryAfter(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Retry-After", "2")
		http.Error(w, "slow down", http.StatusTooManyRequests)
	}))
	defer srv.Close()

	client := catalog.NewClient()
	client.BaseURL = srv.URL
	client.CacheDir = t.TempDir()

	res := client.RefreshCatalog(t.Context(), true)
	if res.Err == nil {
		t.Fatal("expected 429 error")
	}
	if res.RetryAfter < 2*time.Second {
		t.Fatalf("expected retry-after >= 2s, got %s", res.RetryAfter)
	}
}
